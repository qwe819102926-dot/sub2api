//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLog_UpstreamModelMismatchFilterAndPartialIndex(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "model-audit@test.com"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-model-audit", Name: "model-audit"})
	account := mustCreateAccount(t, client, &service.Account{Name: "model-audit-account"})
	now := time.Now().UTC()
	responseModel := "gpt-5.4"
	for _, mismatch := range []bool{true, false} {
		mismatchValue := mismatch
		_, err := repo.Create(ctx, &service.UsageLog{
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			Model: "gpt-5.5", InputTokens: 1, OutputTokens: 1,
			UpstreamResponseModel: &responseModel, UpstreamModelMismatch: &mismatchValue,
			CreatedAt: now,
		})
		require.NoError(t, err)
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	trueValue := true
	stats, err := repo.GetStatsWithFilters(ctx, usagestats.UsageLogFilters{
		UserID: user.ID, StartTime: &start, EndTime: &end, UpstreamModelMismatch: &trueValue,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.TotalRequests)
	require.Equal(t, []usagestats.EndpointStat{{
		Endpoint: "unknown", Requests: 1, TotalTokens: 2,
	}}, stats.Endpoints)
	require.Equal(t, []usagestats.EndpointStat{{
		Endpoint: "unknown", Requests: 1, TotalTokens: 2,
	}}, stats.UpstreamEndpoints)
	require.Equal(t, []usagestats.EndpointStat{{
		Endpoint: "unknown -> unknown", Requests: 1, TotalTokens: 2,
	}}, stats.EndpointPaths)

	trend, err := repo.GetUsageTrendWithUsageFilters(ctx, start, end, "hour", usagestats.UsageLogFilters{
		UserID: user.ID, UpstreamModelMismatch: &trueValue,
	})
	require.NoError(t, err)
	require.Len(t, trend, 1)
	require.Equal(t, int64(1), trend[0].Requests)

	_, err = tx.ExecContext(ctx, "SET LOCAL enable_seqscan = off")
	require.NoError(t, err)
	assertPlanUsesIndex := func(query, indexName string, args ...any) {
		rows, queryErr := tx.QueryContext(ctx, query, args...)
		require.NoError(t, queryErr)
		var planLines []string
		for rows.Next() {
			var line string
			require.NoError(t, rows.Scan(&line))
			planLines = append(planLines, line)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		require.Contains(t, strings.Join(planLines, "\n"), indexName)
	}
	assertPlanUsesIndex(`
EXPLAIN (COSTS OFF)
SELECT id
FROM usage_logs
WHERE upstream_model_mismatch IS TRUE
ORDER BY created_at DESC, id DESC
LIMIT 100
`, usageLogsUpstreamModelMismatchIndex)
	assertPlanUsesIndex(`
EXPLAIN (COSTS OFF)
SELECT id
FROM usage_logs
WHERE COALESCE(NULLIF(TRIM(requested_model), ''), model) = $1
  AND created_at >= $2 AND created_at < $3
ORDER BY created_at DESC, id DESC
LIMIT 100
`, usageLogsEffectiveRequestedModelIndex, "gpt-5.5", start, end)
	assertPlanUsesIndex(`
EXPLAIN (COSTS OFF)
SELECT id
FROM usage_logs
WHERE COALESCE(NULLIF(TRIM(upstream_model), ''), model) = $1
  AND created_at >= $2 AND created_at < $3
ORDER BY created_at DESC, id DESC
LIMIT 100
`, usageLogsEffectiveUpstreamModelIndex, "gpt-5.5", start, end)
}

func TestUsageLog_GetStatsWithFilters_AggregatesAndEndpoints(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "stats@test.com"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-stats-1", Name: "k"})
	account := mustCreateAccount(t, client, &service.Account{Name: "acc-stats"})

	now := time.Now().UTC()
	inboundEndpoint := "/v1/messages"
	upstreamEndpoint := "/v1/responses"
	for i := 0; i < 3; i++ {
		_, err := repo.Create(ctx, &service.UsageLog{
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			Model: "claude-3", InputTokens: 2, OutputTokens: 3,
			CacheCreationTokens: 4, CacheReadTokens: 5,
			TotalCost: 0.5, ActualCost: 0.4, CreatedAt: now,
			InboundEndpoint: &inboundEndpoint, UpstreamEndpoint: &upstreamEndpoint,
		})
		require.NoError(t, err)
	}

	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)
	// 按本测试创建的 user 维度过滤:集成库为共享实例,其它用 testEntClient 的兄弟测试会留下
	// 已提交的 usage_log 行(含零 token 的失败请求),不限定 user 会把它们计入 TotalRequests。
	stats, err := repo.GetStatsWithFilters(ctx, usagestats.UsageLogFilters{UserID: user.ID, StartTime: &start, EndTime: &end})
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.TotalRequests)
	require.Equal(t, int64(6), stats.TotalInputTokens)
	require.Equal(t, int64(9), stats.TotalOutputTokens)
	require.Equal(t, int64(27), stats.TotalCacheTokens)
	require.Equal(t, int64(12), stats.TotalCacheCreationTokens)
	require.Equal(t, int64(15), stats.TotalCacheReadTokens)
	require.InDelta(t, 1.2, stats.TotalActualCost, 1e-9)
	require.NotNil(t, stats.TotalWalletCost)
	require.NotNil(t, stats.TotalBonusCost)
	require.NotNil(t, stats.TotalAccountCost)
	require.NotNil(t, stats.TotalProfit)
	require.InDelta(t, 1.2, *stats.TotalWalletCost, 1e-9)
	require.InDelta(t, 0, *stats.TotalBonusCost, 1e-9)
	require.InDelta(t, 1.5, *stats.TotalAccountCost, 1e-9)
	require.InDelta(t, -0.3, *stats.TotalProfit, 1e-9)
	require.NotEmpty(t, stats.Endpoints)
	require.NotEmpty(t, stats.UpstreamEndpoints)
	require.NotEmpty(t, stats.EndpointPaths)
}

func TestUsageLog_GetStatsWithFilters_WalletBonusProfit(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "profit-stats@test.com"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-profit-stats", Name: "profit"})
	account := mustCreateAccount(t, client, &service.Account{Name: "acc-profit-stats"})
	now := time.Now().UTC()

	mixedWallet := 0.75
	mixedBonus := 0.50 // billed equivalent would be actual-wallet=0.25
	zeroWallet := 0.0
	allBonus := 0.15 // billed equivalent would be 0.40
	legacyWallet := 0.60 // bonus_cost NULL -> billed difference 0.40
	subscriptionWallet := 2.0
	logs := []*service.UsageLog{
		{ // mixed principal + bonus: stored bonus_cost wins over billed difference
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			Model: "profit-mix", InputTokens: 1, OutputTokens: 1,
			TotalCost: 0.5, ActualCost: 1.0, WalletCost: &mixedWallet, BonusCost: &mixedBonus,
			BillingType: service.BillingTypeBalance, CreatedAt: now,
		},
		{ // all bonus: stored bonus_cost wins over billed difference
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			Model: "profit-bonus", InputTokens: 1, OutputTokens: 1,
			TotalCost: 0.3, ActualCost: 0.4, WalletCost: &zeroWallet, BonusCost: &allBonus,
			BillingType: service.BillingTypeBalance, CreatedAt: now,
		},
		{ // historical wallet_cost/bonus_cost NULL -> principal actual_cost, bonus 0
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			Model: "profit-legacy", InputTokens: 1, OutputTokens: 1,
			TotalCost: 0.1, ActualCost: 0.2, CreatedAt: now,
		},
		{ // historical bonus_cost NULL with wallet_cost set -> billed bonus equivalent
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			Model: "profit-legacy-wallet", InputTokens: 1, OutputTokens: 1,
			TotalCost: 0.2, ActualCost: 1.0, WalletCost: &legacyWallet,
			BillingType: service.BillingTypeBalance, CreatedAt: now,
		},
		{ // subscription is excluded from wallet/bonus but still counted in account cost
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			Model: "profit-sub", InputTokens: 1, OutputTokens: 1,
			TotalCost: 1.0, ActualCost: 2.0, WalletCost: &subscriptionWallet,
			BillingType: service.BillingTypeSubscription, CreatedAt: now,
		},
	}
	for _, log := range logs {
		_, err := repo.Create(ctx, log)
		require.NoError(t, err)
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	stats, err := repo.GetStatsWithFilters(ctx, usagestats.UsageLogFilters{UserID: user.ID, StartTime: &start, EndTime: &end})
	require.NoError(t, err)
	require.Equal(t, int64(5), stats.TotalRequests)
	require.NotNil(t, stats.TotalWalletCost)
	require.NotNil(t, stats.TotalBonusCost)
	require.NotNil(t, stats.TotalAccountCost)
	require.NotNil(t, stats.TotalProfit)
	require.InDelta(t, 1.55, *stats.TotalWalletCost, 1e-9)
	require.InDelta(t, 1.05, *stats.TotalBonusCost, 1e-9)
	require.InDelta(t, 2.1, *stats.TotalAccountCost, 1e-9)
	require.InDelta(t, -0.55, *stats.TotalProfit, 1e-9)
}
