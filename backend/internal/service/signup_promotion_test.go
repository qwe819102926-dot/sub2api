//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type promotionRepoStub struct {
	AffiliateRepository
	calls  int
	code   string
	config SignupPromotionConfig
	err    error
}

func (r *promotionRepoStub) ApplySignupPromotion(_ context.Context, _ int64, code string, cfg SignupPromotionConfig) ([]int64, error) {
	r.calls++
	r.code, r.config = code, cfg
	return nil, r.err
}
func (r *promotionRepoStub) GetSignupPromotionDetail(context.Context, int64, int, int) (*SignupPromotionDetail, error) {
	return nil, nil
}

func TestPromotionIndependentOfDisabledRechargeRebates(t *testing.T) {
	settings := &SettingService{settingRepo: &settingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled: "false", SettingKeySignupBonusEnabled: "true", SettingKeySignupBonusAmount: "50", SettingKeyInviteBonusEnabled: "true", SettingKeyInviteBonusAmount: "50",
	}}}
	repo := &promotionRepoStub{}
	s := NewAffiliateService(repo, settings, nil, nil)
	require.False(t, s.IsEnabled(context.Background()))
	require.NoError(t, s.ApplySignupPromotion(context.Background(), 2, "  friend_2026  "))
	require.Equal(t, 1, repo.calls)
	require.Equal(t, "FRIEND_2026", repo.code)
	require.Equal(t, float64(50), repo.config.SignupBonusAmount)
	require.Equal(t, float64(50), repo.config.InviteBonusAmount)
	amount, err := s.AccrueInviteRebate(context.Background(), 2, 100)
	require.NoError(t, err)
	require.Zero(t, amount)
}

func TestPromotionDisabledAndInvalidCode(t *testing.T) {
	store := &settingRepoStub{values: map[string]string{}}
	repo := &promotionRepoStub{}
	s := NewAffiliateService(repo, &SettingService{settingRepo: store}, nil, nil)
	require.NoError(t, s.ApplySignupPromotion(context.Background(), 2, "FRIEND"))
	require.Zero(t, repo.calls)
	store.values[SettingKeySignupBonusEnabled] = "true"
	require.NoError(t, s.ApplySignupPromotion(context.Background(), 2, "invalid code!"))
	require.Equal(t, "", repo.code)
	require.Equal(t, 1, repo.calls)
	repo.err = errors.New("gift transaction failed")
	require.Error(t, s.ApplySignupPromotion(context.Background(), 2, "FRIEND"))
}

func TestPromotionSignupDoesNotStackDefaultPrincipal(t *testing.T) {
	settings := &SettingService{cfg: &config.Config{}, settingRepo: &settingRepoStub{values: map[string]string{
		SettingKeySignupBonusEnabled: "true", SettingKeyDefaultBalance: "999", SettingKeyDefaultConcurrency: "3",
	}}}
	s := &AuthService{cfg: &config.Config{}, settingService: settings}
	for _, source := range []string{"email", "github", "google", "linuxdo", "wechat", "dingtalk", "oidc"} {
		t.Run(source, func(t *testing.T) { require.Zero(t, s.resolveSignupGrantPlan(context.Background(), source).Balance) })
	}
}

func TestPromotionAmountParsingRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-Inf", "-1", "bad", ""} {
		cfg := promotionConfigFromValues(map[string]string{SettingKeySignupBonusAmount: value})
		require.Equal(t, float64(10), cfg.SignupBonusAmount)
	}
	require.Zero(t, promotionConfigFromValues(map[string]string{SettingKeySignupBonusAmount: "0"}).SignupBonusAmount)
}
