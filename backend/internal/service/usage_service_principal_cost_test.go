package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type principalKeyUsageRepoStub struct {
	UsageLogRepository
	ids        []int64
	start, end time.Time
	err        error
}

func (r *principalKeyUsageRepoStub) GetBatchAPIKeyUsageStatsWithPrincipalCost(_ context.Context, ids []int64, start, end time.Time) (map[int64]*usagestats.BatchAPIKeyUsageStats, error) {
	r.ids, r.start, r.end = ids, start, end
	return map[int64]*usagestats.BatchAPIKeyUsageStats{7: {APIKeyID: 7}}, r.err
}

func TestUsageServiceBatchAPIKeyUsageUsesPrincipalCost(t *testing.T) {
	for _, repoErr := range []error{nil, errors.New("query failed")} {
		repo := &principalKeyUsageRepoStub{err: repoErr}
		svc := NewUsageService(repo, nil, nil, nil)
		start, end := time.Now().Add(-time.Hour), time.Now()
		stats, err := svc.GetBatchAPIKeyUsageStats(context.Background(), []int64{7}, start, end)
		require.Equal(t, []int64{7}, repo.ids)
		require.Equal(t, start, repo.start)
		require.Equal(t, end, repo.end)
		if repoErr != nil {
			require.ErrorIs(t, err, repoErr)
			continue
		}
		require.NoError(t, err)
		require.Zero(t, stats[7].TodayActualCost)
		require.Zero(t, stats[7].TotalActualCost)
	}
}
