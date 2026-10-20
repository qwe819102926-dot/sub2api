package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetUserRechargeLotteryHistory(t *testing.T) {
	columns := []string{"total", "winning_draws", "total_prize_amount", "id", "order_id", "prize_amount", "is_winner", "created_at"}
	createdAt := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		page    int
		rows    *sqlmock.Rows
		total   int64
		winners int64
		amount  float64
		count   int
	}{
		{
			name: "wins losses and administrator chances", page: 1,
			rows: sqlmock.NewRows(columns).
				AddRow(3, 2, 8.75, 13, nil, 7.5, true, createdAt).
				AddRow(3, 2, 8.75, 12, 42, 0, false, createdAt).
				AddRow(3, 2, 8.75, 11, 42, 1.25, true, createdAt),
			total: 3, winners: 2, amount: 8.75, count: 3,
		},
		{
			name: "no draws", page: 1,
			rows: sqlmock.NewRows(columns).AddRow(0, 0, 0, nil, nil, nil, nil, nil),
		},
		{
			name: "out of range page retains lifetime totals", page: 3,
			rows:  sqlmock.NewRows(columns).AddRow(21, 5, 12.5, nil, nil, nil, nil, nil),
			total: 21, winners: 5, amount: 12.5,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			service := newPaymentServiceForSQLMock(db)
			t.Cleanup(func() { _ = service.entClient.Close() })
			mock.ExpectQuery(`(?s)WITH summary AS.*FROM recharge_lottery_draws WHERE user_id = \$1.*LEFT JOIN.*FROM recharge_lottery_draws WHERE user_id = \$1.*ORDER BY created_at DESC, id DESC LIMIT \$2 OFFSET \$3`).
				WithArgs(int64(7), 20, int64((tc.page-1)*20)).WillReturnRows(tc.rows)
			history, err := service.GetUserRechargeLotteryHistory(context.Background(), 7, tc.page, 20)
			require.NoError(t, err)
			require.NotNil(t, history.Items)
			require.Len(t, history.Items, tc.count)
			require.Equal(t, tc.total, history.Total)
			require.Equal(t, tc.winners, history.WinningDraws)
			require.Equal(t, tc.amount, history.TotalPrizeAmount)
			require.Equal(t, (tc.total+19)/20, history.TotalPages)
			if tc.count > 0 {
				require.Nil(t, history.Items[0].OrderID)
				require.Equal(t, int64(42), *history.Items[1].OrderID)
				require.False(t, history.Items[1].IsWinner)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetUserRechargeLotteryHistoryReturnsReadErrors(t *testing.T) {
	for _, iteration := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		service := newPaymentServiceForSQLMock(db)
		t.Cleanup(func() { _ = service.entClient.Close() })
		expectation := mock.ExpectQuery(`(?s)WITH summary AS`).WithArgs(int64(7), 20, int64(0))
		if iteration {
			expectation.WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1).RowError(0, errors.New("read failed")))
		} else {
			expectation.WillReturnError(errors.New("read failed"))
		}
		_, err = service.GetUserRechargeLotteryHistory(context.Background(), 7, 1, 20)
		require.ErrorContains(t, err, "read failed")
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
