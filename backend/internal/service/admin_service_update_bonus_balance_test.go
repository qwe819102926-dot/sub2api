//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type bonusBalanceUserRepoStub struct {
	*userRepoStub
	adjustErr   error
	current     float64
	bonusByUser map[int64]float64
	bonusErr    error
	listUsers   []User
	changes     []BalanceChange
}

func (s *bonusBalanceUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	user, err := s.userRepoStub.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	clone := *user
	clone.BonusBalance = 0
	return &clone, nil
}

func (s *bonusBalanceUserRepoStub) GetBonusBalancesByUserIDs(_ context.Context, userIDs []int64) (map[int64]float64, error) {
	if s.bonusErr != nil {
		return nil, s.bonusErr
	}
	out := make(map[int64]float64, len(userIDs))
	for _, id := range userIDs {
		if s.bonusByUser != nil {
			out[id] = s.bonusByUser[id]
			continue
		}
		out[id] = s.current
	}
	return out, nil
}

func (s *bonusBalanceUserRepoStub) ListWithFilters(_ context.Context, params pagination.PaginationParams, _ UserListFilters) ([]User, *pagination.PaginationResult, error) {
	out := make([]User, len(s.listUsers))
	copy(out, s.listUsers)
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: params.Page, PageSize: params.PageSize}, nil
}

func (s *bonusBalanceUserRepoStub) GetLatestUsedAtByUserIDs(_ context.Context, _ []int64) (map[int64]*time.Time, error) {
	return map[int64]*time.Time{}, nil
}

func (s *bonusBalanceUserRepoStub) GetLatestUsedAtByUserID(_ context.Context, _ int64) (*time.Time, error) {
	return nil, nil
}

func (s *bonusBalanceUserRepoStub) AdjustBonusBalance(ctx context.Context, id int64, delta float64) (BalanceChange, error) {
	return s.apply(func(current float64) float64 { return current + delta })
}

func (s *bonusBalanceUserRepoStub) SetBonusBalance(ctx context.Context, id int64, value float64) (BalanceChange, error) {
	return s.apply(func(float64) float64 { return value })
}

func (s *bonusBalanceUserRepoStub) apply(next func(current float64) float64) (BalanceChange, error) {
	if s.adjustErr != nil {
		return BalanceChange{}, s.adjustErr
	}
	if s.userRepoStub == nil || s.userRepoStub.user == nil {
		return BalanceChange{}, ErrUserNotFound
	}
	change := BalanceChange{Old: s.current}
	change.New = next(change.Old)
	if change.New < 0 {
		return change, ErrBalanceNegative
	}
	s.current = change.New
	s.changes = append(s.changes, change)
	return change, nil
}

func TestAdminService_UpdateUserBonusBalance_UsesAtomicPrimitives(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		amount    float64
		want      BalanceChange
	}{
		{name: "add", operation: "add", amount: 5, want: BalanceChange{Old: 10, New: 15}},
		{name: "subtract", operation: "subtract", amount: 4, want: BalanceChange{Old: 10, New: 6}},
		{name: "set", operation: "set", amount: 2, want: BalanceChange{Old: 10, New: 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &bonusBalanceUserRepoStub{
				userRepoStub: &userRepoStub{user: &User{ID: 7, Balance: 99}},
				current:      10,
			}
			redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
			svc := &adminServiceImpl{
				userRepo:       repo,
				redeemCodeRepo: redeemRepo,
			}

			user, err := svc.UpdateUserBonusBalance(context.Background(), 7, tt.amount, tt.operation, "manual")
			require.NoError(t, err)
			require.Equal(t, []BalanceChange{tt.want}, repo.changes)
			require.Equal(t, tt.want.New, user.BonusBalance)
			require.True(t, user.BonusBalanceKnown)
			require.Equal(t, 0.0, repo.userRepoStub.user.BonusBalance, "Ent snapshot must not be treated as the source of bonus_balance")
			require.Len(t, redeemRepo.created, 1)
			require.Equal(t, AdjustmentTypeAdminBonusBalance, redeemRepo.created[0].Type)
			require.Equal(t, tt.want.New-tt.want.Old, redeemRepo.created[0].Value)
			require.Equal(t, "manual", redeemRepo.created[0].Notes)
		})
	}
}

func TestAdminService_UpdateUserBonusBalance_RejectsNegativeResult(t *testing.T) {
	repo := &bonusBalanceUserRepoStub{
		userRepoStub: &userRepoStub{user: &User{ID: 7}},
		current:      3,
	}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}},
	}

	_, err := svc.UpdateUserBonusBalance(context.Background(), 7, 4, "subtract", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "bonus balance cannot be negative")
	require.Empty(t, repo.changes)
	require.Equal(t, 3.0, repo.current)
}

func TestAdminService_UpdateUserBonusBalance_RejectsUnknownOperation(t *testing.T) {
	repo := &bonusBalanceUserRepoStub{
		userRepoStub: &userRepoStub{user: &User{ID: 7}},
		current:      10,
	}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}},
	}

	_, err := svc.UpdateUserBonusBalance(context.Background(), 7, 1, "multiply", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported bonus balance operation")
	require.Empty(t, repo.changes)
}

func TestAdminService_UpdateUserBonusBalance_NoChangeNoRecord(t *testing.T) {
	repo := &bonusBalanceUserRepoStub{
		userRepoStub: &userRepoStub{user: &User{ID: 7}},
		current:      10,
	}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: redeemRepo,
	}

	user, err := svc.UpdateUserBonusBalance(context.Background(), 7, 10, "set", "")
	require.NoError(t, err)
	require.Equal(t, 10.0, user.BonusBalance)
	require.True(t, user.BonusBalanceKnown)
	require.Empty(t, redeemRepo.created)
}

func TestAdminService_UpdateUserBonusBalance_RequiresStore(t *testing.T) {
	svc := &adminServiceImpl{
		userRepo:       &userRepoStub{user: &User{ID: 7}},
		redeemCodeRepo: &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}},
	}

	_, err := svc.UpdateUserBonusBalance(context.Background(), 7, 1, "add", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "bonus balance store is not configured")
}

func TestAdminService_ListUsers_AttachesBonusBalances(t *testing.T) {
	repo := &bonusBalanceUserRepoStub{
		userRepoStub: &userRepoStub{},
		listUsers: []User{
			{ID: 101, Email: "a@example.com"},
			{ID: 202, Email: "b@example.com"},
		},
		bonusByUser: map[int64]float64{101: 1.25, 202: 0},
	}
	svc := &adminServiceImpl{userRepo: repo}

	users, total, err := svc.ListUsers(context.Background(), 1, 20, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, users, 2)
	require.True(t, users[0].BonusBalanceKnown)
	require.Equal(t, 1.25, users[0].BonusBalance)
	require.True(t, users[1].BonusBalanceKnown)
	require.Equal(t, 0.0, users[1].BonusBalance)
}

func TestAdminService_ListUsers_BonusBalanceLoadFailureLeavesUnknown(t *testing.T) {
	repo := &bonusBalanceUserRepoStub{
		userRepoStub: &userRepoStub{},
		listUsers:    []User{{ID: 101, Email: "a@example.com", BonusBalance: 9}},
		bonusErr:     errors.New("db unavailable"),
	}
	svc := &adminServiceImpl{userRepo: repo}

	users, total, err := svc.ListUsers(context.Background(), 1, 20, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	require.False(t, users[0].BonusBalanceKnown)
	require.Equal(t, 9.0, users[0].BonusBalance)
}

func TestAdminService_GetUser_AttachesBonusBalance(t *testing.T) {
	repo := &bonusBalanceUserRepoStub{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Email: "u@example.com"}},
		current:      3.5,
	}
	svc := &adminServiceImpl{userRepo: repo}

	user, err := svc.GetUser(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, user.BonusBalanceKnown)
	require.Equal(t, 3.5, user.BonusBalance)
}

func TestAdminService_GetUser_BonusBalanceLoadFailureLeavesUnknown(t *testing.T) {
	repo := &bonusBalanceUserRepoStub{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Email: "u@example.com", BonusBalance: 4}},
		bonusErr:     errors.New("db unavailable"),
	}
	svc := &adminServiceImpl{userRepo: repo}

	user, err := svc.GetUser(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, user.BonusBalanceKnown)
	require.Equal(t, 0.0, user.BonusBalance)
}
