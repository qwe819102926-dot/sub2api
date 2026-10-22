package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetBonusBalanceCredit(t *testing.T) {
	for _, tc := range []struct {
		rate string
		want float64
	}{
		{"1", 5}, {"10", 0.5}, {"0.5", 10}, {" 2 ", 2.5},
		{"0", 5}, {"-1", 5}, {"NaN", 5}, {"+Inf", 5}, {"invalid", 5},
	} {
		t.Run(tc.rate, func(t *testing.T) {
			repo, mock := newRedeemAdjustmentRepoMock(t)
			mock.ExpectQuery(`(?s)SELECT COALESCE\(bonus_balance, 0\).*FROM users WHERE id = \$1 AND deleted_at IS NULL`).
				WithArgs(int64(1), service.SettingBonusBalanceConsumptionRate).
				WillReturnRows(sqlmock.NewRows([]string{"bonus", "rate"}).AddRow(5, tc.rate))
			credit, err := repo.GetBonusBalanceCredit(context.Background(), 1)
			require.NoError(t, err)
			require.Equal(t, tc.want, credit)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetBonusBalanceCreditFailsOnDatabaseError(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectQuery(`SELECT COALESCE`).WillReturnError(errors.New("database offline"))
	_, err := repo.GetBonusBalanceCredit(context.Background(), 1)
	require.ErrorContains(t, err, "database offline")
	require.NoError(t, mock.ExpectationsWereMet())
}
