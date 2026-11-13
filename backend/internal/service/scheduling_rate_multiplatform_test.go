package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestSchedulingRatesAllAPIKeyPlatforms(t *testing.T) {
	now := time.Now()
	for _, platform := range []string{
		PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity,
		PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek,
	} {
		t.Run(platform, func(t *testing.T) {
			for _, tc := range []struct {
				name, status, source string
				age                  time.Duration
				probe, saved, want   float64
			}{
				{"fresh", UpstreamBillingProbeStatusOK, "probe", time.Minute, 0.02, 0.04, 0.02},
				{"expired", UpstreamBillingProbeStatusOK, "account", 2 * time.Hour, 0.02, 0.01, 0.01},
				{"failed", UpstreamBillingProbeStatusFailed, "account", 2 * time.Hour, 0.02, 0.01, 0.01},
				{"failed with fresh cache", UpstreamBillingProbeStatusFailed, "probe", time.Minute, 0.02, 0.04, 0.02},
				{"unsupported", UpstreamBillingProbeStatusUnsupported, "account", time.Minute, 0.02, 0.01, 0.01},
				{"invalid probe", UpstreamBillingProbeStatusOK, "account", time.Minute, -1, 0.01, 0.01},
			} {
				t.Run(tc.name, func(t *testing.T) {
					a := upstreamCostTestAccount(1, tc.status, tc.probe, now.Add(-tc.age), 30*time.Minute)
					a.Platform = platform
					a.RateMultiplier = &tc.saved
					rate, source, ok := openAISchedulingRateWithSource(a, now, 1)
					require.True(t, ok)
					require.Equal(t, tc.want, rate)
					require.Equal(t, tc.source, source)
				})
			}
			manual := &Account{Platform: platform, Type: AccountTypeAPIKey}
			rate, source, ok := openAISchedulingRateWithSource(manual, now, 1)
			require.True(t, ok)
			require.Equal(t, 1.0, rate)
			require.Equal(t, "default", source)
			zero := 0.0
			manual.RateMultiplier = &zero
			rate, source, ok = openAISchedulingRateWithSource(manual, now, 1)
			require.True(t, ok)
			require.Zero(t, rate)
			require.Equal(t, "account", source)
			for _, invalid := range []float64{-1, math.NaN(), math.Inf(1)} {
				manual.RateMultiplier = &invalid
				_, _, ok := openAISchedulingRateWithSource(manual, now, 1)
				require.False(t, ok)
			}
		})
	}
}

func TestSchedulingCostScoresAndDiagnosticsAllAPIKeyPlatforms(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
	now := time.Now()
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.UpstreamCost = 12
	for _, platform := range []string{
		PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity,
		PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek,
	} {
		t.Run(platform, func(t *testing.T) {
			cheapRate, expensiveRate := 0.01, 0.04
			cheap := &Account{ID: 2211, Name: "wk", Platform: platform, Type: AccountTypeAPIKey, RateMultiplier: &cheapRate}
			expensive := &Account{ID: 2198, Platform: platform, Type: AccountTypeAPIKey, RateMultiplier: &expensiveRate}
			accounts := []*Account{cheap, expensive}
			legacy := newOpenAILegacyUpstreamRateOrder(accounts, now, 1)
			require.True(t, legacy.enabled)
			require.Negative(t, legacy.compare(cheap, expensive))
			scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{cfg: cfg}}
			d := &SchedulerDiagnostic{}
			ctx := context.WithValue(context.Background(), schedulerDiagnosticKey{}, d)
			plan := scheduler.buildOpenAIAccountLoadPlan(ctx, OpenAIAccountScheduleRequest{Platform: platform, UseUpstreamTokenCost: true}, accounts, nil)
			require.Greater(t, plan.candidates[0].score, plan.candidates[1].score)
			require.Equal(t, cheap.ID, plan.selectionOrder[0].account.ID)
			require.True(t, plan.includeOverflowFallback)
			require.Len(t, d.Accounts, 2)
			require.NotNil(t, d.Accounts[0].Rate)
			require.Equal(t, cheapRate, *d.Accounts[0].Rate)
			require.Equal(t, "account", d.Accounts[0].RateSource)
			require.Greater(t, d.Accounts[0].Parts["cost"], d.Accounts[1].Parts["cost"])
			withoutCost := scheduler.buildOpenAIAccountLoadPlan(context.Background(), OpenAIAccountScheduleRequest{Platform: platform}, accounts, nil)
			require.Equal(t, withoutCost.candidates[0].score, withoutCost.candidates[1].score)
		})
	}
}

func TestCompatiblePlatformSchedulerSelectsSavedRateOnProbeFailure(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
	now := time.Now()
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.UpstreamCost = 12
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 3
	group := int64(918274)
	for _, platform := range []string{PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek} {
		t.Run(platform, func(t *testing.T) {
			model := "test-model"
			if platform == PlatformGrok {
				model = xai.DefaultTextModel
			}
			cheapRate := 0.01
			cheap := upstreamCostTestAccount(2211, UpstreamBillingProbeStatusFailed, 0.02, now.Add(-2*time.Hour), 30*time.Minute)
			cheap.RateMultiplier = &cheapRate
			expensive := upstreamCostTestAccount(2198, UpstreamBillingProbeStatusOK, 0.04, now.Add(-time.Minute), 30*time.Minute)
			for _, a := range []*Account{cheap, expensive} {
				a.Platform = platform
				a.Status = StatusActive
				a.Schedulable = true
				a.Concurrency = 100
				a.GroupIDs = []int64{group}
			}
			service := &OpenAIGatewayService{
				cfg:                cfg,
				accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{*cheap, *expensive}},
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			}
			stats := newOpenAIAccountRuntimeStats()
			cheapTTFT, expensiveTTFT := 5114, 1894
			stats.report(cheap.ID, true, &cheapTTFT)
			stats.report(expensive.ID, true, &expensiveTTFT)
			scheduler := newDefaultOpenAIAccountScheduler(service, stats)
			selection, decision, err := scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{
				GroupID: &group, Platform: platform, RequestedModel: model, UseUpstreamTokenCost: true,
			})
			require.NoError(t, err)
			require.NotNil(t, selection)
			t.Cleanup(selection.ReleaseFunc)
			require.Equal(t, cheap.ID, selection.Account.ID)
			require.Equal(t, cheap.ID, decision.SelectedAccountID)
			require.Equal(t, platform, selection.Account.Platform)
		})
	}
}
