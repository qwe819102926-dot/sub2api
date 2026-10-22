package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// This is a single, permanent conversion, not a general wallet adjustment tool.
const BonusBalanceConversionConfirmation = "DIVIDE_BY_10"

type BonusBalanceConversion struct {
	Completed   bool       `json:"completed"`
	Users       int64      `json:"users"`
	BeforeTotal string     `json:"before_total"`
	AfterTotal  string     `json:"after_total"`
	CompletedAt *time.Time `json:"completed_at"`
}

func (s *PaymentService) GetBonusBalanceConversion(ctx context.Context) (*BonusBalanceConversion, error) {
	if s == nil || s.entClient == nil {
		return nil, ErrServiceUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// The completed receipt or a live preview, in one consistent statement.
	rows, err := s.entClient.QueryContext(ctx, `
SELECT TRUE, affected_users, before_total::text, after_total::text, completed_at
FROM bonus_balance_conversions WHERE id = 1
UNION ALL
SELECT FALSE, COUNT(*), COALESCE(SUM(bonus_balance),0)::text,
 COALESCE(SUM(ROUND(bonus_balance / 10, 8)),0)::text, NULL::timestamptz
FROM users WHERE bonus_balance > 0
HAVING NOT EXISTS (SELECT 1 FROM bonus_balance_conversions WHERE id = 1)`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, errors.Join(sql.ErrNoRows, rows.Err())
	}
	result := &BonusBalanceConversion{}
	if err := rows.Scan(&result.Completed, &result.Users, &result.BeforeTotal, &result.AfterTotal, &result.CompletedAt); err != nil {
		return nil, err
	}
	return result, errors.Join(rows.Err(), rows.Close())
}

func (s *PaymentService) ConvertBonusBalances(ctx context.Context, adminID int64, confirmation string) (*BonusBalanceConversion, error) {
	if confirmation != BonusBalanceConversionConfirmation {
		return nil, infraerrors.BadRequest("BONUS_CONVERSION_CONFIRMATION_REQUIRED", "请输入 DIVIDE_BY_10 确认全站赠送余额转换")
	}
	if adminID <= 0 {
		return nil, infraerrors.Forbidden("BONUS_CONVERSION_ADMIN_REQUIRED", "需要管理员身份")
	}
	if s == nil || s.entClient == nil {
		return nil, ErrServiceUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	tx, err := s.entClient.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	if _, err := client.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
		return nil, err
	}
	// Serialize with grants, deductions, registrations and other conversions.
	// Wait only briefly for busy wallets; any failure rolls back everything.
	if _, err := client.ExecContext(ctx, `LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, infraerrors.Conflict("BONUS_CONVERSION_BUSY", "账户正在变更，请稍后重试")
	}
	res, err := client.ExecContext(ctx, `INSERT INTO bonus_balance_conversions(id, admin_user_id)
VALUES (1, $1) ON CONFLICT (id) DO NOTHING`, adminID)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, infraerrors.Conflict("BONUS_CONVERSION_ALREADY_COMPLETED", "赠送余额已完成转换，不能重复执行")
	}
	// Snapshot each exact NUMERIC value before touching wallets. Include retained
	// soft-deleted accounts, preventing restoration from reviving old units.
	if _, err := client.ExecContext(ctx, `INSERT INTO bonus_balance_conversion_users(user_id, balance_before, balance_after)
SELECT id, bonus_balance, ROUND(bonus_balance / 10, 8) FROM users WHERE bonus_balance > 0`); err != nil {
		return nil, err
	}
	if _, err := client.ExecContext(ctx, `/* bonus_source:conversion */ UPDATE users u
SET bonus_balance = c.balance_after, updated_at = NOW()
FROM bonus_balance_conversion_users c WHERE u.id = c.user_id`); err != nil {
		return nil, err
	}
	rows, err := client.QueryContext(ctx, `UPDATE bonus_balance_conversions SET
affected_users = (SELECT COUNT(*) FROM bonus_balance_conversion_users),
before_total = (SELECT COALESCE(SUM(balance_before),0) FROM bonus_balance_conversion_users),
after_total = (SELECT COALESCE(SUM(balance_after),0) FROM bonus_balance_conversion_users),
completed_at = clock_timestamp() WHERE id = 1
RETURNING TRUE, affected_users, before_total::text, after_total::text, completed_at`)
	if err != nil {
		return nil, err
	}
	result := &BonusBalanceConversion{}
	if !rows.Next() {
		return nil, errors.Join(sql.ErrNoRows, rows.Err(), rows.Close())
	}
	err = rows.Scan(&result.Completed, &result.Users, &result.BeforeTotal, &result.AfterTotal, &result.CompletedAt)
	if err := errors.Join(err, rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	if result.Users == 0 {
		return nil, infraerrors.BadRequest("BONUS_CONVERSION_EMPTY", "没有可转换的赠送余额")
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
