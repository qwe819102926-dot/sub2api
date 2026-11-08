package service

import "context"

// ExpireBonusBalances sweeps a bounded batch. Billing also expires a user's
// wallet under its transaction lock, and read paths exclude expired lots even
// when the periodic job is delayed. Each user commits independently.
func (s *PaymentService) ExpireBonusBalances(ctx context.Context) (int, error) {
	if s == nil || s.entClient == nil {
		return 0, ErrServiceUnavailable
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT DISTINCT user_id FROM bonus_balance_lots
WHERE remaining>0 AND expires_at<=statement_timestamp() ORDER BY user_id LIMIT 500`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return 0, err
	}
	if closeErr != nil {
		return 0, closeErr
	}
	for i, id := range ids {
		if _, err := s.entClient.ExecContext(ctx, `/* bonus_source:expiration */ SELECT expire_user_bonus_balance($1)`, id); err != nil {
			return i, err
		}
	}
	return len(ids), nil
}
