package service

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSchedulingDiagnosticStoreBoundsAndExpiry(t *testing.T) {
	store := &schedulerDiagnosticStore{}
	now := time.Now()
	for i := 0; i < 150; i++ {
		store.add(SchedulerDiagnostic{ID: uint64(i), At: now, GroupID: int64(i%2 + 1)})
	}
	require.Len(t, store.traces, 100)
	all := store.list(0, now)
	require.Len(t, all, 30)
	require.Equal(t, uint64(149), all[0].ID)
	for _, d := range store.list(1, now) {
		require.Equal(t, int64(1), d.GroupID)
	}
	require.Empty(t, store.list(0, now.Add(16*time.Minute)))
}

func TestSchedulingDiagnosticConcurrentReaders(t *testing.T) {
	store := &schedulerDiagnosticStore{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				store.add(SchedulerDiagnostic{At: time.Now()})
				_ = store.list(0, time.Now())
			}
		}()
	}
	wg.Wait()
	require.Len(t, store.traces, 100)
}

func TestSchedulingDiagnosticBoundedRowsAndRedaction(t *testing.T) {
	d := &SchedulerDiagnostic{}
	ctx := context.WithValue(context.Background(), schedulerDiagnosticKey{}, d)
	for i := 1; i <= 250; i++ {
		recordSchedulerDiagnosticExclusion(ctx, &Account{ID: int64(i), Name: "test", Credentials: map[string]any{"api_key": "secret-key"}}, "model_not_supported")
	}
	require.Len(t, d.Accounts, 200)
	require.True(t, d.Truncated)
	recordSchedulerDiagnosticExclusion(ctx, &Account{ID: 1}, "runtime_blocked")
	require.Equal(t, "runtime_blocked", d.Accounts[0].Reason)
	data, err := json.Marshal(d)
	require.NoError(t, err)
	require.False(t, strings.Contains(string(data), "secret-key"))
	require.NotContains(t, string(data), "credentials")
}

func TestSchedulingDiagnosticScoresMatchSelection(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.UpstreamCost = 12
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 3
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.SessionSticky = 0.5
	now := time.Now()
	accounts := []*Account{
		upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 0.02, now.Add(-time.Minute), 30*time.Minute),
		upstreamCostTestAccount(2, UpstreamBillingProbeStatusOK, 0.035, now.Add(-time.Minute), 30*time.Minute),
	}
	scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{cfg: cfg}, stats: newOpenAIAccountRuntimeStats()}
	fast := 1000
	scheduler.stats.report(1, true, &fast)
	req := OpenAIAccountScheduleRequest{UseUpstreamTokenCost: true, StickyWeighted: true, StickyAccountID: 1}
	d := &SchedulerDiagnostic{}
	ctx := context.WithValue(context.Background(), schedulerDiagnosticKey{}, d)
	plan := scheduler.buildOpenAIAccountLoadPlan(ctx, req, accounts, nil)
	require.Len(t, d.Accounts, 2)
	for i, row := range d.Accounts {
		require.NotNil(t, row.Score)
		sum := 0.0
		for _, v := range row.Parts {
			sum += v
		}
		require.InDelta(t, plan.candidates[i].score, *row.Score, 1e-9)
		require.InDelta(t, *row.Score, sum, 1e-9)
		require.NotNil(t, row.Rate)
	}
	require.Equal(t, 1, d.Accounts[0].Order)
	recordSchedulerDiagnosticExclusion(ctx, accounts[0], "slot_unavailable")
	require.NotNil(t, d.Accounts[0].Score)
	require.Equal(t, "slot_unavailable", d.Accounts[0].Reason)
}

func TestSchedulingDiagnosticCapturesRealSelection(t *testing.T) {
	group := int64(918273)
	accounts := []Account{
		{ID: 991, Name: "allowed", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 10, GroupIDs: []int64{group}},
		{ID: 992, Name: "unsupported", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 10, GroupIDs: []int64{group}, Credentials: map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}},
	}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cfg: &config.Config{}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
	scheduler := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
	selected, decision, err := scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{GroupID: &group, Platform: PlatformOpenAI, RequestedModel: "test-model", UseUpstreamTokenCost: true})
	require.NoError(t, err)
	require.NotNil(t, selected)
	if selected.ReleaseFunc != nil {
		defer selected.ReleaseFunc()
	}
	traces := RecentSchedulerDiagnostics(group)
	require.NotEmpty(t, traces)
	require.Equal(t, decision.SelectedAccountID, traces[0].SelectedID)
	require.Equal(t, int64(991), traces[0].SelectedID)
	require.Equal(t, "load_balance", traces[0].Layer)
	require.Len(t, traces[0].Accounts, 2)
	for _, row := range traces[0].Accounts {
		if row.ID == 992 {
			require.Equal(t, "model_not_supported", row.Reason)
		} else {
			require.NotNil(t, row.Score)
		}
	}
}
