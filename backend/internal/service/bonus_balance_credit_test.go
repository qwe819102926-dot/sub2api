//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type billingBonusCreditRepo struct {
	UserRepository
	credit float64
	err    error
}

func (r *billingBonusCreditRepo) GetBonusBalanceCredit(context.Context, int64) (float64, error) {
	return r.credit, r.err
}

func TestBonusBalanceCreditEligibility(t *testing.T) {
	for _, tc := range []struct {
		name   string
		credit float64
		err    error
		want   error
	}{
		{"bonus_only", 0.5, nil, nil},
		{"empty", 0, nil, ErrInsufficientBalance},
		{"below_reserve", 0.005, nil, ErrInsufficientBalance},
		{"read_failure", 0, errors.New("db unavailable"), ErrBillingServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := newMemInflightCache(0)
			repo := &billingBonusCreditRepo{credit: tc.credit, err: tc.err}
			cfg := &config.Config{}
			cfg.Billing.MinimumBalanceReserve = 0.01
			svc := NewBillingCacheService(cache, repo, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(svc.Stop)
			err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
			if tc.want == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}

func TestBonusBalanceCreditConcurrentReservations(t *testing.T) {
	cache := newMemInflightCache(0)
	svc := newInflightSvc(t, cache, 60)
	svc.userRepo = &billingBonusCreditRepo{credit: 1}
	user := &User{ID: 1}
	ctx := context.Background()
	first, err := svc.ReserveInflight(ctx, user, nil, nil, 0.4)
	require.NoError(t, err)
	defer first.Release()
	second, err := svc.ReserveInflight(ctx, user, nil, nil, 0.4)
	require.NoError(t, err, "bonus-only wallets must support concurrent requests")
	defer second.Release()
	_, err = svc.ReserveInflight(ctx, user, nil, nil, 0.4)
	require.ErrorIs(t, err, ErrInsufficientBalance, "pending requests still cannot overcommit bonus credit")
}

func TestBonusBalanceCreditFixedCharge(t *testing.T) {
	cache := newMemInflightCache(0)
	svc := newInflightSvc(t, cache, 60)
	svc.userRepo = &billingBonusCreditRepo{credit: 0.5}
	ctx := context.Background()
	res, err := svc.ReserveFixedCharge(ctx, &User{ID: 1}, nil, nil, nil, 0.4)
	require.NoError(t, err)
	defer res.Release()
	_, err = svc.ReserveFixedCharge(ctx, &User{ID: 1}, nil, nil, nil, 0.2)
	require.ErrorIs(t, err, ErrInsufficientBalance, "fixed charges must count other in-flight requests")
	_, err = svc.ReserveFixedCharge(ctx, &User{ID: 1}, nil, nil, nil, 0.6)
	require.ErrorIs(t, err, ErrInsufficientBalance, "a fixed charge cannot exceed available bonus credit")
}
