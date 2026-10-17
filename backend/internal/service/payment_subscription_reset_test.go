package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type paidResetGroupRepo struct{ GroupRepository }

func (r paidResetGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	return &Group{ID: id, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}, nil
}

func TestPaidSubscriptionResetQuoteUsesCurrentPriceAndOriginalDays(t *testing.T) {
	for _, scenario := range []string{"quote", "other user", "wrong plan", "changed price", "unknown term"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })
			planID := int64(7)
			repo := &paidResetSubscriptionRepo{sub: UserSubscription{ID: 5, UserID: 1, GroupID: 2, Status: SubscriptionStatusActive,
				BaseValidityDays: 30, SourcePlanID: &planID, ExpiresAt: time.Now().AddDate(0, 0, 10)}}
			subs := NewSubscriptionService(nil, repo, nil, nil, nil)
			t.Cleanup(subs.Stop)
			svc := &PaymentService{subscriptionSvc: subs, configService: &PaymentConfigService{entClient: client}, groupRepo: paidResetGroupRepo{}}
			switch scenario {
			case "other user":
				_, err := svc.GetSubscriptionResetQuote(context.Background(), 99, 5)
				require.Equal(t, "SUBSCRIPTION_NOT_FOUND", infraerrors.Reason(err))
			case "unknown term":
				repo.sub.BaseValidityDays = 0
				_, err := svc.GetSubscriptionResetQuote(context.Background(), 1, 5)
				require.Equal(t, "SUBSCRIPTION_TERM_UNKNOWN", infraerrors.Reason(err))
			default:
				mock.ExpectQuery(`SELECT .*subscription_plans`).WithArgs(planID).WillReturnRows(
					sqlmock.NewRows([]string{"id", "group_id", "name", "price", "validity_days", "validity_unit", "for_sale"}).
						AddRow(7, 2, "Current plan", 128, 60, "day", true))
				if scenario == "quote" {
					quote, err := svc.GetSubscriptionResetQuote(context.Background(), 1, 5)
					require.NoError(t, err)
					require.Equal(t, 30, quote.ValidityDays, "changing the sale plan must not replace the original term")
					require.Equal(t, float64(128), quote.Price)
				} else {
					req := CreateOrderRequest{UserID: 1, OrderType: payment.OrderTypeSubscription, SubscriptionResetID: 5, PlanID: 7, Amount: 128}
					reason := "RESET_PLAN_MISMATCH"
					if scenario == "wrong plan" {
						req.PlanID = 9
					} else {
						req.Amount = 99
						reason = "RESET_PRICE_CHANGED"
					}
					_, err := svc.CreateOrder(context.Background(), req)
					require.Equal(t, reason, infraerrors.Reason(err))
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPaidSubscriptionResetReplacesRemainingDaysAndAllQuota(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	sub := &UserSubscription{ID: 5, UserID: 1, GroupID: 2, Status: SubscriptionStatusActive,
		BaseValidityDays: 30, StartsAt: now.AddDate(0, 0, -20), ExpiresAt: now.AddDate(0, 0, 10),
		DailyUsageUSD: 5, WeeklyUsageUSD: 35, MonthlyUsageUSD: 160}
	require.NoError(t, resetSubscriptionTerm(sub, 1, 2, 30, now))
	require.Equal(t, now, sub.StartsAt)
	require.Equal(t, now.AddDate(0, 0, 30), sub.ExpiresAt)
	require.NotEqual(t, now.AddDate(0, 0, 40), sub.ExpiresAt)
	require.Zero(t, sub.DailyUsageUSD)
	require.Zero(t, sub.WeeklyUsageUSD)
	require.Zero(t, sub.MonthlyUsageUSD)
	require.Equal(t, now, *sub.WeeklyWindowStart)
	require.Equal(t, now, *sub.MonthlyWindowStart)
	require.Equal(t, 30, sub.BaseValidityDays)
}

func TestPaidSubscriptionResetRejectsOtherUsersGroupsAndRevokedSubscriptions(t *testing.T) {
	for _, test := range []struct {
		name        string
		user, group int64
		status      string
		deleted     bool
	}{
		{"other user", 9, 2, SubscriptionStatusActive, false},
		{"other group", 1, 9, SubscriptionStatusActive, false},
		{"suspended", 1, 2, SubscriptionStatusSuspended, false},
		{"deleted", 1, 2, SubscriptionStatusActive, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now()
			sub := &UserSubscription{UserID: 1, GroupID: 2, Status: test.status, MonthlyUsageUSD: 160, ExpiresAt: now.AddDate(0, 0, 10)}
			if test.deleted {
				sub.DeletedAt = &now
			}
			before := *sub
			require.Error(t, resetSubscriptionTerm(sub, test.user, test.group, 30, now))
			require.Equal(t, before, *sub)
		})
	}
}

type paidResetSubscriptionRepo struct {
	UserSubscriptionRepository
	sub     UserSubscription
	updates int
}

func (r *paidResetSubscriptionRepo) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	copy := r.sub
	return &copy, nil
}

func (r *paidResetSubscriptionRepo) GetByIDForUpdate(ctx context.Context, id int64) (*UserSubscription, error) {
	if dbent.TxFromContext(ctx) == nil {
		return nil, errors.New("reset must lock inside its transaction")
	}
	return r.GetByID(ctx, id)
}

func (r *paidResetSubscriptionRepo) Update(ctx context.Context, sub *UserSubscription) error {
	tx := dbent.TxFromContext(ctx)
	if tx == nil {
		return errors.New("reset update must share the audit transaction")
	}
	copy := *sub
	tx.OnCommit(func(next dbent.Committer) dbent.Committer {
		return dbent.CommitFunc(func(ctx context.Context, tx *dbent.Tx) error {
			if err := next.Commit(ctx, tx); err != nil {
				return err
			}
			r.sub, r.updates = copy, r.updates+1
			return nil
		})
	})
	return nil
}

func TestPaidSubscriptionResetCallbackIsIdempotentAndAtomic(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		auditFails, paid bool
	}{
		{"repeated callback", false, true}, {"audit rollback", true, true}, {"unpaid order", false, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })
			repo := &paidResetSubscriptionRepo{sub: UserSubscription{ID: 5, UserID: 1, GroupID: 2, Status: SubscriptionStatusActive, BaseValidityDays: 30, MonthlyUsageUSD: 160, ExpiresAt: time.Now().AddDate(0, 0, 10)}}
			subs := NewSubscriptionService(nil, repo, nil, client, nil)
			t.Cleanup(subs.Stop)
			svc := &PaymentService{entClient: client, subscriptionSvc: subs}
			order := &dbent.PaymentOrder{ID: 7, UserID: 1, OrderType: payment.OrderTypeSubscription, ProviderSnapshot: map[string]any{"subscription_action": "reset", "subscription_reset_id": float64(5)}}
			expectLockedOrder := func(applied int) {
				var paidAt any
				if scenario.paid {
					paidAt = time.Now()
				}
				mock.ExpectBegin()
				mock.ExpectQuery(`SELECT .*payment_orders.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "status", "paid_at"}).AddRow(7, OrderStatusRecharging, paidAt))
				mock.ExpectQuery(`SELECT COUNT.*payment_audit_logs`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(applied))
			}
			expectLockedOrder(0)
			if !scenario.paid {
				mock.ExpectRollback()
				err := svc.ensurePaymentSubscriptionReset(context.Background(), order, 2, 30)
				require.Equal(t, "INVALID_STATUS", infraerrors.Reason(err))
				require.Zero(t, repo.updates)
				require.Equal(t, float64(160), repo.sub.MonthlyUsageUSD)
			} else if scenario.auditFails {
				mock.ExpectQuery(`INSERT INTO .*payment_audit_logs`).WillReturnError(errors.New("audit storage unavailable"))
				mock.ExpectRollback()
				require.Error(t, svc.ensurePaymentSubscriptionReset(context.Background(), order, 2, 30))
				require.Equal(t, float64(160), repo.sub.MonthlyUsageUSD)
				require.Zero(t, repo.updates)
			} else {
				mock.ExpectQuery(`INSERT INTO .*payment_audit_logs`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(8))
				mock.ExpectCommit()
				require.NoError(t, svc.ensurePaymentSubscriptionReset(context.Background(), order, 2, 30))
				require.Zero(t, repo.sub.MonthlyUsageUSD)
				require.Equal(t, 1, repo.updates)
				expiry := repo.sub.ExpiresAt
				repo.sub.MonthlyUsageUSD = 3
				expectLockedOrder(1)
				mock.ExpectCommit()
				require.NoError(t, svc.ensurePaymentSubscriptionReset(context.Background(), order, 2, 30))
				require.Equal(t, float64(3), repo.sub.MonthlyUsageUSD, "a repeated notification must not clear new usage")
				require.Equal(t, expiry, repo.sub.ExpiresAt)
				require.Equal(t, 1, repo.updates)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
