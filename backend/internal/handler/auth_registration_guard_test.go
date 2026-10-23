//go:build unit

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const guardBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/155.0.0.0 Safari/537.36"

// HTTP tests use the service contract; Redis atomicity and expiration are
// exercised separately by repository tests against miniredis.
type registrationCacheStub struct {
	tickets map[string]string
	locks   map[string]string
	counts  map[string]int
}

func (s *registrationCacheStub) ReserveAttempts(_ context.Context, quotas []service.RegistrationQuota) (bool, error) {
	for _, quota := range quotas {
		if s.counts[quota.Key] >= quota.Limit {
			return false, nil
		}
	}
	for _, quota := range quotas {
		s.counts[quota.Key]++
	}
	return true, nil
}

func (s *registrationCacheStub) SetTicket(_ context.Context, key, binding string, _ time.Duration) error {
	s.tickets[key] = binding
	return nil
}

func (s *registrationCacheStub) AcquireTicket(_ context.Context, key, binding, lease string, _ time.Duration) (bool, error) {
	if s.tickets[key] != binding || s.locks[key] != "" {
		return false, nil
	}
	s.locks[key] = lease
	return true, nil
}

func (s *registrationCacheStub) FinishTicket(_ context.Context, key, lease string, success bool) error {
	if s.locks[key] == lease {
		delete(s.locks, key)
		if success {
			delete(s.tickets, key)
		}
	}
	return nil
}

func registrationGuardHandler(t *testing.T, emailEnabled bool) *AuthHandler {
	t.Helper()
	cache := &registrationCacheStub{tickets: map[string]string{}, locks: map[string]string{}, counts: map[string]int{}}
	cfg := &config.Config{}
	settings := service.NewSettingService(&oauthCaptchaSettingRepo{values: map[string]string{
		service.SettingKeyEmailVerifyEnabled: fmt.Sprint(emailEnabled),
	}}, cfg)
	auth := service.NewAuthService(nil, nil, nil, nil, cfg, settings, nil, nil, nil, nil, nil, nil, nil)
	return &AuthHandler{authService: auth, registrationGuard: service.NewRegistrationGuard(cache)}
}

func guardContext(ticket, ua string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://app.example/api/v1/auth/register", nil)
	c.Request.Header.Set("User-Agent", ua)
	if ticket != "" {
		c.Request.AddCookie(&http.Cookie{Name: registrationCookieName, Value: ticket})
	}
	return c
}

func TestBrowserRegistrationRequiresBoundCookieAndAllowsRetry(t *testing.T) {
	h := registrationGuardHandler(t, true)
	_, err := h.beginBrowserRegistration(guardContext("", guardBrowserUA), "user@example.com")
	require.ErrorIs(t, err, service.ErrRegistrationSession)
	ticket, err := h.registrationGuard.IssueTicket(context.Background(), "user@example.com", guardBrowserUA)
	require.NoError(t, err)
	_, err = h.beginBrowserRegistration(guardContext(ticket, "node"), "user@example.com")
	require.ErrorIs(t, err, service.ErrRegistrationBrowser)
	_, err = h.beginBrowserRegistration(guardContext(ticket, guardBrowserUA), "other@example.com")
	require.ErrorIs(t, err, service.ErrRegistrationSession)
	finish, err := h.beginBrowserRegistration(guardContext(ticket, guardBrowserUA), "user@example.com")
	require.NoError(t, err)
	finish(false)
	finish, err = h.beginBrowserRegistration(guardContext(ticket, guardBrowserUA), "user@example.com")
	require.NoError(t, err)
	finish(true)
	_, err = h.beginBrowserRegistration(guardContext(ticket, guardBrowserUA), "user@example.com")
	require.ErrorIs(t, err, service.ErrRegistrationSession)
}

func TestRegistrationCookieIsHttpOnlyScopedAndSecure(t *testing.T) {
	c := guardContext("", guardBrowserUA)
	setRegistrationCookie(c, "opaque-ticket", 600)
	cookie := c.Writer.Header().Get("Set-Cookie")
	require.Contains(t, cookie, "HttpOnly")
	require.Contains(t, cookie, "Secure")
	require.Contains(t, cookie, "SameSite=Strict")
	require.Contains(t, cookie, "Path=/api/v1/auth")
	require.NotContains(t, cookie, "Domain=")
}

func TestRegistrationQuotaIgnoresSpoofedForwardedHeaders(t *testing.T) {
	h := registrationGuardHandler(t, false)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.POST("/register", func(c *gin.Context) {
		_, err := h.beginBrowserRegistration(c, "user@example.com")
		if err != nil {
			c.Status(http.StatusTooManyRequests)
			return
		}
		c.Status(http.StatusOK)
	})
	for i := range 4 {
		req := httptest.NewRequest(http.MethodPost, "/register", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		req.Header.Set("User-Agent", guardBrowserUA)
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		req.Header.Set("CF-Connecting-IP", fmt.Sprintf("198.51.100.%d", i+1))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if i == 3 {
			require.Equal(t, http.StatusTooManyRequests, w.Code)
		} else {
			require.Equal(t, http.StatusOK, w.Code)
		}
	}
}

func TestRegistrationEndpointsRejectNodeBeforeIssuingMailOrAccounts(t *testing.T) {
	for _, path := range []string{"register", "send-verify-code"} {
		h := &AuthHandler{}
		router := gin.New()
		if path == "register" {
			router.POST("/"+path, h.Register)
		} else {
			router.POST("/"+path, h.SendVerifyCode)
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/"+path, strings.NewReader(`{"email":"user@example.com","password":"secret-123"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "node")
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code)
		require.Contains(t, w.Body.String(), "REGISTRATION_BROWSER_REQUIRED")
	}
}
