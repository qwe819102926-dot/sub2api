package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRedeemHistoryPaginationOrderingAndIsolation(t *testing.T) {
	_, client := newAPIKeyRepoSQLite(t)
	repo := NewRedeemCodeRepository(client)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "history@test.com")
	other := mustCreateAPIKeyRepoUser(t, ctx, client, "other-history@test.com")
	empty := mustCreateAPIKeyRepoUser(t, ctx, client, "empty-history@test.com")
	group, err := client.Group.Create().SetName("History group").Save(ctx)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	types := []string{service.RedeemTypeBalance, service.RedeemTypeConcurrency, service.RedeemTypeSubscription, "admin_balance", "admin_concurrency", "admin_bonus"}
	var ids []int64
	for i := 0; i < 105; i++ {
		usedAt := now
		if i == 0 {
			usedAt = now.Add(time.Hour)
		}
		code, err := client.RedeemCode.Create().SetCode(fmt.Sprintf("HISTORY-%d", i)).
			SetType(types[i%len(types)]).SetValue(10).SetStatus(service.StatusUsed).
			SetUsedBy(user.ID).SetUsedAt(usedAt).SetGroupID(group.ID).Save(ctx)
		require.NoError(t, err)
		ids = append(ids, code.ID)
	}
	_, err = client.RedeemCode.Create().SetCode("OTHER-HISTORY").SetType(service.RedeemTypeBalance).
		SetStatus(service.StatusUsed).SetUsedBy(other.ID).SetUsedAt(now.Add(2 * time.Hour)).Save(ctx)
	require.NoError(t, err)
	expected := []int64{ids[0]}
	for i := len(ids) - 1; i > 0; i-- {
		expected = append(expected, ids[i])
	}
	for _, size := range []int{20, 50, 100} {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			var got []int64
			seenTypes := map[string]bool{}
			for page := 1; page <= (len(ids)+size-1)/size+1; page++ {
				codes, result, err := repo.ListByUserPaginated(ctx, user.ID, pagination.PaginationParams{Page: page, PageSize: size}, "")
				require.NoError(t, err)
				require.EqualValues(t, len(ids), result.Total)
				require.LessOrEqual(t, len(codes), size)
				for _, code := range codes {
					require.NotNil(t, code.UsedBy)
					require.Equal(t, user.ID, *code.UsedBy)
					require.NotNil(t, code.Group)
					require.Equal(t, group.Name, code.Group.Name)
					got = append(got, code.ID)
					seenTypes[code.Type] = true
				}
			}
			require.Equal(t, expected, got)
			for _, codeType := range types {
				require.True(t, seenTypes[codeType], "missing history type %s", codeType)
			}
		})
	}
	legacy, err := repo.ListByUser(ctx, user.ID, 25)
	require.NoError(t, err)
	var legacyIDs []int64
	for _, code := range legacy {
		legacyIDs = append(legacyIDs, code.ID)
	}
	require.Equal(t, expected[:25], legacyIDs)
	codes, result, err := repo.ListByUserPaginated(ctx, empty.ID, pagination.PaginationParams{Page: 1, PageSize: 20}, "")
	require.NoError(t, err)
	require.Empty(t, codes)
	require.Zero(t, result.Total)
}
