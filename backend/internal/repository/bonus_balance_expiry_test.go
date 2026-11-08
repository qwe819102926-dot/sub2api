package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func expiryTestRepo(t *testing.T) (*userRepository, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("SUB2API_CONVERSION_TEST_DSN")
	if dsn == "" {
		t.Skip("set SUB2API_CONVERSION_TEST_DSN to run native PostgreSQL expiry tests")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("bonus_expiry_repo_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = base.Close() })
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,email TEXT,balance NUMERIC(20,8),bonus_balance NUMERIC(20,8),deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT);
INSERT INTO settings VALUES('BONUS_BALANCE_CONSUMPTION_RATE','2');
CREATE TABLE bonus_balance_events(id BIGSERIAL PRIMARY KEY,event_key TEXT UNIQUE,user_id BIGINT,email TEXT,source TEXT,amount NUMERIC(20,8),balance_after NUMERIC(20,8),historical BOOL DEFAULT FALSE,created_at TIMESTAMPTZ DEFAULT NOW(),updated_at TIMESTAMPTZ DEFAULT NOW());
INSERT INTO users VALUES(1,'expiry',20,10,NULL,NOW());
INSERT INTO bonus_balance_events(user_id,email,source,amount,balance_after) VALUES(1,'expiry','checkin',10,10);`)
	require.NoError(t, err)
	for _, file := range []string{"251_bonus_balance_conversion.sql", "254_bonus_balance_expiry.sql"} {
		if file == "254_bonus_balance_expiry.sql" {
			_, err = db.Exec(`CREATE TRIGGER test_audit AFTER UPDATE OF bonus_balance ON users FOR EACH ROW EXECUTE FUNCTION audit_bonus_balance_change()`)
			require.NoError(t, err)
		}
		source, err := migrations.FS.ReadFile(file)
		require.NoError(t, err)
		_, err = db.Exec(string(source))
		require.NoError(t, err)
	}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return newUserRepositoryWithSQL(client, db), db
}

func TestBonusExpiryRepositoryBillingCannotSpendExpiredCredits(t *testing.T) {
	repo, db := expiryTestRepo(t)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE bonus_balance_lots SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=1`)
	require.NoError(t, err)
	credit, err := repo.GetBonusBalanceCredit(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, credit, "auth/precheck must reject expired credits before cleanup")
	balances, err := repo.GetBonusBalancesByUserIDs(ctx, []int64{1})
	require.NoError(t, err)
	require.Zero(t, balances[1])
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	principal, sufficient, bonusUsed, principalUsed, err := deductUsageBillingBalanceDetailed(ctx, tx, 1, 3, 2)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.Equal(t, 17.0, principal)
	require.Zero(t, bonusUsed)
	require.Equal(t, 3.0, principalUsed)
	require.NoError(t, tx.Commit())
	var expiration, consumption float64
	require.NoError(t, db.QueryRow(`SELECT COALESCE(-SUM(amount) FILTER(WHERE source='expiration'),0),COALESCE(-SUM(amount) FILTER(WHERE source='consumption'),0) FROM bonus_balance_events`).Scan(&expiration, &consumption))
	require.Equal(t, 10.0, expiration)
	require.Zero(t, consumption, "expiration must never inflate consumed bonus")
}

func TestBonusExpiryRepositoryMixedLotsAndMultiplier(t *testing.T) {
	repo, db := expiryTestRepo(t)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE bonus_balance_lots SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=1`)
	require.NoError(t, err)
	_, err = db.Exec(`/* bonus_source:checkin */ UPDATE users SET bonus_balance=bonus_balance+4 WHERE id=1`)
	require.NoError(t, err)
	credit, err := repo.GetBonusBalanceCredit(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 2.0, credit)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	principal, _, bonusUsed, principalUsed, err := deductUsageBillingBalanceDetailed(ctx, tx, 1, 3, 2)
	require.NoError(t, err)
	require.Equal(t, 19.0, principal)
	require.Equal(t, 4.0, bonusUsed)
	require.Equal(t, 1.0, principalUsed)
	require.NoError(t, tx.Commit())
}

func TestBonusExpiryRepositoryAdminSetAndAdjustUseLiveBalance(t *testing.T) {
	for _, absolute := range []bool{true, false} {
		t.Run(fmt.Sprint(absolute), func(t *testing.T) {
			repo, db := expiryTestRepo(t)
			_, err := db.Exec(`UPDATE bonus_balance_lots SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=1`)
			require.NoError(t, err)
			var change service.BalanceChange
			if absolute {
				change, err = repo.SetBonusBalance(context.Background(), 1, 5)
			} else {
				change, err = repo.AdjustBonusBalance(context.Background(), 1, 5)
			}
			require.NoError(t, err)
			require.Zero(t, change.Old)
			require.Equal(t, 5.0, change.New)
			credit, err := repo.GetBonusBalanceCredit(context.Background(), 1)
			require.NoError(t, err)
			require.Equal(t, 2.5, credit)
		})
	}
}
