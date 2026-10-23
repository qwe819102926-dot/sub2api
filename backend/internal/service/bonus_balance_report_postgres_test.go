package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBonusReportNewEpochPostgres(t *testing.T) {
	s, db := conversionTestService(t)
	ctx := context.Background()
	report, err := s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{})
	require.NoError(t, err)
	require.Zero(t, report.Summary.Issued, "existing wallets and grants must not enter new totals")
	require.Zero(t, report.Total)
	started := *report.StartedAt
	// Includes a historical import, a positive conversion, unexplained increase,
	// admin grant, deduction, and the smallest wallet unit.
	for _, statement := range []string{
		`/* bonus_source:checkin */ UPDATE users SET bonus_balance=bonus_balance+0.33333333 WHERE id=1`,
		`/* bonus_source:signup */ UPDATE users SET bonus_balance=bonus_balance+0.00000001 WHERE id=1`,
		`/* bonus_source:admin */ UPDATE users SET bonus_balance=bonus_balance+2 WHERE id=2`,
		`UPDATE users SET bonus_balance=bonus_balance+7 WHERE id=1`,
		`/* bonus_source:conversion */ UPDATE users SET bonus_balance=bonus_balance+9 WHERE id=1`,
		`/* bonus_source:consumption */ UPDATE users SET bonus_balance=bonus_balance-1 WHERE id=1`,
		`INSERT INTO bonus_balance_events(user_id,email,source,amount,historical) VALUES(1,'a','checkin',100,TRUE)`,
	} {
		_, err = db.Exec(statement)
		require.NoError(t, err)
	}
	report, err = s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{})
	require.NoError(t, err)
	require.InDelta(t, 2.33333334, report.Summary.Issued, 1e-10)
	require.EqualValues(t, 3, report.Summary.Grants)
	require.EqualValues(t, 2, report.Summary.Users)
	require.Equal(t, 7.0, report.Summary.Pending)
	require.EqualValues(t, 4, report.Total)
	var sum float64
	for _, source := range report.Sources {
		sum += source.Amount
	}
	require.InDelta(t, report.Summary.Issued, sum, 1e-10)
	require.InDelta(t, report.Summary.Issued, report.Summary.TodayIssued, 1e-10)
	filtered, err := s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{Source: "checkin", Search: "1"})
	require.NoError(t, err)
	require.EqualValues(t, 1, filtered.Total)
	require.Equal(t, 0.33333333, filtered.Summary.Issued)
	require.EqualValues(t, 1, filtered.Summary.Users)
	require.Len(t, filtered.Sources, 1)
	// Eight-decimal precision and literal-email search must survive the report path.
	filtered, err = s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{Source: "signup", Search: "a"})
	require.NoError(t, err)
	require.Equal(t, 0.00000001, filtered.Summary.Issued)
	// Local-day bounds must include UTC instants belonging to that Shanghai day.
	_, err = db.Exec(`INSERT INTO bonus_balance_events(user_id,email,source,amount,created_at) VALUES
 (1,'a','checkin',3,'2020-01-01T16:00:00Z'),(1,'a','checkin',5,'2020-01-02T15:59:59Z'),(1,'a','checkin',11,'2020-01-02T16:00:00Z')`)
	require.NoError(t, err)
	filtered, err = s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{StartDate: "2020-01-02", EndDate: "2020-01-02", PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, 8.0, filtered.Summary.Issued, "event watermark, rather than transaction timestamp, defines the boundary")
	require.EqualValues(t, 2, filtered.Total)
	require.Len(t, filtered.Records, 1)
	require.Zero(t, filtered.Summary.TodayIssued)
	// Installing again must preserve the boundary and totals.
	migration, err := migrations.FS.ReadFile("252_bonus_reporting_epoch.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	report, err = s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{})
	require.NoError(t, err)
	require.True(t, started.Equal(*report.StartedAt))
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`/* bonus_source:checkin */ UPDATE users SET bonus_balance=bonus_balance+50 WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	after, err := s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{})
	require.NoError(t, err)
	require.Equal(t, report.Summary, after.Summary)
	// Deleting a recipient must not erase the previously issued grants.
	_, err = db.Exec(`DELETE FROM users WHERE id=1`)
	require.NoError(t, err)
	after, err = s.GetBonusBalanceReport(ctx, BonusBalanceReportFilter{})
	require.NoError(t, err)
	require.Equal(t, report.Summary, after.Summary)
	require.WithinDuration(t, started, *after.StartedAt, time.Nanosecond)
}
