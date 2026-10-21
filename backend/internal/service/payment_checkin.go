package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	dailyCheckinCycleLength = 30
	dailyCheckinBaseReward  = 10.0
)

var dailyCheckinLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type DailyCheckinMilestone struct {
	Day    int     `json:"day"`
	Reward float64 `json:"reward"`
}

// Milestone rewards are the entire gift for that day, replacing the daily 10.
var dailyCheckinMilestones = []DailyCheckinMilestone{{Day: 3, Reward: 20}, {Day: 7, Reward: 30}, {Day: 15, Reward: 50}, {Day: 30, Reward: 100}}

var dailyCheckinRewardCycle = func() []float64 {
	cycle := make([]float64, dailyCheckinCycleLength)
	for i := range cycle {
		cycle[i] = dailyCheckinBaseReward
	}
	for _, milestone := range dailyCheckinMilestones {
		cycle[milestone.Day-1] = milestone.Reward
	}
	return cycle
}()

type DailyCheckinRecord struct {
	Date      string  `json:"date"`
	StreakDay int     `json:"streak_day"`
	Reward    float64 `json:"reward"`
	CreatedAt string  `json:"created_at"`
}

type DailyCheckinStatus struct {
	CheckedInToday  bool                    `json:"checked_in_today"`
	CurrentStreak   int                     `json:"current_streak"`
	TotalDays       int                     `json:"total_days"`
	TotalReward     float64                 `json:"total_reward"`
	CycleDay        int                     `json:"cycle_day"`
	CycleLength     int                     `json:"cycle_length"`
	TodayReward     float64                 `json:"today_reward"`
	RewardCycle     []float64               `json:"reward_cycle"`
	Month           string                  `json:"month"`
	Records         []DailyCheckinRecord    `json:"records"`
	Today           string                  `json:"today"`
	NextResetAt     string                  `json:"next_reset_at"`
	OrdinaryRewards []float64               `json:"ordinary_rewards"`
	Milestones      []DailyCheckinMilestone `json:"milestones"`
}

// Store the business calendar date as UTC midnight so PostgreSQL DATE values
// are independent of the application host's timezone.
func dailyCheckinDate(now time.Time) time.Time {
	local := now.In(dailyCheckinLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func dailyCheckinReward(streakDay int) float64 {
	if streakDay < 1 {
		streakDay = 1
	}
	return dailyCheckinRewardCycle[(streakDay-1)%len(dailyCheckinRewardCycle)]
}

func (s *PaymentService) GetDailyCheckinStatus(ctx context.Context, userID int64, month string) (*DailyCheckinStatus, error) {
	now := time.Now().In(dailyCheckinLocation)
	today := dailyCheckinDate(now)
	if month == "" {
		month = now.Format("2006-01")
	}
	monthStart, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, fmt.Errorf("invalid check-in month: %w", err)
	}
	monthEnd := monthStart.AddDate(0, 1, 0)
	status := &DailyCheckinStatus{
		CycleLength:     len(dailyCheckinRewardCycle),
		RewardCycle:     append([]float64(nil), dailyCheckinRewardCycle...),
		Month:           month,
		Records:         make([]DailyCheckinRecord, 0),
		Today:           today.Format("2006-01-02"),
		NextResetAt:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, dailyCheckinLocation).Format(time.RFC3339),
		OrdinaryRewards: []float64{dailyCheckinBaseReward},
		Milestones:      append([]DailyCheckinMilestone(nil), dailyCheckinMilestones...),
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
		err := rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("missing check-in totals")
	}
	if err := rows.Scan(&totalDays, &status.TotalReward); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	status.TotalDays = totalDays
	// Today's claim and streak must not change when browsing another month.
	var latestDate time.Time
	var latestStreak int
	var latestReward float64
	rows, err = s.entClient.QueryContext(ctx, `SELECT checkin_date, streak_day, reward_amount FROM user_daily_checkins WHERE user_id = $1 ORDER BY checkin_date DESC LIMIT 1`, userID)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		if err := rows.Scan(&latestDate, &latestStreak, &latestReward); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	if !latestDate.IsZero() {
		latest := time.Date(latestDate.Year(), latestDate.Month(), latestDate.Day(), 0, 0, 0, 0, time.UTC)
		if latest.Equal(today) || latest.AddDate(0, 0, 1).Equal(today) {
			status.CurrentStreak = latestStreak
		}
		if latest.Equal(today) {
			status.CheckedInToday = true
			status.CycleDay = ((latestStreak - 1) % status.CycleLength) + 1
			status.TodayReward = latestReward
		}
	}
	if !status.CheckedInToday {
		status.CycleDay = (status.CurrentStreak % status.CycleLength) + 1
		status.TodayReward = dailyCheckinReward(status.CurrentStreak + 1)
	}
	return status, nil
}

func (s *PaymentService) ClaimDailyCheckin(ctx context.Context, userID int64) (*DailyCheckinStatus, error) {
	today := dailyCheckinDate(time.Now())
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
	result, err := tx.Client().ExecContext(ctx, `/* bonus_source:checkin */ UPDATE users SET bonus_balance = COALESCE(bonus_balance, 0) + $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`, reward, userID)
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
