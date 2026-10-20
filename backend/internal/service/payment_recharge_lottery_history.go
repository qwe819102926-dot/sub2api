package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AdminRechargeLotteryDraw struct {
	ID          int64     `json:"id"`
	OrderID     *int64    `json:"order_id"`
	PrizeAmount float64   `json:"prize_amount"`
	IsWinner    bool      `json:"is_winner"`
	CreatedAt   time.Time `json:"created_at"`
}

type AdminRechargeLotteryHistory struct {
	Items            []AdminRechargeLotteryDraw `json:"items"`
	Total            int64                      `json:"total"`
	Page             int                        `json:"page"`
	PageSize         int                        `json:"page_size"`
	TotalPages       int64                      `json:"total_pages"`
	WinningDraws     int64                      `json:"winning_draws"`
	TotalPrizeAmount float64                    `json:"total_prize_amount"`
}

// GetUserRechargeLotteryHistory includes historical draws even when the campaign
// is disabled. Read totals and the page in one statement to use the same snapshot.
func (s *PaymentService) GetUserRechargeLotteryHistory(ctx context.Context, userID int64, page, pageSize int) (*AdminRechargeLotteryHistory, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 20
	}
	if int64(page-1) > (1<<63-1)/int64(pageSize) {
		return nil, infraerrors.BadRequest("INVALID_PAGINATION", "page is too large")
	}
	history := &AdminRechargeLotteryHistory{
		Items: make([]AdminRechargeLotteryDraw, 0), Page: page, PageSize: pageSize,
	}
	rows, err := s.entClient.QueryContext(ctx, `
		WITH summary AS (
			SELECT COUNT(*) AS total,
			       COUNT(*) FILTER (WHERE is_winner) AS winning_draws,
			       COALESCE(SUM(prize_amount), 0) AS total_prize_amount
			FROM recharge_lottery_draws WHERE user_id = $1
		)
		SELECT s.total, s.winning_draws, s.total_prize_amount,
		       d.id, d.order_id, d.prize_amount, d.is_winner, d.created_at
		FROM summary s
		LEFT JOIN (
			SELECT id, order_id, prize_amount, is_winner, created_at
			FROM recharge_lottery_draws WHERE user_id = $1
			ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3
		) d ON TRUE
		ORDER BY d.created_at DESC, d.id DESC
	`, userID, pageSize, int64(page-1)*int64(pageSize))
	if err != nil {
		return nil, fmt.Errorf("list user recharge lottery history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, orderID sql.NullInt64
		var amount sql.NullFloat64
		var winner sql.NullBool
		var createdAt sql.NullTime
		if err := rows.Scan(&history.Total, &history.WinningDraws, &history.TotalPrizeAmount,
			&id, &orderID, &amount, &winner, &createdAt); err != nil {
			return nil, fmt.Errorf("scan user recharge lottery history: %w", err)
		}
		if !id.Valid {
			continue
		}
		draw := AdminRechargeLotteryDraw{
			ID: id.Int64, PrizeAmount: amount.Float64, IsWinner: winner.Bool, CreatedAt: createdAt.Time,
		}
		if orderID.Valid {
			value := orderID.Int64
			draw.OrderID = &value
		}
		history.Items = append(history.Items, draw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user recharge lottery history: %w", err)
	}
	history.TotalPages = (history.Total + int64(pageSize) - 1) / int64(pageSize)
	return history, nil
}
