package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestGroupUserConcurrencySharesSlotsAcrossKeysAndTransports(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := repository.NewConcurrencyCache(client, 15, 900)
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	group := &service.Group{ID: 10, UserConcurrencyLimit: 1}
	firstKey := &service.APIKey{ID: 101, UserID: 7, Group: group}
	secondKey := &service.APIKey{ID: 102, UserID: 7, Group: group}
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	c.Request = c.Request.WithContext(ctx)
	c.Set(string(middleware.ContextKeyAPIKey), firstKey)
	var streamStarted bool
	release, err := helper.AcquireUserSlotWithWait(c, 7, 3, false, &streamStarted)
	require.NoError(t, err)
	release = wrapReleaseOnDone(ctx, release)
	defer release()

	otherRequest, _ := newHelperTestContext(http.MethodPost, "/v1/responses")
	otherRequest.Set(string(middleware.ContextKeyAPIKey), secondKey)
	blockedRelease, err := helper.AcquireUserSlotWithWait(otherRequest, 7, 3, false, &streamStarted)
	require.Nil(t, blockedRelease)
	var limitErr *ConcurrencyError
	require.ErrorAs(t, err, &limitErr)
	require.Equal(t, groupUserConcurrencySlot, limitErr.SlotType)
	status, _, _, _ := concurrencyErrorResponse(err, "user")
	require.Equal(t, http.StatusTooManyRequests, status)

	wsRelease, allowed, err := helper.TryAcquireUserSlotForAPIKey(context.Background(), 7, 3, secondKey)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Nil(t, wsRelease)

	otherGroup := &service.APIKey{ID: 103, UserID: 7, Group: &service.Group{ID: 11, UserConcurrencyLimit: 1}}
	otherGroupRelease, allowed, err := helper.TryAcquireUserSlotForAPIKey(context.Background(), 7, 3, otherGroup)
	require.NoError(t, err)
	require.True(t, allowed)
	defer otherGroupRelease()

	otherUser := &service.APIKey{ID: 104, UserID: 8, Group: group}
	otherUserRelease, allowed, err := helper.TryAcquireUserSlotForAPIKey(context.Background(), 8, 3, otherUser)
	require.NoError(t, err)
	require.True(t, allowed)
	defer otherUserRelease()

	count, err := cache.GetUserConcurrency(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 2, count, "rejected requests must return their global slots")

	uncappedGroup := &service.APIKey{ID: 105, UserID: 7, Group: &service.Group{ID: 12}}
	_, allowed, err = helper.TryAcquireUserSlotForAPIKey(context.Background(), 7, 2, uncappedGroup)
	require.NoError(t, err)
	require.False(t, allowed, "the global user limit must still apply")

	cancel()
	require.Eventually(t, func() bool {
		count, err := cache.GetUserConcurrency(context.Background(), 7)
		return err == nil && count == 1
	}, time.Second, 10*time.Millisecond, "canceling the request must release its slots")
	wsRelease, allowed, err = helper.TryAcquireUserSlotForAPIKey(context.Background(), 7, 3, secondKey)
	require.NoError(t, err)
	require.True(t, allowed)
	wsRelease()
}
