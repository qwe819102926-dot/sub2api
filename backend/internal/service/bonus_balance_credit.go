package service

import "context"

// BonusBalanceCreditStore returns the billable USD covered by the bonus wallet,
// after applying its independently configured consumption multiplier.
type BonusBalanceCreditStore interface {
	GetBonusBalanceCredit(context.Context, int64) (float64, error)
}

func loadBonusBalanceCredit(ctx context.Context, repo UserRepository, userID int64) (float64, error) {
	if store, ok := repo.(BonusBalanceCreditStore); ok {
		return store.GetBonusBalanceCredit(ctx, userID)
	}
	// Compatibility for stores without a separate consumption multiplier.
	if store, ok := repo.(BonusBalanceStore); ok {
		balances, err := store.GetBonusBalancesByUserIDs(ctx, []int64{userID})
		return balances[userID], err
	}
	return 0, nil
}

// CheckBalanceForAuth keeps bonus balances out of the auth snapshot: grants,
// deductions and the one-time conversion take effect without cache expiry.
// MinimumBalanceReserve remains a billing precheck, not an auth threshold.
func (s *APIKeyService) CheckBalanceForAuth(ctx context.Context, user *User) error {
	if user == nil {
		return ErrInsufficientBalance
	}
	if user.Balance > 0 {
		return nil
	}
	bonus, err := loadBonusBalanceCredit(ctx, s.userRepo, user.ID)
	if err != nil {
		return ErrBillingServiceUnavailable.WithCause(err)
	}
	if user.Balance+bonus > 0 {
		return nil
	}
	return ErrInsufficientBalance
}

func (s *BillingCacheService) getSpendableBalance(ctx context.Context, userID int64) (float64, error) {
	principal, err := s.GetUserBalance(ctx, userID)
	if err != nil {
		return 0, err
	}
	bonus, err := loadBonusBalanceCredit(ctx, s.userRepo, userID)
	if err != nil {
		return 0, err
	}
	return principal + bonus, nil
}
