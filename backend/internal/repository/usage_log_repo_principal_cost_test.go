package repository

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Opt in with SUB2API_TEST_PRINCIPAL_DSN. All fixtures are transaction-local
// temporary tables; no existing application tables or balances are changed.
func TestUsageLogPrincipalCostPostgres(t *testing.T) {
	dsn := os.Getenv("SUB2API_TEST_PRINCIPAL_DSN")
	if dsn == "" {
		t.Skip("set SUB2API_TEST_PRINCIPAL_DSN to run PostgreSQL accounting checks")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	// Match the read projection so the actual repository scanner is exercised.
	textColumns := strings.Fields("request_id model requested_model upstream_model upstream_response_model user_agent ip_address image_size image_input_size image_output_size image_size_source image_size_breakdown video_resolution service_tier reasoning_effort requested_reasoning_effort inbound_endpoint upstream_endpoint model_mapping_chain billing_tier billing_mode upstream_request_id session_id")
	boolColumns := strings.Fields("upstream_model_mismatch stream openai_ws_mode cache_ttl_overridden long_context_billing_applied native_compaction_v2")
	definitions := []string{}
	for _, column := range strings.Split(usageLogSelectColumns, ", ") {
		definition := column + " numeric DEFAULT 0"
		for _, name := range textColumns {
			if name == column {
				definition = column + " text DEFAULT ''"
			}
		}
		for _, name := range boolColumns {
			if name == column {
				definition = column + " boolean DEFAULT false"
			}
		}
		if column == "created_at" {
			definition = column + " timestamptz DEFAULT now()"
		}
		definitions = append(definitions, definition)
	}
	definitions = append(definitions, "wallet_cost numeric", "bonus_cost numeric")
	_, err = tx.ExecContext(ctx, "CREATE TEMP TABLE usage_logs ("+strings.Join(definitions, ",")+") ON COMMIT DROP")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
		CREATE TEMP TABLE groups (id bigint, name text, platform text) ON COMMIT DROP;
		CREATE TEMP TABLE accounts (id bigint, platform text) ON COMMIT DROP;
		CREATE TEMP TABLE api_keys (user_id bigint, status text, deleted_at timestamptz) ON COMMIT DROP;
		INSERT INTO groups VALUES (1, 'group', 'openai');
	`)
	require.NoError(t, err)
	now := time.Now().UTC()
	// Each request has billed cost $1. Gift-only consumes $2.50 of gift
	// balance; mixed consumes $1.50 gift and $0.40 principal.
	_, err = tx.ExecContext(ctx, `INSERT INTO usage_logs
		(id,user_id,model,group_id,inbound_endpoint,upstream_endpoint,input_tokens,total_cost,actual_cost,wallet_cost,bonus_cost,billing_type,created_at)
		VALUES
		(1,42,'model',1,'/v1/responses','/v1/responses',10,1,1,0,2.5,0,$1),
		(2,42,'model',1,'/v1/responses','/v1/responses',10,1,1,0.4,1.5,0,$1),
		(3,42,'model',1,'/v1/responses','/v1/responses',10,1,1,1,0,0,$1),
		(4,42,'model',1,'/v1/responses','/v1/responses',10,1,1,0,0,1,$1),
		(5,42,'model',1,'/v1/responses','/v1/responses',10,1,1,NULL,NULL,0,$1),
		(6,99,'model',1,'/v1/responses','/v1/responses',10,9,9,9,0,0,$1)`, now)
	require.NoError(t, err)
	repo := &usageLogRepository{sql: tx}
	expected := []float64{0, 0.4, 1, 1, 1}
	for i, want := range expected {
		userLog, err := repo.GetByIDWithPrincipalCost(ctx, int64(i+1))
		require.NoError(t, err)
		require.InDelta(t, want, userLog.ActualCost, 1e-9)
		adminLog, err := repo.GetByID(ctx, int64(i+1))
		require.NoError(t, err)
		require.Equal(t, 1.0, adminLog.ActualCost)
	}
	for _, fast := range []bool{false, true} {
		params := pagination.PaginationParams{Page: 1, PageSize: 100}
		var logsCount int
		var sum float64
		if fast {
			logs, _, err := repo.listUsageLogsWithFastPagination(ctx, "WHERE user_id = $1", []any{42}, params, true)
			require.NoError(t, err)
			logsCount = len(logs)
			for _, log := range logs {
				sum += log.ActualCost
			}
		} else {
			logs, _, err := repo.listUsageLogsWithPagination(ctx, "WHERE user_id = $1", []any{42}, params, true)
			require.NoError(t, err)
			logsCount = len(logs)
			for _, log := range logs {
				sum += log.ActualCost
			}
		}
		require.Equal(t, 5, logsCount)
		require.InDelta(t, 3.4, sum, 1e-9)
	}
	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	for _, principalOnly := range []bool{true, false} {
		want := 5.0
		if principalOnly {
			want = 3.4
		}
		filters := UsageLogFilters{UserID: 42, StartTime: &start, EndTime: &end, PrincipalCostOnly: principalOnly}
		stats, err := repo.GetStatsWithFilters(ctx, filters)
		require.NoError(t, err)
		require.InDelta(t, want, stats.TotalActualCost, 1e-9)
		require.Equal(t, int64(5), stats.TotalRequests)
		require.Len(t, stats.Endpoints, 1)
		require.InDelta(t, want, stats.Endpoints[0].ActualCost, 1e-9)
		trend, err := repo.GetUsageTrendWithUsageFilters(ctx, start, end, "hour", filters)
		require.NoError(t, err)
		require.Len(t, trend, 1)
		require.InDelta(t, want, trend[0].ActualCost, 1e-9)
		models, err := repo.GetModelStatsWithUsageFiltersBySource(ctx, start, end, filters, usagestats.ModelSourceRequested)
		require.NoError(t, err)
		require.Len(t, models, 1)
		require.InDelta(t, want, models[0].ActualCost, 1e-9)
		groups, err := repo.GetGroupStatsWithUsageFilters(ctx, start, end, filters)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		require.InDelta(t, want, groups[0].ActualCost, 1e-9)
	}
	dashboard, err := repo.GetUserDashboardStats(ctx, 42)
	require.NoError(t, err)
	require.InDelta(t, 3.4, dashboard.TotalActualCost, 1e-9)
	require.InDelta(t, 3.4, dashboard.TodayActualCost, 1e-9)
	require.Len(t, dashboard.ByPlatform, 1)
	require.InDelta(t, 3.4, dashboard.ByPlatform[0].TotalActualCost, 1e-9)
	var rawCost, giftCost float64
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT actual_cost, bonus_cost FROM usage_logs WHERE id = 1").Scan(&rawCost, &giftCost))
	require.Equal(t, 1.0, rawCost)
	require.Equal(t, 2.5, giftCost)
}
