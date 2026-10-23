package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestBonusBalanceConversionValidation(t *testing.T) {
	s := &PaymentService{}
	_, err := s.ConvertBonusBalances(context.Background(), 1, "wrong")
	require.ErrorContains(t, err, "BONUS_CONVERSION_CONFIRMATION_REQUIRED")
	_, err = s.ConvertBonusBalances(context.Background(), 0, BonusBalanceConversionConfirmation)
	require.ErrorContains(t, err, "BONUS_CONVERSION_ADMIN_REQUIRED")
}

// Optional native PostgreSQL coverage, using a fresh schema per test. The DSN
// must use lib/pq's keyword format and must point at a disposable local test DB.
func conversionTestService(t *testing.T) (*PaymentService, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("SUB2API_CONVERSION_TEST_DSN")
	if dsn == "" {
		t.Skip("set SUB2API_CONVERSION_TEST_DSN to run PostgreSQL conversion tests")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("bonus_conversion_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = base.Close() })
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
CREATE TABLE users(id BIGINT PRIMARY KEY, email TEXT, balance NUMERIC(20,8), bonus_balance NUMERIC(20,8), deleted_at TIMESTAMPTZ, updated_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT);
INSERT INTO settings VALUES('BONUS_BALANCE_CONSUMPTION_RATE','10');
CREATE TABLE bonus_balance_events(id BIGSERIAL PRIMARY KEY,event_key TEXT UNIQUE,user_id BIGINT,email TEXT,source TEXT,amount NUMERIC(20,8),balance_after NUMERIC(20,8),historical BOOL DEFAULT FALSE,created_at TIMESTAMPTZ DEFAULT NOW(),updated_at TIMESTAMPTZ DEFAULT NOW());
INSERT INTO users VALUES(1,'a',123,128.6,NULL,NOW()),(2,'b',456,0.00000005,NULL,NOW()),(3,'deleted',789,35,NOW(),NOW()),(4,'zero',11,0,NULL,NOW());
INSERT INTO bonus_balance_events(user_id,email,source,amount,balance_after) SELECT id,email,'admin',bonus_balance,bonus_balance FROM users WHERE bonus_balance > 0;
`)
	require.NoError(t, err)
	source, err := migrations.FS.ReadFile("251_bonus_balance_conversion.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(source))
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TRIGGER conversion_audit AFTER UPDATE OF bonus_balance ON users FOR EACH ROW EXECUTE FUNCTION audit_bonus_balance_change()`)
	require.NoError(t, err)
	source, err = migrations.FS.ReadFile("252_bonus_reporting_epoch.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(source))
	require.NoError(t, err)
	return newPaymentServiceForSQLMock(db), db
}

func TestBonusBalanceConversionPostgres(t *testing.T) {
	t.Run("exact amounts, audit, unchanged principal and multiplier, permanent guard", func(t *testing.T) {
		s, db := conversionTestService(t)
		ctx := context.Background()
		preview, err := s.GetBonusBalanceConversion(ctx)
		require.NoError(t, err)
		require.False(t, preview.Completed)
		require.Equal(t, int64(3), preview.Users)
		require.Equal(t, "163.60000005", preview.BeforeTotal)
		require.Equal(t, "16.36000001", preview.AfterTotal)
		result, err := s.ConvertBonusBalances(ctx, 99, BonusBalanceConversionConfirmation)
		require.NoError(t, err)
		require.True(t, result.Completed)
		require.NotNil(t, result.CompletedAt)
		require.Equal(t, preview.AfterTotal, result.AfterTotal)
		var principal, rate, balance string
		require.NoError(t, db.QueryRow(`SELECT balance::text,bonus_balance::text FROM users WHERE id=1`).Scan(&principal, &balance))
		require.Equal(t, "123.00000000", principal)
		require.Equal(t, "12.86000000", balance)
		require.NoError(t, db.QueryRow(`SELECT value FROM settings WHERE key='BONUS_BALANCE_CONSUMPTION_RATE'`).Scan(&rate))
		require.Equal(t, "10", rate)
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM bonus_balance_events WHERE source='conversion'`).Scan(&count))
		require.Equal(t, 3, count)
		report, err := s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{Scope: "history", Source: "conversion"})
		require.NoError(t, err)
		require.Zero(t, report.Summary.Consumed)
		require.Zero(t, report.Summary.ReconciliationDifference)
		require.Equal(t, int64(3), report.Total)
		_, err = s.ConvertBonusBalances(ctx, 99, BonusBalanceConversionConfirmation)
		require.ErrorContains(t, err, "BONUS_CONVERSION_ALREADY_COMPLETED")
		receipt, err := s.GetBonusBalanceConversion(ctx)
		require.NoError(t, err)
		require.Equal(t, result, receipt)
	})
	t.Run("failure rolls back balances, snapshots, receipt and audit then permits retry", func(t *testing.T) {
		s, db := conversionTestService(t)
		_, err := db.Exec(`CREATE FUNCTION fail_conversion() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN IF NEW.id=3 THEN RAISE EXCEPTION 'test failure'; END IF; RETURN NEW; END; $$;
CREATE TRIGGER fail_conversion BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION fail_conversion()`)
		require.NoError(t, err)
		_, err = s.ConvertBonusBalances(context.Background(), 99, BonusBalanceConversionConfirmation)
		require.Error(t, err)
		var count int
		require.NoError(t, db.QueryRow(`SELECT (SELECT COUNT(*) FROM bonus_balance_conversions)+(SELECT COUNT(*) FROM bonus_balance_conversion_users)+(SELECT COUNT(*) FROM bonus_balance_events WHERE source='conversion')`).Scan(&count))
		require.Zero(t, count)
		preview, err := s.GetBonusBalanceConversion(context.Background())
		require.NoError(t, err)
		require.Equal(t, "163.60000005", preview.BeforeTotal)
		_, err = db.Exec(`DROP TRIGGER fail_conversion ON users`)
		require.NoError(t, err)
		_, err = s.ConvertBonusBalances(context.Background(), 99, BonusBalanceConversionConfirmation)
		require.NoError(t, err)
	})
	t.Run("concurrent requests convert only once", func(t *testing.T) {
		s, db := conversionTestService(t)
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		start := make(chan struct{})
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := s.ConvertBonusBalances(context.Background(), 99, BonusBalanceConversionConfirmation)
				errs <- err
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		successes := 0
		for err := range errs {
			if err == nil {
				successes++
			} else {
				require.ErrorContains(t, err, "BONUS_CONVERSION_ALREADY_COMPLETED")
			}
		}
		require.Equal(t, 1, successes)
		var balance string
		require.NoError(t, db.QueryRow(`SELECT bonus_balance::text FROM users WHERE id=1`).Scan(&balance))
		require.Equal(t, "12.86000000", balance)
	})
	t.Run("empty conversion does not consume the one-time guard", func(t *testing.T) {
		s, db := conversionTestService(t)
		_, err := db.Exec(`UPDATE users SET bonus_balance=0`)
		require.NoError(t, err)
		_, err = s.ConvertBonusBalances(context.Background(), 99, BonusBalanceConversionConfirmation)
		require.ErrorContains(t, err, "BONUS_CONVERSION_EMPTY")
		preview, err := s.GetBonusBalanceConversion(context.Background())
		require.NoError(t, err)
		require.False(t, preview.Completed)
	})
}
