//go:build unit

package repository_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const registrationTestUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/155.0.0.0 Safari/537.36"

func newRegistrationGuardTest(t *testing.T) (*service.RegistrationGuard, *miniredis.Miniredis) {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return service.NewRegistrationGuard(repository.NewRegistrationGuardCache(client)), srv
}

func TestRegistrationGuardBrowserSignal(t *testing.T) {
	for _, ua := range []string{"", "node", "curl/8.0", "python-requests/2.0", "Mozilla/5.0 fake"} {
		require.ErrorIs(t, service.ValidateRegistrationBrowser(ua), service.ErrRegistrationBrowser)
	}
	for _, ua := range []string{registrationTestUA,
		"Mozilla/5.0 (X11; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1"} {
		require.NoError(t, service.ValidateRegistrationBrowser(ua))
	}
}

func TestRegistrationGuardTicketBindingRetryAndReplay(t *testing.T) {
	g, _ := newRegistrationGuardTest(t)
	ctx := context.Background()
	ticket, err := g.IssueTicket(ctx, "User@Example.com", registrationTestUA)
	require.NoError(t, err)
	for _, invalid := range []struct{ token, email, ua string }{
		{"", "user@example.com", registrationTestUA},
		{ticket, "other@example.com", registrationTestUA},
		{ticket, "user@example.com", "node"},
		{ticket, "user@example.com", registrationTestUA + " changed"},
	} {
		_, err := g.Acquire(ctx, invalid.token, invalid.email, invalid.ua)
		require.ErrorIs(t, err, service.ErrRegistrationSession)
	}
	lease, err := g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.NoError(t, err)
	_, err = g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.ErrorIs(t, err, service.ErrRegistrationSession)
	// A wrong verification code must not force the user to request another email.
	require.NoError(t, g.Finish(ctx, ticket, lease, false))
	lease, err = g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.NoError(t, err)
	require.NoError(t, g.Finish(ctx, ticket, lease, true))
	_, err = g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.ErrorIs(t, err, service.ErrRegistrationSession)
}

func TestRegistrationGuardExpiryAndStaleLease(t *testing.T) {
	g, srv := newRegistrationGuardTest(t)
	ctx := context.Background()
	ticket, err := g.IssueTicket(ctx, "user@example.com", registrationTestUA)
	require.NoError(t, err)
	old, err := g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.NoError(t, err)
	srv.FastForward(121 * time.Second)
	current, err := g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.NoError(t, err)
	require.NotEqual(t, old, current)
	require.NoError(t, g.Finish(ctx, ticket, old, true))
	_, err = g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.ErrorIs(t, err, service.ErrRegistrationSession)
	require.NoError(t, g.Finish(ctx, ticket, current, false))
	srv.FastForward(service.RegistrationTicketTTL)
	_, err = g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.ErrorIs(t, err, service.ErrRegistrationSession)
}

func TestRegistrationGuardConcurrentTicketSubmission(t *testing.T) {
	g, _ := newRegistrationGuardTest(t)
	ctx := context.Background()
	ticket, err := g.IssueTicket(ctx, "user@example.com", registrationTestUA)
	require.NoError(t, err)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if _, err := g.Acquire(ctx, ticket, "user@example.com", registrationTestUA); err == nil {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	require.EqualValues(t, 1, accepted.Load())
}

func TestRegistrationGuardQuotasAndWindowExpiry(t *testing.T) {
	g, srv := newRegistrationGuardTest(t)
	ctx := context.Background()
	for range 3 {
		require.NoError(t, g.ReserveAttempt(ctx, "user@example.com", "203.0.113.10"))
	}
	require.ErrorIs(t, g.ReserveAttempt(ctx, "another@example.com", "203.0.113.10"), service.ErrRegistrationLimited)
	srv.FastForward(10 * time.Minute)
	for range 2 {
		require.NoError(t, g.ReserveAttempt(ctx, "USER@example.com", "203.0.113.11"))
	}
	// Email quota remains effective even after changing IP and email case.
	require.ErrorIs(t, g.ReserveAttempt(ctx, "user@example.com", "198.51.100.1"), service.ErrRegistrationLimited)
	srv.FastForward(24 * time.Hour)
	require.NoError(t, g.ReserveAttempt(ctx, "user@example.com", "203.0.113.10"))
}

func TestRegistrationGuardIPv6RotationAndNetworkQuota(t *testing.T) {
	g, _ := newRegistrationGuardTest(t)
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		require.NoError(t, g.ReserveAttempt(ctx, fmt.Sprintf("user%d@example.com", i), fmt.Sprintf("2001:db8:1:2::%x", i)))
	}
	require.ErrorIs(t, g.ReserveAttempt(ctx, "other@example.com", "2001:db8:1:2::ffff"), service.ErrRegistrationLimited)
	// Rotating IPv4 addresses within the subnet cannot bypass its daily ceiling.
	for i := 1; i <= 50; i++ {
		require.NoError(t, g.ReserveAttempt(ctx, fmt.Sprintf("user%d@example.com", i), fmt.Sprintf("203.0.113.%d", i)))
	}
	require.ErrorIs(t, g.ReserveAttempt(ctx, "other@example.com", "203.0.113.100"), service.ErrRegistrationLimited)
	require.NoError(t, g.ReserveAttempt(ctx, "other@example.com", "203.0.114.100"))
}

func TestRegistrationGuardDailyIPQuotaAndFailClosed(t *testing.T) {
	g, srv := newRegistrationGuardTest(t)
	ctx := context.Background()
	for i := range 10 {
		srv.FastForward(11 * time.Minute)
		require.NoError(t, g.ReserveAttempt(ctx, fmt.Sprintf("user%d@example.com", i), "203.0.113.10"))
	}
	require.ErrorIs(t, g.ReserveAttempt(ctx, "other@example.com", "203.0.113.10"), service.ErrRegistrationLimited)
	ticket, err := g.IssueTicket(ctx, "user@example.com", registrationTestUA)
	require.NoError(t, err)
	srv.Close()
	require.ErrorIs(t, g.ReserveAttempt(ctx, "other@example.com", "203.0.114.1"), service.ErrServiceUnavailable)
	_, err = g.IssueTicket(ctx, "user@example.com", registrationTestUA)
	require.ErrorIs(t, err, service.ErrServiceUnavailable)
	_, err = g.Acquire(ctx, ticket, "user@example.com", registrationTestUA)
	require.ErrorIs(t, err, service.ErrServiceUnavailable)
}
