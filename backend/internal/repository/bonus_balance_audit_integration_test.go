//go:build integration

package repository

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBonusBalanceAudit_ReportReconcilesAndPreservesDeletedUsers(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	payments := service.NewPaymentService(client, nil, nil, nil, nil, nil, nil, nil, nil)
	before, err := payments.GetBonusBalanceReport(ctx, service.BonusBalanceReportFilter{})
	require.NoError(t, err)
	u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("bonus-report-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
	defer func() { _, _ = client.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, u.ID) }()
	for _, movement := range []struct {
		source string
		delta  float64
	}{{"signup", 50}, {"checkin", 10}, {"consumption", -15}, {"consumption", -5}, {"admin", -3}} {
		_, err := client.ExecContext(ctx, "/* bonus_source:"+movement.source+" */ UPDATE users SET bonus_balance = bonus_balance + $1 WHERE id = $2", movement.delta, u.ID)
		require.NoError(t, err)
	}
	after, err := payments.GetBonusBalanceReport(ctx, service.BonusBalanceReportFilter{Search: strconv.FormatInt(u.ID, 10)})
	require.NoError(t, err)
	require.InDelta(t, 60, after.Summary.Issued-before.Summary.Issued, 1e-8)
	require.InDelta(t, 20, after.Summary.Consumed-before.Summary.Consumed, 1e-8)
	require.InDelta(t, 37, after.Summary.Remaining-before.Summary.Remaining, 1e-8)
	require.InDelta(t, 3, after.Summary.Adjustments-before.Summary.Adjustments, 1e-8)
	require.InDelta(t, 0, after.Summary.ReconciliationDifference, 1e-8)
	require.Equal(t, int64(4), after.Total, "daily consumption must aggregate rather than grow per request")
	filtered, err := payments.GetBonusBalanceReport(ctx, service.BonusBalanceReportFilter{Search: strconv.FormatInt(u.ID, 10), Source: "consumption", Kind: "consumption"})
	require.NoError(t, err)
	require.Len(t, filtered.Records, 1)
	require.Equal(t, -20.0, filtered.Records[0].Amount)
	require.Equal(t, after.Summary, filtered.Summary, "record filters must not change global summary")
	_, err = client.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
	require.NoError(t, err)
	removed, err := payments.GetBonusBalanceReport(ctx, service.BonusBalanceReportFilter{Search: strconv.FormatInt(u.ID, 10)})
	require.NoError(t, err)
	require.Equal(t, after.Summary.Issued, removed.Summary.Issued)
	require.Equal(t, after.Summary.Consumed, removed.Summary.Consumed)
	require.InDelta(t, 37, removed.Summary.Adjustments-after.Summary.Adjustments, 1e-8)
	require.InDelta(t, 0, removed.Summary.ReconciliationDifference, 1e-8)
}

func TestBonusBalanceAudit_TransactionRollback(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("bonus-rollback-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
	_, err := client.ExecContext(ctx, `SAVEPOINT bonus_before`)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `/* bonus_source:checkin */ UPDATE users SET bonus_balance=10 WHERE id=$1`, u.ID)
	require.NoError(t, err)
	require.Equal(t, 1, querySingleInt(t, ctx, client, `SELECT COUNT(*) FROM bonus_balance_events WHERE user_id=$1`, u.ID))
	_, err = client.ExecContext(ctx, `ROLLBACK TO SAVEPOINT bonus_before`)
	require.NoError(t, err)
	require.Zero(t, querySingleInt(t, ctx, client, `SELECT COUNT(*) FROM bonus_balance_events WHERE user_id=$1`, u.ID))
}

func TestBonusBalanceAudit_RecordsActualWalletUnitsAndActivitySources(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("bonus-real-writes-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
	defer func() {
		_, _ = client.ExecContext(ctx, `DELETE FROM signup_promotion_grants WHERE source_user_id=$1`, u.ID)
		_, _ = client.ExecContext(ctx, `DELETE FROM signup_promotions WHERE user_id=$1`, u.ID)
		_, _ = client.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
	}()
	users := NewUserRepository(client, integrationDB).(service.BonusBalanceStore)
	_, err := users.AdjustBonusBalance(ctx, u.ID, 10)
	require.NoError(t, err)
	payments := service.NewPaymentService(client, nil, nil, nil, nil, nil, nil, nil, nil)
	_, err = payments.ClaimDailyCheckin(ctx, u.ID)
	require.NoError(t, err)
	repo := &affiliateRepository{client: client}
	_, err = repo.ApplySignupPromotion(ctx, u.ID, "", service.SignupPromotionConfig{SignupBonusEnabled: true, SignupBonusAmount: 50})
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, _, bonus, principal, err := deductUsageBillingBalanceDetailed(ctx, tx, u.ID, 2, 3)
	require.NoError(t, err)
	require.Equal(t, 6.0, bonus)
	require.Zero(t, principal)
	require.NoError(t, tx.Commit())
	for source, amount := range map[string]float64{"admin": 10, "checkin": 10, "signup": 50, "consumption": -6} {
		require.Equal(t, amount, querySingleFloat(t, ctx, client, `SELECT SUM(amount)::double precision FROM bonus_balance_events WHERE user_id=$1 AND source=$2`, u.ID, source))
	}
	require.Equal(t, 64.0, querySingleFloat(t, ctx, client, `SELECT SUM(amount)::double precision FROM bonus_balance_events WHERE user_id=$1`, u.ID))
}

func TestBonusBalanceAudit_HistoricalMigration(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	schema := fmt.Sprintf("bonus_history_%d", time.Now().UnixNano())
	_, err := client.ExecContext(ctx, `CREATE SCHEMA `+schema+`; SET LOCAL search_path TO `+schema)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `
CREATE TABLE users(id BIGINT PRIMARY KEY,email TEXT,bonus_balance NUMERIC(20,8),deleted_at TIMESTAMPTZ);
CREATE TABLE schema_migrations(filename TEXT,applied_at TIMESTAMPTZ);
INSERT INTO schema_migrations VALUES('231_add_reward_campaigns.sql','2026-08-26T00:00:00Z');
CREATE TABLE signup_promotion_grants(user_id BIGINT,kind TEXT,amount NUMERIC,created_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE user_daily_checkins(user_id BIGINT,reward_amount NUMERIC,created_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE recharge_bonus_grants(user_id BIGINT,bonus_amount NUMERIC,created_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE consumption_reward_claims(user_id BIGINT,bonus_amount NUMERIC,created_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE recharge_lottery_draws(user_id BIGINT,is_winner BOOL,prize_amount NUMERIC,created_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE redeem_codes(used_by BIGINT,type TEXT,status TEXT,value NUMERIC,used_at TIMESTAMPTZ,created_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE usage_logs(user_id BIGINT,bonus_cost NUMERIC);
INSERT INTO users VALUES(1,'a@example.com',25,NULL),(2,'b@example.com',8,NULL);
INSERT INTO signup_promotion_grants(user_id,kind,amount) VALUES(1,'signup',50);
INSERT INTO user_daily_checkins(user_id,reward_amount) VALUES(1,10);
INSERT INTO redeem_codes(used_by,type,status,value) VALUES(1,'admin_bonus','used',-5);
INSERT INTO usage_logs VALUES(1,20),(1,NULL);
INSERT INTO recharge_lottery_draws(user_id,is_winner,prize_amount,created_at) VALUES(1,TRUE,99,'2026-08-25T00:00:00Z');
`)
	require.NoError(t, err)
	source, err := migrations.FS.ReadFile("248_bonus_balance_audit.sql")
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, string(source))
	require.NoError(t, err)
	// User 1: 60 issued -20 measured consumption -5 admin -10 unknown =25.
	// User 2: retained 8 of unknown origin; do not pretend it was a campaign.
	require.Equal(t, 10.0, querySingleFloat(t, ctx, client, `SELECT -SUM(amount)::double precision FROM bonus_balance_events WHERE source='historical_outflow'`))
	require.Equal(t, 8.0, querySingleFloat(t, ctx, client, `SELECT SUM(amount)::double precision FROM bonus_balance_events WHERE source='historical_credit'`))
	require.Equal(t, 33.0, querySingleFloat(t, ctx, client, `SELECT SUM(amount)::double precision FROM bonus_balance_events`))
	_, err = client.ExecContext(ctx, `/* bonus_source:checkin */ UPDATE users SET bonus_balance=bonus_balance+10 WHERE id=2`)
	require.NoError(t, err)
	require.Equal(t, 20.0, querySingleFloat(t, ctx, client, `SELECT SUM(amount)::double precision FROM bonus_balance_events WHERE source='checkin'`))
}
