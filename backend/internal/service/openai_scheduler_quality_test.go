package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestOpenAISchedulerQualityAgesOut(t *testing.T) {
	s := newOpenAIAccountRuntimeStats()
	slow := 20000
	s.report(1, false, &slow)
	now := time.Now()
	fresh := s.latencyFactor(1, float64(slow), now)
	require.Less(t, fresh, 0.5)
	s.loadOrCreate(1).lastResultAt.Store(now.Add(-time.Hour).UnixNano())
	s.loadOrCreate(1).lastTTFTAt.Store(now.Add(-time.Hour).UnixNano())
	require.Greater(t, s.latencyFactor(1, float64(slow), now), fresh)
	rate, _, _ := s.snapshot(1)
	require.Less(t, rate, 0.01)
	require.InDelta(t, 0.5, s.latencyFactor(1, float64(slow), now), 0.01)
}

func TestOpenAISchedulerStickyDoesNotOverrideScoreOrder(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{}
	req := OpenAIAccountScheduleRequest{StickyWeighted: true, StickyAccountID: 1}
	plan := openAIAccountLoadPlan{topK: 2, candidates: []openAIAccountCandidateScore{
		{account: &Account{ID: 1}, score: 0}, {account: &Account{ID: 2}, score: 1000},
	}}
	wins := 0
	for i := 0; i < 100; i++ {
		req.SessionHash = string(rune(i + 100))
		order := scheduler.buildOpenAISelectionOrder(req, plan)
		if order[0].account.ID == 2 {
			wins++
		}
	}
	require.Greater(t, wins, 90)
}

func TestOpenAISchedulerQualityCombinedRanking(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
	now := time.Now()
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.UpstreamCost = 12
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 3
	stats := newOpenAIAccountRuntimeStats()
	scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{cfg: cfg}, stats: stats}
	accounts := []*Account{
		upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 0.02, now.Add(-time.Minute), 30*time.Minute),
		upstreamCostTestAccount(2, UpstreamBillingProbeStatusOK, 0.035, now.Add(-time.Minute), 30*time.Minute),
	}
	fast, slow := 1000, 20000
	stats.report(1, true, &fast)
	stats.report(2, true, &slow)
	req := OpenAIAccountScheduleRequest{UseUpstreamTokenCost: true}
	plan := scheduler.buildOpenAIAccountLoadPlan(context.Background(), req, accounts, nil)
	require.Greater(t, plan.candidates[0].score, plan.candidates[1].score)
	require.Equal(t, int64(1), plan.selectionOrder[0].account.ID)
	// With identical prices, faster accounts still rank higher.
	accounts[1] = upstreamCostTestAccount(2, UpstreamBillingProbeStatusOK, 0.02, now.Add(-time.Minute), 30*time.Minute)
	plan = scheduler.buildOpenAIAccountLoadPlan(context.Background(), OpenAIAccountScheduleRequest{}, accounts, nil)
	require.Greater(t, plan.candidates[0].score, plan.candidates[1].score)
}
