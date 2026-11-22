package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestLiveLeaseReplacesRegularSlotsAndCountsTowardLimits(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	regular := NewConcurrencyCache(client, 15, 900)
	live, ok := regular.(service.LiveConcurrencyCache)
	require.True(t, ok)
	ctx := context.Background()

	accountAcquired, err := regular.AcquireAccountSlot(ctx, 10, 1, "regular-account")
	require.NoError(t, err)
	require.True(t, accountAcquired)
	userAcquired, err := regular.AcquireUserSlot(ctx, 20, 1, "regular-user")
	require.NoError(t, err)
	require.True(t, userAcquired)

	acquired, err := live.AcquireLiveLease(ctx, 10, 1, 20, 1, 30, "live-lease", true, service.LiveGroupConcurrency{})
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, regular.ReleaseAccountSlot(ctx, 10, "regular-account"))
	require.NoError(t, regular.ReleaseUserSlot(ctx, 20, "regular-user"))

	accountCount, err := regular.GetAccountConcurrency(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, accountCount)
	userCount, err := regular.GetUserConcurrency(ctx, 20)
	require.NoError(t, err)
	require.Equal(t, 1, userCount)
	accountAcquired, err = regular.AcquireAccountSlot(ctx, 10, 1, "ordinary-blocked")
	require.NoError(t, err)
	require.False(t, accountAcquired)

	refreshed, err := live.RefreshLiveLease(ctx, 10, 20, 30, 0, "live-lease")
	require.NoError(t, err)
	require.True(t, refreshed)
	require.NoError(t, live.ReleaseLiveLease(ctx, 10, 20, 30, 0, "live-lease"))
	accountAcquired, err = regular.AcquireAccountSlot(ctx, 10, 1, "ordinary-allowed")
	require.NoError(t, err)
	require.True(t, accountAcquired)
}

func TestLiveLeaseExpiresWithoutRefresh(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	regular := NewConcurrencyCache(client, 15, 900)
	live, ok := regular.(service.LiveConcurrencyCache)
	require.True(t, ok)
	ctx := context.Background()

	acquired, err := live.AcquireLiveLease(ctx, 10, 1, 20, 1, 30, "expired-live", false, service.LiveGroupConcurrency{})
	require.NoError(t, err)
	require.True(t, acquired)

	redisServer.FastForward(61 * time.Second)
	acquired, err = regular.AcquireAccountSlot(ctx, 10, 1, "ordinary-after-expiry")
	require.NoError(t, err)
	require.True(t, acquired)
	refreshed, err := live.RefreshLiveLease(ctx, 10, 20, 30, 0, "expired-live")
	require.NoError(t, err)
	require.False(t, refreshed)
}

func TestGroupUserConcurrencySharedWithLiveLeases(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewConcurrencyCache(client, 15, 900).(*concurrencyCache)
	ctx := context.Background()
	group := service.LiveGroupConcurrency{GroupID: 40, Limit: 1}

	acquired, err := cache.AcquireGroupUserSlot(ctx, 40, 20, 1, "regular")
	require.NoError(t, err)
	require.True(t, acquired)
	acquired, err = cache.AcquireLiveLease(ctx, 10, 0, 20, 0, 30, "live", true, group)
	require.NoError(t, err)
	require.False(t, acquired, "the live transition replaces global slots only")

	require.NoError(t, cache.ReleaseGroupUserSlot(ctx, 40, 20, "regular"))
	acquired, err = cache.AcquireLiveLease(ctx, 10, 0, 20, 0, 30, "live", true, group)
	require.NoError(t, err)
	require.True(t, acquired)

	server.FastForward(45 * time.Second)
	refreshed, err := cache.RefreshLiveLease(ctx, 10, 20, 30, group.GroupID, "live")
	require.NoError(t, err)
	require.True(t, refreshed)
	server.FastForward(30 * time.Second)
	acquired, err = cache.AcquireGroupUserSlot(ctx, 40, 20, 1, "regular")
	require.NoError(t, err)
	require.False(t, acquired)
	require.NoError(t, cache.ReleaseLiveLease(ctx, 10, 20, 30, group.GroupID, "live"))

	acquired, err = cache.AcquireGroupUserSlot(ctx, 40, 20, 1, "regular")
	require.NoError(t, err)
	require.True(t, acquired)
}
