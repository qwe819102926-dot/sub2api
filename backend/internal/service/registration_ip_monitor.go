package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"net/netip"
	"time"
)

const registrationIPMonitorKey = "registration_ip_monitor"

var ErrRegistrationIPBlocked = infraerrors.TooManyRequests("REGISTRATION_IP_BLOCKED", "该 IP 短时间注册账号过多，已被临时禁止注册，请稍后重试或联系客服。")

type registrationIPContextKey struct{}

func WithRegistrationClientIP(ctx context.Context, value string) context.Context {
	addr, err := netip.ParseAddr(value)
	if err != nil || addr.Zone() != "" {
		return context.WithValue(ctx, registrationIPContextKey{}, "")
	}
	return context.WithValue(ctx, registrationIPContextKey{}, addr.Unmap().String())
}

type RegistrationIPMonitorConfig struct {
	Enabled     bool `json:"enabled"`
	WindowHours int  `json:"window_hours"`
	MaxAccounts int  `json:"max_accounts"`
	BanHours    int  `json:"ban_hours"`
}

func (c RegistrationIPMonitorConfig) Validate() error {
	if c.WindowHours < 1 || c.WindowHours > 720 || c.MaxAccounts < 1 || c.MaxAccounts > 10000 || c.BanHours < 1 || c.BanHours > 8760 {
		return infraerrors.BadRequest("INVALID_REGISTRATION_IP_RULE", "窗口需为 1–720 小时，账号数为 1–10000，封禁为 1–8760 小时")
	}
	return nil
}
func (s *SettingService) GetRegistrationIPMonitorConfig(ctx context.Context) (RegistrationIPMonitorConfig, error) {
	c := RegistrationIPMonitorConfig{WindowHours: 24, MaxAccounts: 5, BanHours: 24}
	value, err := s.settingRepo.GetValue(ctx, registrationIPMonitorKey)
	// Missing config is disabled for older installations; the migration explicitly enables it.
	if errors.Is(err, ErrSettingNotFound) || (err == nil && value == "") {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal([]byte(value), &c); err != nil {
		return c, err
	}
	return c, c.Validate()
}
func (s *SettingService) SetRegistrationIPMonitorConfig(ctx context.Context, c RegistrationIPMonitorConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	value, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, registrationIPMonitorKey, string(value))
}
func registrationIPRow(ctx context.Context, client *dbent.Client, query string, args []any, dest ...any) error {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Scan(dest...)
}
func lockRegistrationIP(ctx context.Context, client *dbent.Client, ip string) error {
	rows, err := client.QueryContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('registration-ip:' || $1,0))`, ip)
	if err != nil {
		return err
	}
	return rows.Close()
}

// The IP lock spans both account creation and reward/invitation writes, including
// callers' existing transactions. Failed registrations never consume quota.
func (s *AuthService) withRegistrationIPMonitor(ctx context.Context, user *User, create func(context.Context) error) error {
	c, err := s.settingService.GetRegistrationIPMonitorConfig(ctx)
	if err != nil {
		return ErrServiceUnavailable.WithCause(err)
	}
	ip, _ := ctx.Value(registrationIPContextKey{}).(string)
	if ip == "" || s.entClient == nil {
		if c.Enabled {
			return ErrServiceUnavailable
		}
		return create(ctx)
	}
	tx := dbent.TxFromContext(ctx)
	owned := tx == nil
	if owned {
		tx, err = s.entClient.Tx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		ctx = dbent.NewTxContext(ctx, tx)
	}
	client := tx.Client()
	if err = lockRegistrationIP(ctx, client, ip); err != nil {
		return err
	}
	if c.Enabled {
		var blocked bool
		var count int
		err = registrationIPRow(ctx, client, `SELECT
   EXISTS(SELECT 1 FROM registration_ip_blocks WHERE ip=$1::inet AND blocked_until>clock_timestamp()),
   (SELECT COUNT(*) FROM registration_ip_events WHERE ip=$1::inet
    AND created_at>clock_timestamp()-($2 * interval '1 hour')
    AND created_at>COALESCE((SELECT reset_at FROM registration_ip_blocks WHERE ip=$1::inet),'-infinity'::timestamptz))`, []any{ip, c.WindowHours}, &blocked, &count)
		if err != nil {
			return err
		}
		if blocked {
			return ErrRegistrationIPBlocked
		}
		if count >= c.MaxAccounts {
			if !owned {
				// Registration has been rejected. Abort its outer transaction
				// before opening the ban transaction, so saturated pools cannot
				// deadlock while holding every connection on the same IP lock.
				if err = tx.Rollback(); err != nil {
					return err
				}
				banTx, beginErr := s.entClient.Tx(ctx)
				if beginErr != nil {
					return beginErr
				}
				defer banTx.Rollback()
				client = banTx.Client()
				if err = lockRegistrationIP(ctx, client, ip); err != nil {
					return err
				}
				// A manual unblock may have reset the quota after rollback.
				err = registrationIPRow(ctx, client, `SELECT COUNT(*) FROM registration_ip_events WHERE ip=$1::inet
                AND created_at>clock_timestamp()-($2 * interval '1 hour')
                AND created_at>COALESCE((SELECT reset_at FROM registration_ip_blocks WHERE ip=$1::inet),'-infinity'::timestamptz)`, []any{ip, c.WindowHours}, &count)
				if err != nil {
					return err
				}
				if count >= c.MaxAccounts {
					if err = writeRegistrationIPBan(ctx, client, ip, c, count); err != nil {
						return err
					}
				}
				if err = banTx.Commit(); err != nil {
					return err
				}
			} else {
				// No account or reward mutations have occurred in this owned
				// transaction, so commit only the ban and return a rejection.
				if err = writeRegistrationIPBan(ctx, client, ip, c, count); err != nil {
					return err
				}
				if err = tx.Commit(); err != nil {
					return err
				}
			}
			return ErrRegistrationIPBlocked
		}
	}
	if err = create(ctx); err != nil {
		return err
	}
	_, err = client.ExecContext(ctx, `INSERT INTO registration_ip_events(user_id,ip,email,source) VALUES($1,$2::inet,$3,$4)`, user.ID, ip, user.Email, user.SignupSource)
	if err != nil {
		return err
	}
	if owned {
		return tx.Commit()
	}
	return nil
}

func writeRegistrationIPBan(ctx context.Context, client *dbent.Client, ip string, c RegistrationIPMonitorConfig, count int) error {
	_, err := client.ExecContext(ctx, `INSERT INTO registration_ip_blocks(ip,blocked_at,blocked_until,registrations)
 VALUES($1::inet,clock_timestamp(),clock_timestamp()+($2 * interval '1 hour'),$3)
 ON CONFLICT(ip) DO UPDATE SET blocked_at=EXCLUDED.blocked_at,blocked_until=EXCLUDED.blocked_until,
 registrations=EXCLUDED.registrations,ban_count=registration_ip_blocks.ban_count+1
 WHERE registration_ip_blocks.blocked_until<=clock_timestamp()`, ip, c.BanHours, count)
	return err
}

type RegistrationIPStat struct {
	IP               string     `json:"ip"`
	Total            int64      `json:"total"`
	Recent           int64      `json:"recent"`
	LastRegisteredAt time.Time  `json:"last_registered_at"`
	BlockedAt        *time.Time `json:"blocked_at"`
	BlockedUntil     *time.Time `json:"blocked_until"`
	Active           bool       `json:"active"`
	BanCount         int        `json:"ban_count"`
}
type RegistrationIPAccount struct {
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
	Deleted   bool      `json:"deleted"`
}
type RegistrationIPReport struct {
	Config       RegistrationIPMonitorConfig `json:"config"`
	Rows         []RegistrationIPStat        `json:"rows"`
	Accounts     []RegistrationIPAccount     `json:"accounts"`
	Total        int64                       `json:"total"`
	AccountTotal int64                       `json:"account_total"`
	Page         int                         `json:"page"`
}

func (s *AuthService) GetRegistrationIPReport(ctx context.Context, search, selectedIP string, page int) (*RegistrationIPReport, error) {
	if s.entClient == nil || s.settingService == nil {
		return nil, ErrServiceUnavailable
	}
	if len(search) > 64 {
		return nil, infraerrors.BadRequest("INVALID_SEARCH", "search is too long")
	}
	if selectedIP != "" {
		ip, err := netip.ParseAddr(selectedIP)
		if err != nil {
			return nil, infraerrors.BadRequest("INVALID_IP", "invalid IP")
		}
		selectedIP = ip.Unmap().String()
	}
	if page < 1 {
		page = 1
	}
	if page > 1000000 {
		page = 1000000
	}
	c, err := s.settingService.GetRegistrationIPMonitorConfig(ctx)
	if err != nil {
		return nil, err
	}
	r := &RegistrationIPReport{Config: c, Rows: []RegistrationIPStat{}, Accounts: []RegistrationIPAccount{}, Page: page}
	tx, err := s.entClient.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	client := tx.Client()
	err = registrationIPRow(ctx, client, `SELECT COUNT(DISTINCT ip) FROM registration_ip_events WHERE position($1 in host(ip))>0`, []any{search}, &r.Total)
	if err != nil {
		return nil, err
	}
	rows, err := client.QueryContext(ctx, `SELECT host(e.ip),COUNT(*),COUNT(*) FILTER (WHERE e.created_at>clock_timestamp()-($2 * interval '1 hour')),
 MAX(e.created_at),b.blocked_at,b.blocked_until,COALESCE(b.blocked_until>clock_timestamp(),FALSE),COALESCE(b.ban_count,0)
 FROM registration_ip_events e LEFT JOIN registration_ip_blocks b ON b.ip=e.ip
 WHERE position($1 in host(e.ip))>0 GROUP BY e.ip,b.ip
 ORDER BY COALESCE(b.blocked_until>clock_timestamp(),FALSE) DESC,MAX(e.created_at) DESC,e.ip LIMIT 20 OFFSET $3`, search, c.WindowHours, (page-1)*20)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v RegistrationIPStat
		if err = rows.Scan(&v.IP, &v.Total, &v.Recent, &v.LastRegisteredAt, &v.BlockedAt, &v.BlockedUntil, &v.Active, &v.BanCount); err != nil {
			rows.Close()
			return nil, err
		}
		r.Rows = append(r.Rows, v)
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return nil, err
	}
	if selectedIP != "" {
		err = registrationIPRow(ctx, client, `SELECT COUNT(*) FROM registration_ip_events WHERE ip=$1::inet`, []any{selectedIP}, &r.AccountTotal)
		if err != nil {
			return nil, err
		}
		rows, err = client.QueryContext(ctx, `SELECT e.user_id,e.email,e.source,e.created_at,COALESCE(u.status,'deleted'),(u.id IS NULL OR u.deleted_at IS NOT NULL)
   FROM registration_ip_events e LEFT JOIN users u ON u.id=e.user_id WHERE e.ip=$1::inet ORDER BY e.created_at DESC,e.user_id DESC LIMIT 100`, selectedIP)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var v RegistrationIPAccount
			if err = rows.Scan(&v.UserID, &v.Email, &v.Source, &v.CreatedAt, &v.Status, &v.Deleted); err != nil {
				rows.Close()
				return nil, err
			}
			r.Accounts = append(r.Accounts, v)
		}
		err = errors.Join(rows.Err(), rows.Close())
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}
func (s *AuthService) UnblockRegistrationIP(ctx context.Context, value string) error {
	ip, err := netip.ParseAddr(value)
	if err != nil || ip.Zone() != "" {
		return infraerrors.BadRequest("INVALID_IP", "invalid IP")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockRegistrationIP(ctx, tx.Client(), ip.Unmap().String()); err != nil {
		return err
	}
	_, err = tx.Client().ExecContext(ctx, `UPDATE registration_ip_blocks SET blocked_until=clock_timestamp(),reset_at=clock_timestamp() WHERE ip=$1::inet`, ip.Unmap().String())
	if err != nil {
		return err
	}
	return tx.Commit()
}
