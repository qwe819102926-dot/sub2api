package service

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type BonusBalanceReportFilter struct {
	Source   string
	Kind     string
	Search   string
	Page     int
	PageSize int
}

type BonusBalanceReportSummary struct {
	Issued                   float64 `json:"issued"`
	Consumed                 float64 `json:"consumed"`
	Remaining                float64 `json:"remaining"`
	Adjustments              float64 `json:"adjustments"`
	HistoricalOutflow        float64 `json:"historical_outflow"`
	DeletedUserRemaining     float64 `json:"deleted_user_remaining"`
	ReconciliationDifference float64 `json:"reconciliation_difference"`
}

type BonusBalanceReportSource struct {
	Source string  `json:"source"`
	Amount float64 `json:"amount"`
	Grants int64   `json:"grants"`
	Users  int64   `json:"users"`
}

type BonusBalanceReportRecord struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Email        string    `json:"email"`
	Source       string    `json:"source"`
	Amount       float64   `json:"amount"`
	BalanceAfter *float64  `json:"balance_after"`
	Historical   bool      `json:"historical"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type BonusBalanceReport struct {
	Summary  BonusBalanceReportSummary  `json:"summary"`
	Sources  []BonusBalanceReportSource `json:"sources"`
	Records  []BonusBalanceReportRecord `json:"records"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

func normalizeBonusReportFilter(f BonusBalanceReportFilter) (BonusBalanceReportFilter, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Page > 1000000 {
		f.Page = 1000000
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	f.Search = strings.TrimSpace(f.Search)
	if len(f.Search) > 128 {
		return f, infraerrors.BadRequest("INVALID_SEARCH", "search is too long")
	}
	switch f.Source {
	case "", "signup", "invite", "checkin", "recharge", "consumption_reward", "lottery", "admin", "consumption", "historical_credit", "historical_outflow", "account_removed", "conversion", "other":
	default:
		return f, infraerrors.BadRequest("INVALID_BONUS_SOURCE", "unknown bonus source")
	}
	switch f.Kind {
	case "", "grant", "consumption", "adjustment":
	default:
		return f, infraerrors.BadRequest("INVALID_BONUS_KIND", "unknown bonus movement type")
	}
	return f, nil
}

// Reports use a single read-only snapshot so the source totals and retained
// wallet balances reconcile even while live billing updates them.
func (s *PaymentService) GetBonusBalanceReport(ctx context.Context, filter BonusBalanceReportFilter) (*BonusBalanceReport, error) {
	f, err := normalizeBonusReportFilter(filter)
	if err != nil {
		return nil, err
	}
	if s == nil || s.entClient == nil {
		return nil, ErrServiceUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := s.entClient.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := readBonusBalanceReport(ctx, tx.Client(), f)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

const bonusReportSummarySQL = `SELECT
COALESCE(SUM(amount) FILTER (WHERE amount > 0), 0)::double precision,
COALESCE(-SUM(amount) FILTER (WHERE source = 'consumption' AND amount < 0), 0)::double precision,
(SELECT COALESCE(SUM(bonus_balance), 0)::double precision FROM users),
COALESCE(-SUM(amount) FILTER (WHERE amount < 0 AND source NOT IN ('consumption', 'historical_outflow')), 0)::double precision,
COALESCE(-SUM(amount) FILTER (WHERE amount < 0 AND source = 'historical_outflow'), 0)::double precision,
(SELECT COALESCE(SUM(bonus_balance), 0)::double precision FROM users WHERE deleted_at IS NOT NULL),
(COALESCE(SUM(amount), 0) - (SELECT COALESCE(SUM(bonus_balance), 0) FROM users))::double precision
FROM bonus_balance_events`

const bonusReportWhereSQL = ` WHERE ($1 = '' OR source = $1)
AND ($3 = '' OR ($2 <> '' AND email ILIKE $2) OR user_id::text = $3)
AND ($4 = '' OR ($4 = 'grant' AND amount > 0) OR ($4 = 'consumption' AND source = 'consumption' AND amount < 0)
OR ($4 = 'adjustment' AND amount < 0 AND source <> 'consumption'))`

func readBonusBalanceReport(ctx context.Context, client *dbent.Client, f BonusBalanceReportFilter) (*BonusBalanceReport, error) {
	result := &BonusBalanceReport{Page: f.Page, PageSize: f.PageSize, Sources: []BonusBalanceReportSource{}, Records: []BonusBalanceReportRecord{}}
	rows, err := client.QueryContext(ctx, bonusReportSummarySQL)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		err = errors.Join(rows.Err(), rows.Close())
		if err == nil {
			err = sql.ErrNoRows
		}
		return nil, err
	}
	s := &result.Summary
	err = rows.Scan(&s.Issued, &s.Consumed, &s.Remaining, &s.Adjustments, &s.HistoricalOutflow, &s.DeletedUserRemaining, &s.ReconciliationDifference)
	err = errors.Join(err, rows.Close())
	if err != nil {
		return nil, err
	}
	rows, err = client.QueryContext(ctx, `SELECT source, SUM(amount)::double precision, COUNT(*), COUNT(DISTINCT user_id) FROM bonus_balance_events WHERE amount > 0 GROUP BY source ORDER BY SUM(amount) DESC, source`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item BonusBalanceReportSource
		if err := rows.Scan(&item.Source, &item.Amount, &item.Grants, &item.Users); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		result.Sources = append(result.Sources, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	search := ""
	if id, err := strconv.ParseInt(f.Search, 10, 64); err == nil && id > 0 {
		f.Search = strconv.FormatInt(id, 10)
	} else if f.Search != "" {
		search = "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(f.Search) + "%"
	}
	args := []any{f.Source, search, f.Search, f.Kind}
	rows, err = client.QueryContext(ctx, `SELECT COUNT(*) FROM bonus_balance_events`+bonusReportWhereSQL, args...)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		err = errors.Join(rows.Err(), rows.Close())
		if err == nil {
			err = sql.ErrNoRows
		}
		return nil, err
	}
	err = rows.Scan(&result.Total)
	err = errors.Join(err, rows.Close())
	if err != nil {
		return nil, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err = client.QueryContext(ctx, `SELECT id, user_id, email, source, amount::double precision, balance_after::double precision, historical, created_at, updated_at FROM bonus_balance_events`+bonusReportWhereSQL+` ORDER BY created_at DESC, id DESC LIMIT $5 OFFSET $6`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item BonusBalanceReportRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.Email, &item.Source, &item.Amount, &item.BalanceAfter, &item.Historical, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result.Records = append(result.Records, item)
	}
	return result, errors.Join(rows.Err(), rows.Close())
}
