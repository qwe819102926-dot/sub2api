package service

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"fmt"
	"math"
	"math/big"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RechargeLotteryDraw struct {
	PrizeAmount float64   `json:"prize_amount"`
	IsWinner    bool      `json:"is_winner"`
	CreatedAt   time.Time `json:"created_at"`
}

type RechargeLotteryStatus struct {
	Enabled        bool                   `json:"enabled"`
	Threshold      float64                `json:"threshold"`
	Prizes         []RechargeLotteryPrize `json:"prizes"`
	RemainingDraws int                    `json:"remaining_draws"`
	RecentDraws    []RechargeLotteryDraw  `json:"recent_draws"`
}

type RechargeLotteryDrawResult struct {
	IsWinner       bool    `json:"is_winner"`
	PrizeAmount    float64 `json:"prize_amount"`
	RemainingDraws int     `json:"remaining_draws"`
}

// grantRechargeLotteryEntries issues one chance for each completed threshold of a balance recharge.
// The unique order_id constraint makes webhook retries harmless.
func (s *PaymentService) grantRechargeLotteryEntries(ctx context.Context, order *dbent.PaymentOrder) error {
	if order == nil || order.OrderType != payment.OrderTypeBalance {
		return nil
	}
	// PaymentService unit users may omit the optional payment configuration
	// service; with no configuration there is no lottery campaign to grant.
	if s.configService == nil {
		return nil
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return err
	}
	lottery := cfg.RechargeLottery
	if !lottery.Enabled || lottery.Threshold <= 0 {
		return nil
	}
	drawCount := int(math.Floor(order.Amount / lottery.Threshold))
	if drawCount <= 0 {
		return nil
	}
	_, err = s.entClient.ExecContext(ctx, `
		INSERT INTO recharge_lottery_entries (order_id, user_id, draw_count)
		VALUES ($1, $2, $3)
		ON CONFLICT (order_id) DO NOTHING
	`, order.ID, order.UserID, drawCount)
	if err != nil {
		return fmt.Errorf("grant recharge lottery entries: %w", err)
	}
	return nil
}

func (s *PaymentService) GetRechargeLotteryStatus(ctx context.Context, userID int64) (*RechargeLotteryStatus, error) {
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, err
	}
	status := &RechargeLotteryStatus{
		Enabled:   cfg.RechargeLottery.Enabled,
		Threshold: cfg.RechargeLottery.Threshold,
		Prizes:    cfg.RechargeLottery.Prizes,
	}
	if !status.Enabled {
		return status, nil
	}
	remaining, err := queryRechargeLotteryRemaining(ctx, s.entClient, userID)
	if err != nil {
		return nil, fmt.Errorf("get recharge lottery draws: %w", err)
	}
	status.RemainingDraws = remaining
	rows, err := s.entClient.QueryContext(ctx, `
		SELECT prize_amount, is_winner, created_at
		FROM recharge_lottery_draws
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list recharge lottery draws: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var draw RechargeLotteryDraw
		if err := rows.Scan(&draw.PrizeAmount, &draw.IsWinner, &draw.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recharge lottery draw: %w", err)
		}
		status.RecentDraws = append(status.RecentDraws, draw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recharge lottery draws: %w", err)
	}
	return status, nil
}

func (s *PaymentService) DrawRechargeLottery(ctx context.Context, userID int64) (*RechargeLotteryDrawResult, error) {
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.RechargeLottery.Enabled {
		return nil, infraerrors.BadRequest("RECHARGE_LOTTERY_DISABLED", "recharge lottery is not active")
	}
	if err := validateRechargeLotteryConfig(cfg.RechargeLottery); err != nil {
		return nil, err
	}

	prize, err := selectRechargeLotteryPrize(cfg.RechargeLottery.Prizes)
	if err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start recharge lottery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	entryID, orderID, err := queryRechargeLotteryEntry(ctx, txClient, userID)
	if err == sql.ErrNoRows {
		return nil, infraerrors.Conflict("LOTTERY_NO_DRAW_CHANCES", "no recharge lottery chances available")
	}
	if err != nil {
		return nil, fmt.Errorf("lock recharge lottery entry: %w", err)
	}
	updated, err := txClient.ExecContext(ctx, `
		UPDATE recharge_lottery_entries
		SET used_count = used_count + 1
		WHERE id = $1 AND used_count < draw_count
	`, entryID)
	if err != nil {
		return nil, fmt.Errorf("consume recharge lottery chance: %w", err)
	}
	if affected, _ := updated.RowsAffected(); affected != 1 {
		return nil, infraerrors.Conflict("LOTTERY_NO_DRAW_CHANCES", "no recharge lottery chances available")
	}

	amount := 0.0
	isWinner := prize != nil
	if prize != nil {
		amount = prize.Amount
		if _, err := txClient.User.Update().Where(user.IDEQ(userID)).AddBalance(amount).Save(ctx); err != nil {
			return nil, fmt.Errorf("credit recharge lottery prize: %w", err)
		}
	}
	if _, err := txClient.ExecContext(ctx, `
		INSERT INTO recharge_lottery_draws (user_id, entry_id, order_id, prize_amount, is_winner)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, entryID, orderID, amount, isWinner); err != nil {
		return nil, fmt.Errorf("record recharge lottery draw: %w", err)
	}
	remaining, err := queryRechargeLotteryRemaining(ctx, txClient, userID)
	if err != nil {
		return nil, fmt.Errorf("count remaining recharge lottery draws: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit recharge lottery draw: %w", err)
	}
	return &RechargeLotteryDrawResult{IsWinner: isWinner, PrizeAmount: amount, RemainingDraws: remaining}, nil
}

func queryRechargeLotteryEntry(ctx context.Context, client *dbent.Client, userID int64) (int64, int64, error) {
	rows, err := client.QueryContext(ctx, `
		SELECT id, order_id
		FROM recharge_lottery_entries
		WHERE user_id = $1 AND used_count < draw_count
		ORDER BY created_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, userID)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, err
		}
		return 0, 0, sql.ErrNoRows
	}
	var entryID, orderID int64
	if err := rows.Scan(&entryID, &orderID); err != nil {
		return 0, 0, err
	}
	return entryID, orderID, nil
}

func queryRechargeLotteryRemaining(ctx context.Context, client *dbent.Client, userID int64) (int, error) {
	rows, err := client.QueryContext(ctx, `
		SELECT COALESCE(SUM(draw_count - used_count), 0)
		FROM recharge_lottery_entries WHERE user_id = $1
	`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, sql.ErrNoRows
	}
	var remaining int
	if err := rows.Scan(&remaining); err != nil {
		return 0, err
	}
	return remaining, nil
}

func selectRechargeLotteryPrize(prizes []RechargeLotteryPrize) (*RechargeLotteryPrize, error) {
	roll, err := cryptorand.Int(cryptorand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return nil, fmt.Errorf("secure recharge lottery random source: %w", err)
	}
	value := float64(roll.Int64()) / 10_000
	cumulative := 0.0
	for i := range prizes {
		cumulative += prizes[i].Probability
		if value < cumulative {
			return &prizes[i], nil
		}
	}
	return nil, nil
}
