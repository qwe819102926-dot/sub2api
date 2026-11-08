package service

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func installBonusExpiry(t *testing.T, db *sql.DB) {
	t.Helper()
	source, err := migrations.FS.ReadFile("254_bonus_balance_expiry.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(source))
	require.NoError(t, err)
}

func bonusSQLFloat(t *testing.T, db *sql.DB, query string) float64 {
	t.Helper()
	var value float64
	require.NoError(t, db.QueryRow(query).Scan(&value))
	return value
}

func TestBonusExpiryPostgresHistoricalBalances(t *testing.T) {
	for _, convert := range []bool{false, true} {
		t.Run(map[bool]string{false: "original units", true: "converted units"}[convert], func(t *testing.T) {
			s, db := conversionTestService(t)
			_, err := db.Exec(`UPDATE bonus_balance_events SET amount=80,created_at=NOW()-INTERVAL '40 days' WHERE user_id=1;
INSERT INTO bonus_balance_events(user_id,email,source,amount,created_at) VALUES
(1,'a','checkin',50,NOW()-INTERVAL '10 days'),(1,'a','consumption',-1.4,NOW());`)
			require.NoError(t, err)
			if convert {
				_, err = s.ConvertBonusBalances(context.Background(), 99, BonusBalanceConversionConfirmation)
				require.NoError(t, err)
			}
			installBonusExpiry(t, db)
			want := 50.0
			if convert {
				want = 5
			}
			require.InDelta(t, want, bonusSQLFloat(t, db, `SELECT bonus_balance FROM users WHERE id=1`), 1e-8)
			require.InDelta(t, want, bonusSQLFloat(t, db, `SELECT spendable_bonus_balance(1)`), 1e-8)
			require.Equal(t, 123.0, bonusSQLFloat(t, db, `SELECT balance FROM users WHERE id=1`))
			require.Equal(t, 1.0, bonusSQLFloat(t, db, `SELECT COUNT(*) FROM bonus_balance_events WHERE user_id=1 AND source='expiration'`))
			require.Equal(t, -1.4, bonusSQLFloat(t, db, `SELECT SUM(amount) FROM bonus_balance_events WHERE user_id=1 AND source='consumption'`))
			require.InDelta(t, 0, bonusSQLFloat(t, db, `SELECT SUM(amount)-(SELECT bonus_balance FROM users WHERE id=1) FROM bonus_balance_events WHERE user_id=1`), 1e-8)
			_, err = db.Exec(`/* bonus_source:expiration */ SELECT expire_user_bonus_balance(1)`)
			require.NoError(t, err)
			require.Equal(t, 1.0, bonusSQLFloat(t, db, `SELECT COUNT(*) FROM bonus_balance_events WHERE user_id=1 AND source='expiration'`))
		})
	}
}

func TestBonusExpiryPostgresUnknownDatesGetGraceMonth(t *testing.T) {
	_, db := conversionTestService(t)
	_, err := db.Exec(`UPDATE bonus_balance_events SET source='historical_credit',created_at=NOW()-INTERVAL '1 year' WHERE user_id=1;
DELETE FROM bonus_balance_events WHERE user_id=2;`)
	require.NoError(t, err)
	installBonusExpiry(t, db)
	require.Equal(t, 128.6, bonusSQLFloat(t, db, `SELECT spendable_bonus_balance(1)`))
	require.Equal(t, 0.00000005, bonusSQLFloat(t, db, `SELECT spendable_bonus_balance(2)`))
	require.Equal(t, 2.0, bonusSQLFloat(t, db, `SELECT COUNT(*) FROM bonus_balance_lots WHERE user_id IN (1,2) AND date_estimated AND expires_at=bonus_balance_expires_at(granted_at)`))
}

func TestBonusExpiryPostgresCalendarMonth(t *testing.T) {
	_, db := conversionTestService(t)
	installBonusExpiry(t, db)
	for _, tc := range []struct{ grant, expiry string }{
		{"2027-01-31T10:20:30+08:00", "2027-02-28T10:20:30+08:00"},
		{"2028-01-31T10:20:30+08:00", "2028-02-29T10:20:30+08:00"},
		{"2026-11-08T10:20:30+08:00", "2026-12-08T10:20:30+08:00"},
	} {
		var expiry time.Time
		require.NoError(t, db.QueryRow(`SELECT bonus_balance_expires_at($1::timestamptz)`, tc.grant).Scan(&expiry))
		want, err := time.Parse(time.RFC3339, tc.expiry)
		require.NoError(t, err)
		require.True(t, want.Equal(expiry))
	}
}

func TestBonusExpiryPostgresFIFOAndNewGrantsDoNotRenewOldOnes(t *testing.T) {
	_, db := conversionTestService(t)
	installBonusExpiry(t, db)
	_, err := db.Exec(`UPDATE bonus_balance_lots SET granted_at=NOW()-INTERVAL '10 days',expires_at=NOW()+INTERVAL '20 days' WHERE user_id=1;
/* bonus_source:checkin */ UPDATE users SET bonus_balance=bonus_balance+10 WHERE id=1;`)
	require.NoError(t, err)
	_, err = db.Exec(`/* bonus_source:consumption */ UPDATE users SET bonus_balance=bonus_balance-120 WHERE id=1`)
	require.NoError(t, err)
	require.InDelta(t, 8.6, bonusSQLFloat(t, db, `SELECT SUM(remaining) FROM bonus_balance_lots WHERE user_id=1 AND expires_at<NOW()+INTERVAL '25 days'`), 1e-8)
	require.Equal(t, 10.0, bonusSQLFloat(t, db, `SELECT SUM(remaining) FROM bonus_balance_lots WHERE user_id=1 AND expires_at>NOW()+INTERVAL '25 days'`))
	_, err = db.Exec(`UPDATE bonus_balance_lots SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=1 AND expires_at<NOW()+INTERVAL '25 days'`)
	require.NoError(t, err)
	// Read-side validity is immediate, before the periodic cleanup mutates users.
	require.Equal(t, 10.0, bonusSQLFloat(t, db, `SELECT spendable_bonus_balance(1)`))
	require.InDelta(t, 18.6, bonusSQLFloat(t, db, `SELECT bonus_balance FROM users WHERE id=1`), 1e-8)
	_, err = db.Exec(`/* bonus_source:expiration */ SELECT expire_user_bonus_balance(1)`)
	require.NoError(t, err)
	require.Equal(t, 10.0, bonusSQLFloat(t, db, `SELECT bonus_balance FROM users WHERE id=1`))
}

func TestBonusExpiryPostgresConcurrentSweepsAndRollback(t *testing.T) {
	s, db := conversionTestService(t)
	installBonusExpiry(t, db)
	_, err := db.Exec(`UPDATE bonus_balance_lots SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=1`)
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`/* bonus_source:expiration */ SELECT expire_user_bonus_balance(1)`)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	require.Equal(t, 128.6, bonusSQLFloat(t, db, `SELECT bonus_balance FROM users WHERE id=1`))
	require.Equal(t, 128.6, bonusSQLFloat(t, db, `SELECT SUM(remaining) FROM bonus_balance_lots WHERE user_id=1`))
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := db.Exec(`/* bonus_source:expiration */ SELECT expire_user_bonus_balance(1)`)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Zero(t, bonusSQLFloat(t, db, `SELECT bonus_balance FROM users WHERE id=1`))
	require.Equal(t, -128.6, bonusSQLFloat(t, db, `SELECT SUM(amount) FROM bonus_balance_events WHERE user_id=1 AND source='expiration'`))
	require.Equal(t, 1.0, bonusSQLFloat(t, db, `SELECT COUNT(*) FROM bonus_balance_events WHERE user_id=1 AND source='expiration'`))
	n, err := s.ExpireBonusBalances(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	_, err = db.Exec(`UPDATE bonus_balance_lots SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=2`)
	require.NoError(t, err)
	n, err = s.ExpireBonusBalances(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Zero(t, bonusSQLFloat(t, db, `SELECT bonus_balance FROM users WHERE id=2`))
}

func TestBonusExpiryPostgresConversionPreservesDatesAndPrecision(t *testing.T) {
	s, db := conversionTestService(t)
	installBonusExpiry(t, db)
	_, err := db.Exec(`/* bonus_source:checkin */ UPDATE users SET bonus_balance=bonus_balance+0.00000004 WHERE id=2;
/* bonus_source:checkin */ UPDATE users SET bonus_balance=bonus_balance+0.00000004 WHERE id=2;`)
	require.NoError(t, err)
	_, err = s.ConvertBonusBalances(context.Background(), 99, BonusBalanceConversionConfirmation)
	require.NoError(t, err)
	require.Equal(t, 0.00000001, bonusSQLFloat(t, db, `SELECT spendable_bonus_balance(2)`))
	require.Zero(t, bonusSQLFloat(t, db, `SELECT COUNT(*) FROM users u WHERE u.bonus_balance<>(SELECT COALESCE(SUM(remaining),0) FROM bonus_balance_lots l WHERE l.user_id=u.id)`))
	require.Zero(t, bonusSQLFloat(t, db, `SELECT COUNT(*) FROM bonus_balance_lots WHERE expires_at<>bonus_balance_expires_at(granted_at)`))
}

func TestBonusExpiryPostgresWaitingBillingChecksTimeAfterLock(t *testing.T) {
	_, db := conversionTestService(t)
	installBonusExpiry(t, db)
	tx, err := db.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.Exec(`SELECT id FROM users WHERE id=1 FOR UPDATE;
UPDATE bonus_balance_lots SET expires_at=clock_timestamp()+INTERVAL '200 milliseconds' WHERE user_id=1;`)
	require.NoError(t, err)
	type result struct {
		balance float64
		err     error
	}
	done := make(chan result, 1)
	go func() {
		var balance float64
		err := db.QueryRow(`/* bonus_source:expiration */ SELECT expire_user_bonus_balance(1)`).Scan(&balance)
		done <- result{balance, err}
	}()
	// Wait for the statement to actually be blocked, then cross its deadline.
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%SELECT expire_user_bonus_balance(1)%' AND pid<>pg_backend_pid())`).Scan(&waiting)
		return err == nil && waiting
	}, 2*time.Second, 10*time.Millisecond)
	time.Sleep(250 * time.Millisecond)
	require.NoError(t, tx.Commit())
	got := <-done
	require.NoError(t, got.err)
	require.Zero(t, got.balance)
}
