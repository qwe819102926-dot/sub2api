//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestInviteRewardsPageSettings(t *testing.T) {
	ctx := context.Background()
	for _, raw := range []string{"", "false", "true"} {
		t.Run("public_and_injected_"+raw, func(t *testing.T) {
			svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
				SettingKeyInviteRewardsPageEnabled: raw,
				SettingKeyInviteBonusEnabled:       "true",
			}}, &config.Config{})
			public, err := svc.GetPublicSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, raw == "true", public.InviteRewardsPageEnabled)
			require.True(t, public.InviteBonusEnabled)
			payload, err := svc.GetPublicSettingsForInjection(ctx)
			require.NoError(t, err)
			injected := payload.(*PublicSettingsInjectionPayload)
			require.Equal(t, raw == "true", injected.InviteRewardsPageEnabled)
		})
	}
	for _, enabled := range []bool{true, false} {
		t.Run("save_"+strconv.FormatBool(enabled), func(t *testing.T) {
			repo := &bmUpdateRepoStub{}
			svc := NewSettingService(repo, &config.Config{})
			require.NoError(t, svc.UpdateSettings(ctx, &SystemSettings{
				InviteRewardsPageEnabled: enabled,
				InviteBonusEnabled:       true,
				SignupBonusEnabled:       true,
			}))
			require.Equal(t, strconv.FormatBool(enabled), repo.updates[SettingKeyInviteRewardsPageEnabled])
			require.Equal(t, "true", repo.updates[SettingKeyInviteBonusEnabled])
			require.Equal(t, "true", repo.updates[SettingKeySignupBonusEnabled])
		})
	}
}
