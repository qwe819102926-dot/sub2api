package service

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newPaymentServiceForSQLMock(db *sql.DB) *PaymentService {
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	return &PaymentService{entClient: client}
}

func TestQueryTotalConsumptionCountsPrincipalBalanceUsageOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(CASE WHEN billing_type = 0 THEN COALESCE\(wallet_cost, actual_cost\) ELSE 0 END\), 0\).*FROM usage_logs.*WHERE user_id = \$1`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(12.5))

	total, err := queryTotalConsumption(context.Background(), client, 42)
	require.NoError(t, err)
	require.Equal(t, 12.5, total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDailyCheckinRewardCycle(t *testing.T) {
	wantMilestones := map[int]float64{3: 1, 7: 3, 15: 5, 30: 10}
	for day := 0; day <= 300; day++ {
		reward := dailyCheckinReward(day)
		if amount, ok := wantMilestones[(day-1)%30+1]; ok {
			require.Equal(t, amount, reward)
			require.Equal(t, amount, dailyCheckinRewardPreview(day))
		} else {
			require.GreaterOrEqual(t, reward, 0.2)
			require.LessOrEqual(t, reward, 0.5)
			require.InDelta(t, math.Round(reward*100), reward*100, 1e-9)
			require.Equal(t, 0.2, dailyCheckinRewardPreview(day))
		}
	}
	for _, milestone := range dailyCheckinMilestones {
		require.Equal(t, milestone.Reward, dailyCheckinReward(milestone.Day))
	}
}

// Capture the random insert amount and require the balance credit to match it.
type checkinRewardArgument struct {
	amount float64
}

func (r *checkinRewardArgument) Match(value driver.Value) bool {
	amount, ok := value.(float64)
	if !ok || amount < 0.2 || amount > 0.5 || math.Abs(amount*100-math.Round(amount*100)) > 1e-9 {
		return false
	}
	if r.amount == 0 {
		r.amount = amount
	}
	return r.amount == amount
}

func TestDailyCheckinBusinessDateAtShanghaiMidnight(t *testing.T) {
	before := time.Date(2026, 10, 31, 15, 59, 59, 0, time.UTC)
	require.Equal(t, "2026-10-31", dailyCheckinDate(before).Format("2006-01-02"))
	require.Equal(t, "2026-11-01", dailyCheckinDate(before.Add(time.Second)).Format("2006-01-02"))
}

func TestGetDailyCheckinStatusReturnsRowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	service := newPaymentServiceForSQLMock(db)
	t.Cleanup(func() { _ = service.entClient.Close() })

	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day, reward_amount, created_at.*FROM user_daily_checkins`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount", "created_at"}).
			AddRow(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), 1, 0.10, time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)).
			AddRow(time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), 2, 0.10, time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)).
			RowError(1, errors.New("iteration failed")))

	_, err = service.GetDailyCheckinStatus(context.Background(), 42, "2026-09")
	require.ErrorContains(t, err, "iteration failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimDailyCheckinDuplicateIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	service := newPaymentServiceForSQLMock(db)
	t.Cleanup(func() { _ = service.entClient.Close() })
	today := dailyCheckinDate(time.Now())
	createdAt := today.Add(9 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day.*FOR UPDATE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}).AddRow(today, 1))
	mock.ExpectQuery(`(?s)INSERT INTO user_daily_checkins.*ON CONFLICT \(user_id, checkin_date\) DO NOTHING.*RETURNING id`).
		WithArgs(int64(42), today, 1, &checkinRewardArgument{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day, reward_amount, created_at.*FROM user_daily_checkins`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount", "created_at"}).AddRow(today, 1, 0.10, createdAt))
	mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(reward_amount\), 0\) FROM user_daily_checkins`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "total"}).AddRow(1, 0.10))
	mock.ExpectQuery(`SELECT checkin_date, streak_day, reward_amount FROM user_daily_checkins`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount"}).AddRow(today, 1, 0.10))

	status, err := service.ClaimDailyCheckin(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, status.CheckedInToday)
	require.Equal(t, 1, status.TotalDays)
	require.Equal(t, 0.10, status.TodayReward, "duplicate claims preserve the amount originally credited")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimDailyCheckinRollsBackWhenUserIsNotUpdated(t *testing.T) {
	reward := &checkinRewardArgument{}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	service := newPaymentServiceForSQLMock(db)
	t.Cleanup(func() { _ = service.entClient.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day.*FOR UPDATE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}))
	mock.ExpectQuery(`(?s)INSERT INTO user_daily_checkins.*RETURNING id`).
		WithArgs(int64(42), sqlmock.AnyArg(), 1, reward).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec(`UPDATE users SET bonus_balance = COALESCE\(bonus_balance, 0\).*WHERE id = \$2 AND deleted_at IS NULL`).
		WithArgs(reward, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err = service.ClaimDailyCheckin(context.Background(), 42)
	require.ErrorContains(t, err, "user is missing or inactive")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimDailyCheckinResetsStreakAfterBreak(t *testing.T) {
	reward := &checkinRewardArgument{}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	service := newPaymentServiceForSQLMock(db)
	t.Cleanup(func() { _ = service.entClient.Close() })
	today := dailyCheckinDate(time.Now())
	previous := today.AddDate(0, 0, -2)
	createdAt := today.Add(9 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day.*FOR UPDATE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}).AddRow(previous, 4))
	mock.ExpectQuery(`(?s)INSERT INTO user_daily_checkins.*RETURNING id`).
		WithArgs(int64(42), today, 1, reward).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(8))
	mock.ExpectExec(`UPDATE users SET bonus_balance = COALESCE\(bonus_balance, 0\).*WHERE id = \$2 AND deleted_at IS NULL`).
		WithArgs(reward, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day, reward_amount, created_at.*FROM user_daily_checkins`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount", "created_at"}).AddRow(today, 1, 0.37, createdAt))
	mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(reward_amount\), 0\) FROM user_daily_checkins`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "total"}).AddRow(5, 11.0))
	mock.ExpectQuery(`SELECT checkin_date, streak_day, reward_amount FROM user_daily_checkins`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount"}).AddRow(today, 1, 0.37))

	status, err := service.ClaimDailyCheckin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, status.CurrentStreak)
	require.Equal(t, 1, status.Records[0].StreakDay)
	require.Equal(t, 0.37, status.TodayReward)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetDailyCheckinHistoricalMonthPreservesToday(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	service := newPaymentServiceForSQLMock(db)
	t.Cleanup(func() { _ = service.entClient.Close() })
	today := dailyCheckinDate(time.Now())
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day, reward_amount, created_at.*FROM user_daily_checkins`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount", "created_at"}))
	mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(reward_amount\), 0\) FROM user_daily_checkins`).
		WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"count", "total"}).AddRow(30, 35.8))
	mock.ExpectQuery(`SELECT checkin_date, streak_day, reward_amount FROM user_daily_checkins`).
		WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount"}).AddRow(today, 30, 100.0))
	status, err := service.GetDailyCheckinStatus(context.Background(), 42, "2025-01")
	require.NoError(t, err)
	require.True(t, status.CheckedInToday)
	require.Equal(t, 30, status.CurrentStreak)
	require.Equal(t, 30, status.CycleDay)
	require.Equal(t, 100.0, status.TodayReward, "historical credits retain the original reward amount")
	require.Equal(t, today.Format("2006-01-02"), status.Today)
	require.Empty(t, status.Records)
	require.Len(t, status.Milestones, 4)
	require.Equal(t, []float64{0.2, 0.5}, status.OrdinaryRewards)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimDailyCheckinMilestoneCreditsOnlyBonusBalance(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	service := newPaymentServiceForSQLMock(db)
	t.Cleanup(func() { _ = service.entClient.Close() })
	today := dailyCheckinDate(time.Now())
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day.*FOR UPDATE`).WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}).AddRow(today.AddDate(0, 0, -1), 29))
	mock.ExpectQuery(`(?s)INSERT INTO user_daily_checkins.*RETURNING id`).WithArgs(int64(42), today, 30, 10.0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))
	mock.ExpectExec(`UPDATE users SET bonus_balance = COALESCE\(bonus_balance, 0\) \+ \$1, updated_at = NOW\(\) WHERE id = \$2 AND deleted_at IS NULL`).
		WithArgs(10.0, int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day, reward_amount, created_at.*FROM user_daily_checkins`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount", "created_at"}).AddRow(today, 30, 10.0, today))
	mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(reward_amount\), 0\) FROM user_daily_checkins`).
		WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"count", "total"}).AddRow(30, 32.0))
	mock.ExpectQuery(`SELECT checkin_date, streak_day, reward_amount FROM user_daily_checkins`).WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount"}).AddRow(today, 30, 10.0))
	status, err := service.ClaimDailyCheckin(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, status.CheckedInToday)
	require.Equal(t, 10.0, status.TodayReward)
	require.Equal(t, 30, status.CurrentStreak)
	require.NoError(t, mock.ExpectationsWereMet())
}
