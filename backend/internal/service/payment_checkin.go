package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DailyCheckinRewardCycle is the recurring seven-day bonus-balance reward.
// Check-in rewards are always credited to bonus_balance, never principal balance.
var dailyCheckinRewardCycle = []float64{0.10, 0.10, 0.20, 0.20, 0.30, 0.50, 1.00}

type DailyCheckinRecord struct {
	Date      string  `json:"date"`
	StreakDay int     `json:"streak_day"`
	Reward    float64 `json:"reward"`
	CreatedAt string  `json:"created_at"`
}

type DailyCheckinStatus struct {
	CheckedInToday bool                 `json:"checked_in_today"`
	CurrentStreak  int                  `json:"current_streak"`
	TotalDays      int                  `json:"total_days"`
	TotalReward    float64              `json:"total_reward"`
	CycleDay       int                  `json:"cycle_day"`
	CycleLength    int                  `json:"cycle_length"`
	TodayReward    float64              `json:"today_reward"`
	RewardCycle    []float64            `json:"reward_cycle"`
	Month          string               `json:"month"`
	Records        []DailyCheckinRecord `json:"records"`
}

func dailyCheckinReward(streakDay int) float64 {
	if streakDay < 1 {
		streakDay = 1
	}
	return dailyCheckinRewardCycle[(streakDay-1)%len(dailyCheckinRewardCycle)]
}

func (s *PaymentService) GetDailyCheckinStatus(ctx context.Context, userID int64, month string) (*DailyCheckinStatus, error) {
	now := time.Now()
	if month == "" {
		month = now.Format("2006-01")
	}
	monthStart, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, fmt.Errorf("invalid check-in month: %w", err)
	}
	monthEnd := monthStart.AddDate(0, 1, 0)
	status := &DailyCheckinStatus{
		CycleLength: len(dailyCheckinRewardCycle),
		RewardCycle: append([]float64(nil), dailyCheckinRewardCycle...),
		Month:       month,
		Records:     make([]DailyCheckinRecord, 0),
	}
	rows, err := s.entClient.QueryContext(ctx, `
		SELECT checkin_date, streak_day, reward_amount, created_at
		FROM user_daily_checkins
		WHERE user_id = $1 AND checkin_date >= $2 AND checkin_date < $3
		ORDER BY checkin_date ASC
	`, userID, monthStart, monthEnd)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var date time.Time
		var record DailyCheckinRecord
		var createdAt time.Time
		if err := rows.Scan(&date, &record.StreakDay, &record.Reward, &createdAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		record.Date = date.Format("2006-01-02")
		record.CreatedAt = createdAt.Format(time.RFC3339)
		status.Records = append(status.Records, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	var totalDays int
	rows, err = s.entClient.QueryContext(ctx, `SELECT COUNT(*), COALESCE(SUM(reward_amount), 0) FROM user_daily_checkins WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		_ = rows.Close()
		return nil, rows.Err()
	}
	if err := rows.Scan(&totalDays, &status.TotalReward); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	status.TotalDays = totalDays
	if len(status.Records) > 0 {
		last := status.Records[len(status.Records)-1]
		lastDate, _ := time.Parse("2006-01-02", last.Date)
		if lastDate.AddDate(0, 0, 1).Equal(now.Truncate(24*time.Hour)) || lastDate.Equal(now.Truncate(24*time.Hour)) {
			status.CurrentStreak = last.StreakDay
		}
	}
	// The current streak may be in a previous month, so read the latest row when
	// the selected month does not contain it.
	var latestDate time.Time
	var latestStreak int
	rows, err = s.entClient.QueryContext(ctx, `SELECT checkin_date, streak_day FROM user_daily_checkins WHERE user_id = $1 ORDER BY checkin_date DESC LIMIT 1`, userID)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		if err := rows.Scan(&latestDate, &latestStreak); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	if !latestDate.IsZero() {
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		latest := time.Date(latestDate.Year(), latestDate.Month(), latestDate.Day(), 0, 0, 0, 0, time.UTC)
		if latest.Equal(today) || latest.AddDate(0, 0, 1).Equal(today) {
			status.CurrentStreak = latestStreak
		}
	}
	for _, record := range status.Records {
		if record.Date == now.Format("2006-01-02") {
			status.CheckedInToday = true
			status.CycleDay = ((record.StreakDay - 1) % status.CycleLength) + 1
			status.TodayReward = record.Reward
		}
	}
	if !status.CheckedInToday {
		status.CycleDay = (status.CurrentStreak % status.CycleLength) + 1
		status.TodayReward = dailyCheckinReward(status.CurrentStreak + 1)
	}
	return status, nil
}

func (s *PaymentService) ClaimDailyCheckin(ctx context.Context, userID int64) (*DailyCheckinStatus, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start check-in transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var previousDate time.Time
	var previousStreak int
	rows, queryErr := tx.Client().QueryContext(ctx, `SELECT checkin_date, streak_day FROM user_daily_checkins WHERE user_id = $1 ORDER BY checkin_date DESC LIMIT 1 FOR UPDATE`, userID)
	if queryErr != nil {
		return nil, fmt.Errorf("query previous check-in: %w", queryErr)
	}
	if rows.Next() {
		if err := rows.Scan(&previousDate, &previousStreak); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("read previous check-in: %w", err)
	}
	_ = rows.Close()
	streak := 1
	if !previousDate.IsZero() {
		previous := time.Date(previousDate.Year(), previousDate.Month(), previousDate.Day(), 0, 0, 0, 0, time.UTC)
		if previous.AddDate(0, 0, 1).Equal(today) {
			streak = previousStreak + 1
		}
	}
	reward := dailyCheckinReward(streak)
	var recordID int64
	rows, err = tx.Client().QueryContext(ctx, `
		INSERT INTO user_daily_checkins (user_id, checkin_date, streak_day, reward_amount)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, checkin_date) DO NOTHING
		RETURNING id
	`, userID, today, streak, reward)
	if err == nil {
		if rows.Next() {
			err = rows.Scan(&recordID)
		} else {
			err = rows.Err()
			if err == nil {
				err = sql.ErrNoRows
			}
		}
		_ = rows.Close()
	}
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("record check-in: %w", err)
		}
		// A duplicate is a successful idempotent response; no second bonus is added.
		_ = tx.Rollback()
		status, statusErr := s.GetDailyCheckinStatus(ctx, userID, today.Format("2006-01"))
		if statusErr != nil {
			return nil, fmt.Errorf("check existing check-in: %w", statusErr)
		}
		return status, nil
	}
	if recordID == 0 {
		return nil, fmt.Errorf("check-in was not recorded")
	}
	result, err := tx.Client().ExecContext(ctx, `UPDATE users SET bonus_balance = COALESCE(bonus_balance, 0) + $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`, reward, userID)
	if err != nil {
		return nil, fmt.Errorf("credit check-in bonus: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read credited check-in bonus result: %w", err)
	}
	if affected != 1 {
		return nil, fmt.Errorf("credit check-in bonus: user is missing or inactive")
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit check-in: %w", err)
	}
	s.invalidateRewardBalance(ctx, userID)
	return s.GetDailyCheckinStatus(ctx, userID, today.Format("2006-01"))
}
