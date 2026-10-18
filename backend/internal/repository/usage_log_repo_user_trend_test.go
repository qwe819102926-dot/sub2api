package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetUserUsageTrendSelectsRankingMetric(t *testing.T) {
	for _, tc := range []struct {
		metric string
		rank   string
	}{
		{"tokens", "SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens)"},
		{"actual_cost", "SUM(actual_cost)"},
		{"unexpected", "SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens)"},
	} {
		t.Run(tc.metric, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := &usageLogRepository{sql: db}
			start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			end := start.Add(24 * time.Hour)
			query := regexp.QuoteMeta("ORDER BY "+tc.rank+" DESC, user_id ASC") +
				`\s+LIMIT \$3[\s\S]+` + regexp.QuoteMeta("COALESCE(SUM(u.actual_cost), 0) as actual_cost")
			mock.ExpectQuery(query).WithArgs(start, end, 12, start, end).
				WillReturnRows(sqlmock.NewRows([]string{"date", "user_id", "email", "username", "requests", "tokens", "cost", "actual_cost"}).
					AddRow("2026-10-01", int64(7), "user@example.com", "user", int64(2), int64(100), 10.0, 5.0))
			points, err := repo.GetUserUsageTrend(context.Background(), start, end, "day", 12, tc.metric)
			require.NoError(t, err)
			require.Len(t, points, 1)
			require.EqualValues(t, 100, points[0].Tokens)
			require.Equal(t, 5.0, points[0].ActualCost)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
