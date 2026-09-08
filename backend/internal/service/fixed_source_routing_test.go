package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type fixedRoutingSettingRepo struct {
	values map[string]string
}

func (r *fixedRoutingSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *fixedRoutingSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (r *fixedRoutingSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}
func (r *fixedRoutingSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (r *fixedRoutingSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (r *fixedRoutingSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}
func (r *fixedRoutingSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestFixedSourceRoutingSettingsNormalizeAndRoundTrip(t *testing.T) {
	repo := &fixedRoutingSettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	input := &FixedSourceRoutingSettings{
		Enabled: true,
		Domains: []string{" WWW.Example.COM. ", "example.com"},
		IPs:     []string{" 192.0.2.10 ", "2001:db8::/32"},
		Routes:  []FixedSourceRoute{{SourceGroupID: 1, TargetGroupID: 2, AccountID: 3}},
	}

	require.NoError(t, svc.SetFixedSourceRoutingSettings(context.Background(), input))
	got, err := svc.GetFixedSourceRoutingSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"example.com"}, got.Domains)
	require.Equal(t, []string{"192.0.2.10", "2001:db8::/32"}, got.IPs)
	require.Equal(t, input.Routes, got.Routes)
}

func TestFixedSourceRoutingSettingsRejectInvalidSourcesAndDuplicateRoutes(t *testing.T) {
	svc := NewSettingService(&fixedRoutingSettingRepo{values: map[string]string{}}, &config.Config{})

	err := svc.SetFixedSourceRoutingSettings(context.Background(), &FixedSourceRoutingSettings{Domains: []string{"https://example.com"}})
	require.ErrorContains(t, err, "invalid fixed source domain")
	err = svc.SetFixedSourceRoutingSettings(context.Background(), &FixedSourceRoutingSettings{IPs: []string{"not-an-ip"}})
	require.ErrorContains(t, err, "invalid fixed source IP or CIDR")
	err = svc.SetFixedSourceRoutingSettings(context.Background(), &FixedSourceRoutingSettings{Routes: []FixedSourceRoute{
		{SourceGroupID: 1, TargetGroupID: 2, AccountID: 3},
		{SourceGroupID: 1, TargetGroupID: 4, AccountID: 5},
	}})
	require.ErrorContains(t, err, "more than one fixed route")
}

type fixedRoutingAccountRepo struct {
	AccountRepository
	accounts map[int64]*Account
}

func (r *fixedRoutingAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	account := r.accounts[id]
	if account == nil {
		return nil, errors.New("not found")
	}
	return account, nil
}

func TestGatewayFixedRouteDoesNotFallback(t *testing.T) {
	groupID := int64(20)
	repo := &fixedRoutingAccountRepo{accounts: map[int64]*Account{
		30: {ID: 30, Platform: PlatformAnthropic, Status: StatusDisabled, Schedulable: false, GroupIDs: []int64{groupID}},
		31: {ID: 31, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, GroupIDs: []int64{groupID}},
	}}
	svc := &GatewayService{accountRepo: repo}
	ctx := context.WithValue(context.Background(), ctxkey.FixedRouteAccountID, int64(30))

	account, err := svc.SelectAccountForModelWithExclusions(ctx, &groupID, "", "claude-sonnet-4", nil)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, account)
}

func TestGatewayFixedRouteSelectsPinnedClaudeAccount(t *testing.T) {
	groupID := int64(20)
	repo := &fixedRoutingAccountRepo{accounts: map[int64]*Account{
		30: {
			ID: 30, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
			Status: StatusActive, Schedulable: true,
			GroupIDs: []int64{groupID}, AccountGroups: []AccountGroup{{GroupID: groupID}},
		},
	}}
	svc := &GatewayService{accountRepo: repo}
	ctx := context.WithValue(context.Background(), ctxkey.FixedRouteAccountID, int64(30))

	account, err := svc.SelectAccountForModelWithExclusions(ctx, &groupID, "", "claude-sonnet-4", nil)
	require.NoError(t, err)
	require.Equal(t, int64(30), account.ID)
}

func TestOpenAIFixedRouteDoesNotFallback(t *testing.T) {
	groupID := int64(20)
	repo := &fixedRoutingAccountRepo{accounts: map[int64]*Account{
		30: {ID: 30, Platform: PlatformOpenAI, Status: StatusDisabled, Schedulable: false, GroupIDs: []int64{groupID}},
		31: {ID: 31, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, GroupIDs: []int64{groupID}},
	}}
	svc := &OpenAIGatewayService{accountRepo: repo}
	ctx := context.WithValue(context.Background(), ctxkey.FixedRouteAccountID, int64(30))

	account, err := svc.SelectAccountForModelWithExclusions(ctx, &groupID, "", "gpt-5", nil)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, account)
}

func TestOpenAIFixedRouteSelectsPinnedAccountForTokenCount(t *testing.T) {
	groupID := int64(20)
	repo := &fixedRoutingAccountRepo{accounts: map[int64]*Account{
		30: {
			ID: 30, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Status: StatusActive, Schedulable: true,
			GroupIDs: []int64{groupID}, AccountGroups: []AccountGroup{{GroupID: groupID}},
		},
	}}
	svc := &OpenAIGatewayService{accountRepo: repo}
	ctx := context.WithValue(context.Background(), ctxkey.FixedRouteAccountID, int64(30))

	account, err := svc.SelectAccountForTokenCount(ctx, &groupID, "", "gpt-5", "", PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, int64(30), account.ID)
}
