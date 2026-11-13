package service

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
)

func TestOpenAISchedulingRateFallback(t *testing.T) {
	now := time.Now()
	ptr := func(v float64) *float64 { return &v }
	fresh := upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 0.02, now.Add(-time.Minute), 30*time.Minute)
	fresh.RateMultiplier = ptr(0.035)
	stale := upstreamCostTestAccount(2, UpstreamBillingProbeStatusOK, 0.02, now.Add(-2*time.Hour), 30*time.Minute)
	stale.RateMultiplier = ptr(0.03)
	unsupported := upstreamCostTestAccount(3, UpstreamBillingProbeStatusUnsupported, 0.02, now.Add(-time.Minute), 30*time.Minute)
	unsupported.RateMultiplier = ptr(0.04)
	invalidProbe := upstreamCostTestAccount(4, UpstreamBillingProbeStatusOK, -1, now.Add(-time.Minute), 30*time.Minute)
	invalidProbe.RateMultiplier = ptr(0.05)
	failedWithFreshCache := upstreamCostTestAccount(5, UpstreamBillingProbeStatusFailed, 0.025, now.Add(-time.Minute), 30*time.Minute)
	failedWithFreshCache.RateMultiplier = ptr(0.06)
	tests := []struct {
		name   string
		a      *Account
		want   float64
		source string
		ok     bool
	}{
		{"fresh probe overrides saved", fresh, 0.02, "probe", true},
		{"expired probe uses saved", stale, 0.03, "account", true},
		{"unsupported probe uses saved", unsupported, 0.04, "account", true},
		{"invalid probe uses saved", invalidProbe, 0.05, "account", true},
		{"fresh cached success survives failed refresh", failedWithFreshCache, 0.025, "probe", true},
		{"manual without probe", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: ptr(0.03)}, 0.03, "account", true},
		{"free account", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: ptr(0)}, 0, "account", true},
		{"unconfigured uses billing default", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, 1, "default", true},
		{"oauth keeps reference", upstreamCostTestOAuthAccount(6), 0.5, "oauth", true},
		{"nil", nil, 0, "", false},
		{"other supported platform", &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, RateMultiplier: ptr(0.01)}, 0.01, "account", true},
		{"unknown platform ignored", &Account{Platform: "unknown", Type: AccountTypeAPIKey, RateMultiplier: ptr(0.01)}, 0, "", false},
		{"negative saved rate rejected", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: ptr(-1)}, 0, "", false},
		{"nan saved rate rejected", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: ptr(math.NaN())}, 0, "", false},
		{"infinite saved rate rejected", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: ptr(math.Inf(1))}, 0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, source, ok := openAISchedulingRateWithSource(tt.a, now, 0.5)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.source, source)
			require.Equal(t, tt.want, rate)
			plain, plainOK := openAISchedulingRate(tt.a, now, 0.5)
			require.Equal(t, ok, plainOK)
			require.Equal(t, rate, plain)
		})
	}
}

func TestOpenAISavedRatesAffectBothSchedulers(t *testing.T) {
	cheap, expensive := 0.02, 0.035
	a := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: &cheap}
	b := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: &expensive}
	now := time.Now()
	factors := openAIUpstreamCostFactors([]*Account{a, b}, now, 1)
	require.Greater(t, factors[1], factors[2])
	legacy := newOpenAILegacyUpstreamRateOrder([]*Account{a, b}, now, 1)
	require.True(t, legacy.enabled)
	require.Negative(t, legacy.compare(a, b))
}
