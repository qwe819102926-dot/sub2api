package service

import (
	"context"
	"database/sql"
	"errors"
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
	want := []float64{0.10, 0.10, 0.20, 0.20, 0.30, 0.50, 1.00, 0.10}
	for day, expected := range want {
		require.Equal(t, expected, dailyCheckinReward(day+1))
	}
	require.Equal(t, 0.10, dailyCheckinReward(0))
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
	today := time.Now().UTC().Truncate(24 * time.Hour)
	createdAt := today.Add(9 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day.*FOR UPDATE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}).AddRow(today, 1))
	mock.ExpectQuery(`(?s)INSERT INTO user_daily_checkins.*ON CONFLICT \(user_id, checkin_date\) DO NOTHING.*RETURNING id`).
		WithArgs(int64(42), sqlmock.AnyArg(), 1, 0.10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day, reward_amount, created_at.*FROM user_daily_checkins`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount", "created_at"}).AddRow(today, 1, 0.10, createdAt))
	mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(reward_amount\), 0\) FROM user_daily_checkins`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "total"}).AddRow(1, 0.10))
	mock.ExpectQuery(`SELECT checkin_date, streak_day FROM user_daily_checkins`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}).AddRow(today, 1))

	status, err := service.ClaimDailyCheckin(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, status.CheckedInToday)
	require.Equal(t, 1, status.TotalDays)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimDailyCheckinRollsBackWhenUserIsNotUpdated(t *testing.T) {
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
		WithArgs(int64(42), sqlmock.AnyArg(), 1, 0.10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec(`UPDATE users SET bonus_balance = COALESCE\(bonus_balance, 0\).*WHERE id = \$2 AND deleted_at IS NULL`).
		WithArgs(0.10, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err = service.ClaimDailyCheckin(context.Background(), 42)
	require.ErrorContains(t, err, "user is missing or inactive")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimDailyCheckinResetsStreakAfterBreak(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	service := newPaymentServiceForSQLMock(db)
	t.Cleanup(func() { _ = service.entClient.Close() })
	today := time.Now().UTC().Truncate(24 * time.Hour)
	previous := today.AddDate(0, 0, -2)
	createdAt := today.Add(9 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day.*FOR UPDATE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}).AddRow(previous, 4))
	mock.ExpectQuery(`(?s)INSERT INTO user_daily_checkins.*RETURNING id`).
		WithArgs(int64(42), sqlmock.AnyArg(), 1, 0.10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(8))
	mock.ExpectExec(`UPDATE users SET bonus_balance = COALESCE\(bonus_balance, 0\).*WHERE id = \$2 AND deleted_at IS NULL`).
		WithArgs(0.10, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day, reward_amount, created_at.*FROM user_daily_checkins`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day", "reward_amount", "created_at"}).AddRow(today, 1, 0.10, createdAt))
	mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(reward_amount\), 0\) FROM user_daily_checkins`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "total"}).AddRow(5, 1.10))
	mock.ExpectQuery(`SELECT checkin_date, streak_day FROM user_daily_checkins`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}).AddRow(today, 1))

	status, err := service.ClaimDailyCheckin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, status.CurrentStreak)
	require.Equal(t, 1, status.Records[0].StreakDay)
	require.NoError(t, mock.ExpectationsWereMet())
}
