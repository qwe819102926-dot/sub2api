package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
)

type ipMonitorTestSettings struct {
	SettingRepository
	db *sql.DB
}

func (r *ipMonitorTestSettings) GetValue(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrSettingNotFound
	}
	return value, err
}
func (r *ipMonitorTestSettings) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, key, value)
	return err
}
func TestRegistrationIPRuleAndNormalization(t *testing.T) {
	for _, c := range []RegistrationIPMonitorConfig{{WindowHours: 0, MaxAccounts: 5, BanHours: 24}, {WindowHours: 24, MaxAccounts: 0, BanHours: 24}, {WindowHours: 24, MaxAccounts: 5, BanHours: 8761}} {
		require.Error(t, c.Validate())
	}
	require.NoError(t, (RegistrationIPMonitorConfig{WindowHours: 24, MaxAccounts: 5, BanHours: 24}).Validate())
	ctx := WithRegistrationClientIP(context.Background(), "::ffff:192.0.2.1")
	require.Equal(t, "192.0.2.1", ctx.Value(registrationIPContextKey{}))
	ctx = WithRegistrationClientIP(ctx, "forged")
	require.Equal(t, "", ctx.Value(registrationIPContextKey{}))
}
func TestRegistrationIPMonitorPostgres(t *testing.T) {
	payment, db := conversionTestService(t)
	_, err := db.Exec(`ALTER TABLE settings ADD COLUMN updated_at TIMESTAMPTZ DEFAULT NOW();ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active'`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("253_registration_ip_monitor.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	s := &AuthService{entClient: payment.entClient, settingService: &SettingService{settingRepo: &ipMonitorTestSettings{db: db}}}
	db.SetMaxOpenConns(3) // A saturated pool must not deadlock on ban persistence.
	var sequence atomic.Int64
	sequence.Store(100)
	register := func(ip string, fail bool, outer *dbent.Tx) error {
		ctx := WithRegistrationClientIP(context.Background(), ip)
		if outer != nil {
			ctx = dbent.NewTxContext(ctx, outer)
		}
		user := &User{ID: sequence.Add(1), Email: fmt.Sprintf("member%d@example.com", sequence.Load()), SignupSource: "email"}
		return s.withRegistrationIPMonitor(ctx, user, func(ctx context.Context) error {
			_, err := dbent.TxFromContext(ctx).Client().ExecContext(ctx, `INSERT INTO users(id,email) VALUES($1,$2)`, user.ID, user.Email)
			if err != nil {
				return err
			}
			if fail {
				return errors.New("signup reward failed")
			}
			return nil
		})
	}
	ctx := context.Background()
	require.Error(t, register("192.0.2.1", true, nil))
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM registration_ip_events`).Scan(&n))
	require.Zero(t, n)
	for i := 0; i < 5; i++ {
		require.NoError(t, register("192.0.2.1", false, nil))
	}
	require.ErrorIs(t, register("::ffff:192.0.2.1", false, nil), ErrRegistrationIPBlocked)
	require.ErrorIs(t, register("192.0.2.1", false, nil), ErrRegistrationIPBlocked)
	report, err := s.GetRegistrationIPReport(ctx, "192.0.2.1", "192.0.2.1", 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, report.Total)
	require.Len(t, report.Rows, 1)
	require.EqualValues(t, 5, report.Rows[0].Total)
	require.EqualValues(t, 5, report.Rows[0].Recent)
	require.True(t, report.Rows[0].Active)
	require.Equal(t, 1, report.Rows[0].BanCount)
	require.Len(t, report.Accounts, 5)
	require.NoError(t, register("192.0.2.2", false, nil))
	require.NoError(t, s.UnblockRegistrationIP(ctx, "192.0.2.1"))
	require.NoError(t, register("192.0.2.1", false, nil), "manual unblock resets enforcement quota")
	// Successful inner creation followed by invitation rollback does not leave a registration.
	tx, err := s.entClient.Tx(ctx)
	require.NoError(t, err)
	require.NoError(t, register("192.0.2.3", false, tx))
	require.NoError(t, tx.Rollback())
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM registration_ip_events WHERE ip='192.0.2.3'`).Scan(&n))
	require.Zero(t, n)
	// Parallel registrations cannot overshoot the account cap or extend the first ban.
	var ok, blocked atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := register("2001:db8::1", false, nil)
			if err == nil {
				ok.Add(1)
			} else if errors.Is(err, ErrRegistrationIPBlocked) {
				blocked.Add(1)
			} else {
				t.Errorf("unexpected signup failure: %v", err)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 5, ok.Load())
	require.EqualValues(t, 7, blocked.Load())
	// Ban persists even when the rejected OAuth caller rolls back its outer transaction.
	tx, err = s.entClient.Tx(ctx)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		require.NoError(t, register("192.0.2.4", false, nil))
	}
	require.ErrorIs(t, register("192.0.2.4", false, tx), ErrRegistrationIPBlocked)
	_ = tx.Rollback()
	require.ErrorIs(t, register("192.0.2.4", false, nil), ErrRegistrationIPBlocked)
	// Natural expiry plus an elapsed rolling window permits a fresh signup.
	_, err = db.Exec(`UPDATE registration_ip_events SET created_at=NOW()-interval '25 hours' WHERE ip='192.0.2.4';UPDATE registration_ip_blocks SET blocked_until=NOW()-interval '1 minute' WHERE ip='192.0.2.4'`)
	require.NoError(t, err)
	require.NoError(t, register("192.0.2.4", false, nil))
	// Account removal preserves evidence and registered email.
	_, err = db.Exec(`DELETE FROM users WHERE id IN(SELECT user_id FROM registration_ip_events WHERE ip='192.0.2.2')`)
	require.NoError(t, err)
	report, err = s.GetRegistrationIPReport(ctx, "192.0.2.2", "192.0.2.2", 1)
	require.NoError(t, err)
	require.True(t, report.Accounts[0].Deleted)
	c := RegistrationIPMonitorConfig{Enabled: false, WindowHours: 24, MaxAccounts: 5, BanHours: 24}
	require.NoError(t, s.settingService.SetRegistrationIPMonitorConfig(ctx, c))
	require.NoError(t, register("2001:db8::1", false, nil), "disabled enforcement allows signup but still records it")
}
