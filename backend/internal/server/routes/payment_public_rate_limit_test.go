package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newPaymentRoutesTestRouter(redisClient *redis.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	noop := func(c *gin.Context) { c.Next() }

	RegisterPaymentRoutes(
		v1,
		&handler.PaymentHandler{},
		&handler.PaymentWebhookHandler{},
		&admin.PaymentHandler{},
		servermiddleware.JWTAuthMiddleware(noop),
		servermiddleware.AdminAuthMiddleware(noop),
		servermiddleware.AuditLogMiddleware(noop),
		nil,
		nil,
		redisClient,
	)
	return router
}

func postPublicOrderResolve(router *gin.Engine, remoteAddr string) *httptest.ResponseRecorder {
	// Empty body fails binding in the handler, so no service is touched and a
	// non-429 response proves the request passed the limiter.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/public/orders/resolve", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestPublicOrderResolveRateLimitedPerIP(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	router := newPaymentRoutesTestRouter(rdb)

	for i := 1; i <= publicOrderResolveRateLimit; i++ {
		w := postPublicOrderResolve(router, "198.51.100.20:1234")
		require.Equal(t, http.StatusBadRequest, w.Code, "request %d should reach the handler", i)
	}

	w := postPublicOrderResolve(router, "198.51.100.20:1234")
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Contains(t, w.Body.String(), "rate limit exceeded")
	require.Equal(t, "60", w.Header().Get("Retry-After"))

	// A different client IP has its own budget.
	w = postPublicOrderResolve(router, "198.51.100.21:1234")
	require.Equal(t, http.StatusBadRequest, w.Code)

	mr.FastForward(publicOrderResolveRateLimitWindow)
	w = postPublicOrderResolve(router, "198.51.100.20:1234")
	require.Equal(t, http.StatusBadRequest, w.Code, "expired windows must allow requests again")
}

func TestPublicOrderResolveRateLimitFailsClosedWhenRedisUnavailable(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	router := newPaymentRoutesTestRouter(rdb)
	w := postPublicOrderResolve(router, "203.0.113.30:1234")
	require.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestPublicOrderVerifyRouteIsRetired(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	router := newPaymentRoutesTestRouter(rdb)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/public/orders/verify", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
