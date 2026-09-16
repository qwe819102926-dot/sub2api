//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type usageBillingApplyStub struct {
	UsageBillingRepository

	result *UsageBillingApplyResult
	last   *UsageBillingCommand
}

func (s *usageBillingApplyStub) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	s.last = cmd
	if s.result != nil {
		return s.result, nil
	}
	return &UsageBillingApplyResult{Applied: true}, nil
}

func TestApplyUsageBilling_KeepsBilledActualCostAndRecordsPrincipalWalletCost(t *testing.T) {
	usageLog := &UsageLog{
		ActualCost: 1.25,
		TotalCost:  1.25,
	}
	repo := &usageBillingApplyStub{
		result: &UsageBillingApplyResult{
			Applied:           true,
			BalanceDeducted:   2.50,
			PrincipalDeducted: 0,
		},
	}

	applied, err := applyUsageBilling(context.Background(), "req-bonus-rate", usageLog, &postUsageBillingParams{
		Cost:    &CostBreakdown{ActualCost: 1.25, TotalCost: 1.25},
		User:    &User{ID: 11},
		APIKey:  &APIKey{ID: 22},
		Account: &Account{ID: 33},
	}, &billingDeps{
		deferredService: NewDeferredService(nil, nil, time.Second),
	}, repo)

	require.NoError(t, err)
	require.True(t, applied)
	require.NotNil(t, repo.last)
	require.InDelta(t, 1.25, repo.last.BalanceCost, 1e-12)
	require.InDelta(t, 1.25, usageLog.ActualCost, 1e-12)
	require.InDelta(t, 1.25, usageLog.TotalCost, 1e-12)
	require.NotNil(t, usageLog.WalletCost)
	require.InDelta(t, 0, *usageLog.WalletCost, 1e-12)
}

func TestApplyUsageBilling_WalletCostUsesPrincipalWhenBonusAndBalanceMix(t *testing.T) {
	usageLog := &UsageLog{ActualCost: 1.25, TotalCost: 1.25}
	repo := &usageBillingApplyStub{
		result: &UsageBillingApplyResult{
			Applied:           true,
			BalanceDeducted:   2.00,
			PrincipalDeducted: 0.75,
		},
	}

	applied, err := applyUsageBilling(context.Background(), "req-mixed-wallet", usageLog, &postUsageBillingParams{
		Cost:    &CostBreakdown{ActualCost: 1.25, TotalCost: 1.25},
		User:    &User{ID: 11},
		APIKey:  &APIKey{ID: 22},
		Account: &Account{ID: 33},
	}, &billingDeps{
		deferredService: NewDeferredService(nil, nil, time.Second),
	}, repo)

	require.NoError(t, err)
	require.True(t, applied)
	require.InDelta(t, 1.25, usageLog.ActualCost, 1e-12)
	require.NotNil(t, usageLog.WalletCost)
	require.InDelta(t, 0.75, *usageLog.WalletCost, 1e-12)
}

func TestApplyUsageBilling_SubscriptionKeepsBilledAmountAsWalletCost(t *testing.T) {
	usageLog := &UsageLog{ActualCost: 1.25, TotalCost: 1.25}
	repo := &usageBillingApplyStub{
		result: &UsageBillingApplyResult{
			Applied:           true,
			BalanceDeducted:   0,
			PrincipalDeducted: 0,
		},
	}

	applied, err := applyUsageBilling(context.Background(), "req-sub-wallet", usageLog, &postUsageBillingParams{
		Cost:               &CostBreakdown{ActualCost: 1.25, TotalCost: 1.25},
		User:               &User{ID: 11},
		APIKey:             &APIKey{ID: 22},
		Account:            &Account{ID: 33},
		IsSubscriptionBill: true,
	}, &billingDeps{
		deferredService: NewDeferredService(nil, nil, time.Second),
	}, repo)

	require.NoError(t, err)
	require.True(t, applied)
	require.InDelta(t, 1.25, usageLog.ActualCost, 1e-12)
	require.NotNil(t, usageLog.WalletCost)
	require.InDelta(t, 1.25, *usageLog.WalletCost, 1e-12)
}
