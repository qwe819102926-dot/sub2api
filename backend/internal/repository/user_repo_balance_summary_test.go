package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSumUserBalances_OnlyIncludesNonDeletedUsersWithUserRole(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(balance\), 0\), COALESCE\(SUM\(COALESCE\(bonus_balance, 0\)\), 0\).*FROM users.*WHERE deleted_at IS NULL`).
		WithArgs(service.RoleUser).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "bonus_balance"}).AddRow(12.5, 3.25))

	summary, err := repo.SumUserBalances(context.Background())
	require.NoError(t, err)
	require.Equal(t, 12.5, summary.TotalBalance)
	require.Equal(t, 3.25, summary.TotalBonusBalance)
	require.NoError(t, mock.ExpectationsWereMet())
}
