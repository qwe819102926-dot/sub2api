package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type balanceSummaryRepoStub struct {
	UserRepository
	summary UserBalanceSummary
	err     error
}

func (s *balanceSummaryRepoStub) SumUserBalances(context.Context) (UserBalanceSummary, error) {
	return s.summary, s.err
}

func TestAdminService_GetUserBalanceSummary(t *testing.T) {
	repo := &balanceSummaryRepoStub{summary: UserBalanceSummary{TotalBalance: 10, TotalBonusBalance: 2}}
	svc := &adminServiceImpl{userRepo: repo}

	summary, err := svc.GetUserBalanceSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, 10.0, summary.TotalBalance)
	require.Equal(t, 2.0, summary.TotalBonusBalance)
}

func TestAdminService_GetUserBalanceSummary_QueryError(t *testing.T) {
	repo := &balanceSummaryRepoStub{err: errors.New("db down")}
	svc := &adminServiceImpl{userRepo: repo}

	summary, err := svc.GetUserBalanceSummary(context.Background())
	require.Nil(t, summary)
	require.ErrorContains(t, err, "db down")
}

func TestAdminService_GetUserBalanceSummary_StoreMissing(t *testing.T) {
	svc := &adminServiceImpl{userRepo: &balanceSummaryMissingRepo{}}

	summary, err := svc.GetUserBalanceSummary(context.Background())
	require.Nil(t, summary)
	require.ErrorContains(t, err, "unavailable")
}

type balanceSummaryMissingRepo struct {
	UserRepository
}
