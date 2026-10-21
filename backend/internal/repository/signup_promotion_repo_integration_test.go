//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSignupPromotion_RegistrationAndOAuthFinalization(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	store := NewSettingRepository(client)
	require.NoError(t, store.SetMultiple(ctx, map[string]string{
		"registration_enabled": "true", "email_verify_enabled": "false", "invitation_code_enabled": "false", "registration_email_domain_quota_enabled": "false", "default_balance": "999", "default_concurrency": "2",
		"affiliate_enabled": "false", "signup_bonus_enabled": "true", "signup_bonus_amount": "50", "invite_bonus_enabled": "true", "invite_bonus_amount": "50",
	}))
	cfg := &config.Config{}
	cfg.JWT.Secret = "local-signup-promotion-test-signing-key"
	settings := service.NewSettingService(store, cfg)
	repo := &affiliateRepository{client: client}
	affiliate := service.NewAffiliateService(repo, settings, nil, nil)
	users := NewUserRepository(client, integrationDB)
	auth := service.NewAuthService(client, users, NewRedeemCodeRepository(client), nil, cfg, settings, nil, nil, nil, nil, nil, affiliate, nil)
	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("registration-owner-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
	summary, err := repo.EnsureUserAffiliate(ctx, inviter.ID)
	require.NoError(t, err)
	email := fmt.Sprintf("registration-gift-%d@example.com", time.Now().UnixNano())
	_, user, err := auth.RegisterWithVerification(ctx, email, "testing-password", "", "", "", summary.AffCode)
	require.NoError(t, err)
	require.Zero(t, user.Balance)
	require.Equal(t, float64(50), querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, user.ID))
	_, _, err = auth.RegisterWithVerification(ctx, email, "testing-password", "", "", "", summary.AffCode)
	require.Error(t, err)
	for _, id := range []int64{inviter.ID, user.ID} {
		require.Equal(t, float64(50), querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, id))
	}
	oauth := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("oauth-gift-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
	for i := 0; i < 2; i++ {
		require.NoError(t, auth.FinalizeOAuthEmailAccount(ctx, oauth, "", "github", summary.AffCode))
	}
	require.Equal(t, float64(50), querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, oauth.ID))
	require.Equal(t, float64(100), querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, inviter.ID))
	// A local-email allowlist must not block verified provider identities.
	require.NoError(t, store.Set(ctx, "registration_email_suffix_whitelist", `["@qq.com"]`))
	_, synthetic, err := auth.LoginOrRegisterOAuth(ctx, fmt.Sprintf("promotion-%d%s", time.Now().UnixNano(), service.LinuxDoConnectSyntheticEmailDomain), "provider user")
	require.NoError(t, err)
	require.Zero(t, synthetic.Balance)
	require.Equal(t, float64(50), querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, synthetic.ID))
}

func TestSignupPromotion_ConcurrentDuplicateDelivery(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("promotion-concurrent-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
	repo := &affiliateRepository{client: client}
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.ApplySignupPromotion(ctx, u.ID, "", service.SignupPromotionConfig{SignupBonusEnabled: true, SignupBonusAmount: 50})
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, float64(50), querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, u.ID))
	require.Equal(t, 1, querySingleInt(t, ctx, client, `SELECT COUNT(*) FROM signup_promotion_grants WHERE source_user_id=$1`, u.ID))
}

func TestSignupPromotion_AtomicIdempotentBonusOnly(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	ctx = dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := &affiliateRepository{client: integrationEntClient}
	create := func(label string) int64 {
		return mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("promotion-%s-%d@example.com", label, time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Balance: 7, Concurrency: 1}).ID
	}
	inviter, invitee := create("inviter"), create("invitee")
	summary, err := repo.EnsureUserAffiliate(ctx, inviter)
	require.NoError(t, err)
	cfg := service.SignupPromotionConfig{SignupBonusEnabled: true, SignupBonusAmount: 50, InviteBonusEnabled: true, InviteBonusAmount: 50}
	for i := 0; i < 3; i++ {
		_, err := repo.ApplySignupPromotion(ctx, invitee, summary.AffCode, cfg)
		require.NoError(t, err)
	}
	for _, id := range []int64{inviter, invitee} {
		require.Equal(t, float64(50), querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, id))
		require.Equal(t, float64(7), querySingleFloat(t, ctx, client, `SELECT balance::double precision FROM users WHERE id=$1`, id))
		require.Zero(t, querySingleFloat(t, ctx, client, `SELECT total_recharged::double precision FROM users WHERE id=$1`, id))
	}
	require.Equal(t, 2, querySingleInt(t, ctx, client, `SELECT COUNT(*) FROM signup_promotion_grants WHERE source_user_id=$1`, invitee))
	_, err = client.ExecContext(ctx, `UPDATE users SET bonus_balance=2 WHERE id=$1`, inviter)
	require.NoError(t, err)
	detail, err := repo.GetSignupPromotionDetail(ctx, inviter, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), detail.InvitedCount)
	require.Equal(t, float64(50), detail.TotalBonus)
	require.Equal(t, "credited", detail.Records[0].Status)
	page2, err := repo.GetSignupPromotionDetail(ctx, inviter, 2, 1)
	require.NoError(t, err)
	require.Empty(t, page2.Records)
	require.Equal(t, float64(50), page2.TotalBonus)
}

func TestSignupPromotion_InvalidSelfAndDisabledInvites(t *testing.T) {
	for _, kind := range []string{"invalid", "self", "disabled"} {
		t.Run(kind, func(t *testing.T) {
			tx := testEntTx(t)
			ctx := dbent.NewTxContext(context.Background(), tx)
			client := tx.Client()
			repo := &affiliateRepository{client: integrationEntClient}
			u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("promotion-%s-%d@example.com", kind, time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
			summary, err := repo.EnsureUserAffiliate(ctx, u.ID)
			require.NoError(t, err)
			code := "NOTREALCODE"
			if kind == "self" {
				code = summary.AffCode
			}
			_, err = repo.ApplySignupPromotion(ctx, u.ID, code, service.SignupPromotionConfig{SignupBonusEnabled: true, SignupBonusAmount: 50, InviteBonusEnabled: kind != "disabled", InviteBonusAmount: 50})
			require.NoError(t, err)
			require.Equal(t, float64(50), querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, u.ID))
			require.Equal(t, 1, querySingleInt(t, ctx, client, `SELECT COUNT(*) FROM signup_promotion_grants WHERE source_user_id=$1`, u.ID))
		})
	}
}

func TestSignupPromotion_OuterRollbackRestoresBothBalances(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	repo := &affiliateRepository{client: integrationEntClient}
	u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("promotion-rollback-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("promotion-rollback-owner-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1})
	summary, err := repo.EnsureUserAffiliate(ctx, inviter.ID)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `SAVEPOINT before_promotion`)
	require.NoError(t, err)
	_, err = repo.ApplySignupPromotion(ctx, u.ID, summary.AffCode, service.SignupPromotionConfig{SignupBonusEnabled: true, SignupBonusAmount: 50, InviteBonusEnabled: true, InviteBonusAmount: 50})
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `ROLLBACK TO SAVEPOINT before_promotion`)
	require.NoError(t, err)
	require.Zero(t, querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, u.ID))
	require.Zero(t, querySingleFloat(t, ctx, client, `SELECT bonus_balance::double precision FROM users WHERE id=$1`, inviter.ID))
	require.Zero(t, querySingleInt(t, ctx, client, `SELECT COUNT(*) FROM signup_promotions WHERE user_id=$1`, u.ID))
}
