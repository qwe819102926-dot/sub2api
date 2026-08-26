package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// RewardTierStatus adds the user's claim state to a configured reward tier.
type RewardTierStatus struct {
	Threshold float64 `json:"threshold"`
	Bonus     float64 `json:"bonus"`
	Claimed   bool    `json:"claimed"`
}

// ConsumptionRewardStatus describes a user's cumulative-spend campaign progress.
type ConsumptionRewardStatus struct {
	Enabled    bool               `json:"enabled"`
	TotalSpent float64            `json:"total_spent"`
	Tiers      []RewardTierStatus `json:"tiers"`
}

// RewardCampaignStatus is returned to the dashboard and keeps recharge rewards
// visible without exposing any mutable campaign state to the user.
type RewardCampaignStatus struct {
	RechargeBonus     TieredRewardConfig      `json:"recharge_bonus"`
	ConsumptionReward ConsumptionRewardStatus `json:"consumption_reward"`
	BonusBalance      float64                 `json:"bonus_balance"`
}

// grantRechargeBonus credits the highest matching tier for one completed balance order.
// The unique order row makes retries and repeated fulfillment harmless.
func (s *PaymentService) grantRechargeBonus(ctx context.Context, order *dbent.PaymentOrder) error {
	if order == nil || order.OrderType != payment.OrderTypeBalance || s.configService == nil {
		return nil
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return err
	}
	tier := highestRewardTier(cfg.RechargeBonus, order.Amount)
	if tier == nil {
		return nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("start recharge bonus transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.Client().ExecContext(ctx, `
		INSERT INTO recharge_bonus_grants (order_id, user_id, threshold, bonus_amount)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (order_id) DO NOTHING
	`, order.ID, order.UserID, tier.Threshold, tier.Bonus)
	if err != nil {
		return fmt.Errorf("record recharge bonus grant: %w", err)
	}
	granted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read recharge bonus grant result: %w", err)
	}
	if granted == 1 {
		if _, err := tx.Client().ExecContext(ctx, `UPDATE users SET bonus_balance = COALESCE(bonus_balance, 0) + $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`, tier.Bonus, order.UserID); err != nil {
			return fmt.Errorf("credit recharge bonus: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recharge bonus: %w", err)
	}
	if granted == 1 {
		s.invalidateRewardBalance(ctx, order.UserID)
	}
	return nil
}

func (s *PaymentService) GetRewardCampaignStatus(ctx context.Context, userID int64) (*RewardCampaignStatus, error) {
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, err
	}
	status := &RewardCampaignStatus{RechargeBonus: cfg.RechargeBonus}
	if err := querySingleFloat(ctx, s.entClient, `SELECT COALESCE(bonus_balance, 0) FROM users WHERE id = $1`, &status.BonusBalance, userID); err != nil {
		return nil, fmt.Errorf("query bonus balance: %w", err)
	}
	if !cfg.ConsumptionReward.Enabled {
		return status, nil
	}
	totalSpent, err := queryTotalConsumption(ctx, s.entClient, userID)
	if err != nil {
		return nil, fmt.Errorf("query total consumption: %w", err)
	}
	claimed, err := queryClaimedConsumptionRewards(ctx, s.entClient, userID)
	if err != nil {
		return nil, fmt.Errorf("query consumption reward claims: %w", err)
	}
	status.ConsumptionReward.Enabled = true
	status.ConsumptionReward.TotalSpent = totalSpent
	for _, tier := range cfg.ConsumptionReward.Tiers {
		status.ConsumptionReward.Tiers = append(status.ConsumptionReward.Tiers, RewardTierStatus{
			Threshold: tier.Threshold,
			Bonus:     tier.Bonus,
			Claimed:   claimed[tier.Threshold],
		})
	}
	return status, nil
}

// ClaimConsumptionReward grants exactly one configured tier after recalculating
// current total spend inside the transaction.
func (s *PaymentService) ClaimConsumptionReward(ctx context.Context, userID int64, threshold float64) (float64, error) {
	if !isRechargeLotteryMoney(threshold) {
		return 0, infraerrors.BadRequest("INVALID_REWARD_TIER", "invalid consumption reward tier")
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return 0, err
	}
	if !cfg.ConsumptionReward.Enabled {
		return 0, infraerrors.BadRequest("CONSUMPTION_REWARD_DISABLED", "consumption reward campaign is not active")
	}
	tier := rewardTierByThreshold(cfg.ConsumptionReward.Tiers, threshold)
	if tier == nil {
		return 0, infraerrors.BadRequest("INVALID_REWARD_TIER", "consumption reward tier is not configured")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return 0, fmt.Errorf("start consumption reward transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	totalSpent, err := queryTotalConsumption(ctx, tx.Client(), userID)
	if err != nil {
		return 0, fmt.Errorf("query total consumption: %w", err)
	}
	if totalSpent+1e-9 < tier.Threshold {
		return 0, infraerrors.Conflict("CONSUMPTION_REWARD_NOT_READY", "consumption threshold has not been reached")
	}
	result, err := tx.Client().ExecContext(ctx, `
		INSERT INTO consumption_reward_claims (user_id, threshold, bonus_amount)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, threshold) DO NOTHING
	`, userID, tier.Threshold, tier.Bonus)
	if err != nil {
		return 0, fmt.Errorf("record consumption reward claim: %w", err)
	}
	claimed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read consumption reward claim result: %w", err)
	}
	if claimed != 1 {
		return 0, infraerrors.Conflict("CONSUMPTION_REWARD_ALREADY_CLAIMED", "consumption reward has already been claimed")
	}
	if _, err := tx.Client().ExecContext(ctx, `UPDATE users SET bonus_balance = COALESCE(bonus_balance, 0) + $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`, tier.Bonus, userID); err != nil {
		return 0, fmt.Errorf("credit consumption reward: %w", err)
	}
	var balance float64
	if err := querySingleFloat(ctx, tx.Client(), `SELECT balance FROM users WHERE id = $1`, &balance, userID); err != nil {
		return 0, fmt.Errorf("read user balance: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit consumption reward: %w", err)
	}
	s.invalidateRewardBalance(ctx, userID)
	return balance, nil
}

type rawSQLQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func querySingleFloat(ctx context.Context, client rawSQLQueryer, query string, dest *float64, args ...any) error {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Scan(dest)
}

func (s *PaymentService) invalidateRewardBalance(ctx context.Context, userID int64) {
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			// Grants are committed before cache invalidation; normal cache expiry recovers on failure.
			return
		}
	}
}

func highestRewardTier(cfg TieredRewardConfig, amount float64) *RewardTier {
	if !cfg.Enabled {
		return nil
	}
	var match *RewardTier
	for i := range cfg.Tiers {
		if amount+1e-9 >= cfg.Tiers[i].Threshold {
			match = &cfg.Tiers[i]
		}
	}
	return match
}

func rewardTierByThreshold(tiers []RewardTier, threshold float64) *RewardTier {
	for i := range tiers {
		if math.Abs(tiers[i].Threshold-threshold) < 1e-9 {
			return &tiers[i]
		}
	}
	return nil
}

func queryTotalConsumption(ctx context.Context, client *dbent.Client, userID int64) (float64, error) {
	rows, err := client.QueryContext(ctx, `SELECT COALESCE(SUM(actual_cost), 0) FROM usage_logs WHERE user_id = $1`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var total float64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func queryClaimedConsumptionRewards(ctx context.Context, client *dbent.Client, userID int64) (map[float64]bool, error) {
	rows, err := client.QueryContext(ctx, `SELECT threshold FROM consumption_reward_claims WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	claimed := make(map[float64]bool)
	for rows.Next() {
		var threshold float64
		if err := rows.Scan(&threshold); err != nil {
			return nil, err
		}
		claimed[threshold] = true
	}
	return claimed, rows.Err()
}
