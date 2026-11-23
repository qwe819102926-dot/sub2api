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
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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
			require.LessOrEqual(t, reward, 0.3)
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
	if !ok || amount < 0.2 || amount > 0.3 || math.Abs(amount*100-math.Round(amount*100)) > 1e-9 {
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

func TestDailyCheckinPaidTotal(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	_, err := client.ExecContext(ctx, `ALTER TABLE users ADD COLUMN bonus_balance DECIMAL(20,8) NOT NULL DEFAULT 0`)
	require.NoError(t, err)
	user, err := client.User.Create().SetEmail("checkin-recharge@example.com").SetPasswordHash("hash").Save(ctx)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `UPDATE users SET bonus_balance = 50 WHERE id = ?`, user.ID)
	require.NoError(t, err)
	other, err := client.User.Create().SetEmail("other-recharge@example.com").SetPasswordHash("hash").Save(ctx)
	require.NoError(t, err)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, dailyCheckinLocation)
	cutoff := now.Add(-30 * 24 * time.Hour)

	total, err := dailyCheckinPaidTotal(ctx, client.PaymentOrder, user.ID, now)
	require.NoError(t, err)
	require.True(t, total.IsZero(), "bonus balance is excluded from the paid total")

	addOrder := func(t *testing.T, userID int64, kind, status string, amount float64, paidAt time.Time, currency string) {
		t.Helper()
		create := client.PaymentOrder.Create().SetUserID(userID).SetUserEmail(user.Email).SetUserName("checkin").
			SetAmount(15).SetPayAmount(amount).SetRechargeCode("checkin").SetPaymentType(payment.TypeAlipay).
			SetPaymentTradeNo("checkin").SetOrderType(kind).SetStatus(status).
			SetClientIP("127.0.0.1").SetSrcHost("example.com").SetExpiresAt(now.Add(time.Hour))
		if !paidAt.IsZero() {
			create.SetPaidAt(paidAt)
		}
		if currency != "" {
			create.SetProviderSnapshot(map[string]any{"currency": currency})
		}
		order, err := create.Save(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, client.PaymentOrder.DeleteOneID(order.ID).Exec(ctx)) })
	}

	t.Run("recharge and subscription payments combine", func(t *testing.T) {
		addOrder(t, user.ID, payment.OrderTypeBalance, OrderStatusCompleted, 4.00, now, "CNY")
		addOrder(t, user.ID, payment.OrderTypeSubscription, OrderStatusCompleted, 5.90, now, "CNY")
		addOrder(t, user.ID, payment.OrderTypeSubscription, OrderStatusPending, 5.90, time.Time{}, "CNY")
		total, err := dailyCheckinPaidTotal(ctx, client.PaymentOrder, user.ID, now)
		require.NoError(t, err)
		require.Equal(t, 9.90, total.InexactFloat64())
	})

	for _, tc := range []struct {
		name     string
		userID   int64
		kind     string
		status   string
		amounts  []float64
		paidAt   time.Time
		currency string
		total    float64
	}{
		{"recent recharge", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{12}, now.Add(-time.Hour), "", 12},
		{"exact threshold at cutoff", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{9.90}, cutoff, "CNY", 9.90},
		{"amounts beyond threshold still count", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{6.80, 3.10, 4.40}, now, "CNY", 14.30},
		{"cumulative amount one cent below threshold", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{3.30, 3.30, 3.29}, now, "CNY", 9.89},
		{"different currency", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{10}, now, "USD", 0},
		{"before cutoff", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{12}, cutoff.Add(-time.Second), "", 0},
		{"future payment", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{12}, now.Add(time.Second), "", 0},
		{"no payment time", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{12}, time.Time{}, "", 0},
		{"subscription", user.ID, payment.OrderTypeSubscription, OrderStatusCompleted, []float64{12}, now, "", 12},
		{"unfulfilled order", user.ID, payment.OrderTypeBalance, OrderStatusPaid, []float64{12}, now, "", 0},
		{"refunded order", user.ID, payment.OrderTypeBalance, OrderStatusRefunded, []float64{12}, now, "", 0},
		{"zero payment", user.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{0}, now, "", 0},
		{"another user's order", other.ID, payment.OrderTypeBalance, OrderStatusCompleted, []float64{12}, now, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, amount := range tc.amounts {
				addOrder(t, tc.userID, tc.kind, tc.status, amount, tc.paidAt, tc.currency)
			}

			total, err := dailyCheckinPaidTotal(ctx, client.PaymentOrder, user.ID, now)
			require.NoError(t, err)
			require.Equal(t, tc.total, total.InexactFloat64())
		})
	}
}

func expectDailyCheckinRecharge(mock sqlmock.Sqlmock, amounts ...float64) {
	rows := sqlmock.NewRows([]string{"id", "pay_amount", "provider_snapshot"})
	for index, amount := range amounts {
		rows.AddRow(index+1, amount, nil)
	}
	mock.ExpectQuery(`SELECT .* FROM "payment_orders" WHERE .*`).
		WithArgs(int64(42), OrderStatusCompleted, float64(0), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)
}

func TestClaimDailyCheckinRequiresRecentRecharge(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	service := newPaymentServiceForSQLMock(db)
	t.Cleanup(func() { _ = service.entClient.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT checkin_date, streak_day.*FOR UPDATE`).WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}).AddRow(dailyCheckinDate(time.Now()).AddDate(0, 0, -1), 5))
	expectDailyCheckinRecharge(mock, 3.30, 3.30, 3.29)
	mock.ExpectRollback()

	status, err := service.ClaimDailyCheckin(context.Background(), 42)
	require.Nil(t, status)
	require.True(t, infraerrors.IsForbidden(err))
	require.Equal(t, "CHECKIN_RECHARGE_REQUIRED", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
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
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date", "streak_day"}))
	expectDailyCheckinRecharge(mock, 5.00, 4.90)
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
	expectDailyCheckinRecharge(mock, 6.80)

	status, err := service.ClaimDailyCheckin(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, status.CheckedInToday)
	require.False(t, status.HasRecentRecharge)
	require.Equal(t, DailyCheckinRechargeProgress{Total: 6.80, Target: 9.90, Remaining: 3.10}, status.RechargeProgress)
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
	expectDailyCheckinRecharge(mock, 5.00, 4.90)
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
	expectDailyCheckinRecharge(mock, 5.00, 4.90)
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
	expectDailyCheckinRecharge(mock, 5.00, 4.90)

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
	expectDailyCheckinRecharge(mock, 5.00, 4.90)
	status, err := service.GetDailyCheckinStatus(context.Background(), 42, "2025-01")
	require.NoError(t, err)
	require.True(t, status.CheckedInToday)
	require.True(t, status.HasRecentRecharge)
	require.Equal(t, DailyCheckinRechargeProgress{Total: 9.90, Target: 9.90, Remaining: 0}, status.RechargeProgress)
	require.Equal(t, 30, status.CurrentStreak)
	require.Equal(t, 30, status.CycleDay)
	require.Equal(t, 100.0, status.TodayReward, "historical credits retain the original reward amount")
	require.Equal(t, today.Format("2006-01-02"), status.Today)
	require.Empty(t, status.Records)
	require.Len(t, status.Milestones, 4)
	require.Equal(t, []float64{0.2, 0.3}, status.OrdinaryRewards)
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
	expectDailyCheckinRecharge(mock, 5.00, 4.90)
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
	expectDailyCheckinRecharge(mock, 5.00, 4.90)
	status, err := service.ClaimDailyCheckin(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, status.CheckedInToday)
	require.Equal(t, 10.0, status.TodayReward)
	require.Equal(t, 30, status.CurrentStreak)
	require.NoError(t, mock.ExpectationsWereMet())
}
