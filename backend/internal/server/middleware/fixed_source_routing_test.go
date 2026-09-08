package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fixedRoutingMiddlewareRepo struct {
	service.SettingRepository
	value string
}

func (r *fixedRoutingMiddlewareRepo) GetValue(context.Context, string) (string, error) {
	return r.value, nil
}

type fixedRoutingGroupReader struct {
	group *service.Group
}

func (r *fixedRoutingGroupReader) GetByID(context.Context, int64) (*service.Group, error) {
	return r.group, nil
}

func newFixedRoutingMiddlewareService(t *testing.T, target *service.Group) *service.SettingService {
	t.Helper()
	settings := service.FixedSourceRoutingSettings{
		Enabled: true,
		Domains: []string{"example.com"},
		IPs:     []string{"192.0.2.0/24", "2001:db8::/32"},
		Routes:  []service.FixedSourceRoute{{SourceGroupID: 10, TargetGroupID: 20, AccountID: 30}},
	}
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	svc := service.NewSettingService(&fixedRoutingMiddlewareRepo{value: string(raw)}, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(&fixedRoutingGroupReader{group: target})
	return svc
}

func TestFixedSourceRequestDomainOriginAndReferer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		origin  string
		referer string
		want    string
	}{
		{name: "origin normalized", origin: "https://WWW.Example.COM.:8443/path", want: "example.com"},
		{name: "referer fallback", referer: "https://example.com/some/page", want: "example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Referer", tt.referer)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req
			require.Equal(t, tt.want, fixedSourceRequestDomain(c))
		})
	}
}

func TestFixedSourceRoutingMiddlewareCopiesAPIKeyAndPinsAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	target := &service.Group{ID: 20, Name: "target", Status: service.StatusActive, Hydrated: true}
	svc := newFixedRoutingMiddlewareService(t, target)
	originalGroupID := int64(10)
	original := &service.APIKey{ID: 1, GroupID: &originalGroupID, Group: &service.Group{ID: 10, Hydrated: true}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(NewFixedSourceRoutingMiddleware(svc, &config.Config{}))
	router.GET("/", func(c *gin.Context) {
		key, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		accountID, _ := c.Request.Context().Value(ctxkey.FixedRouteAccountID).(int64)
		c.JSON(http.StatusOK, gin.H{"group_id": *key.GroupID, "account_id": accountID})
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://WWW.EXAMPLE.COM.")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.JSONEq(t, `{"group_id":20,"account_id":30}`, resp.Body.String())
	require.Equal(t, int64(10), *original.GroupID)
	require.Equal(t, int64(10), original.Group.ID)
}

func TestFixedSourceRoutingMiddlewareMatchesCIDRAndFailsClosedForInactiveTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	target := &service.Group{ID: 20, Status: service.StatusDisabled, Hydrated: true}
	svc := newFixedRoutingMiddlewareService(t, target)
	groupID := int64(10)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), &service.APIKey{GroupID: &groupID})
		c.Next()
	})
	router.Use(NewFixedSourceRoutingMiddleware(svc, &config.Config{}))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.42:1234"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestFixedSourceRoutingMiddlewareLeavesUnmatchedAPIKeyUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	target := &service.Group{ID: 20, Status: service.StatusActive, Hydrated: true}
	svc := newFixedRoutingMiddlewareService(t, target)
	groupID := int64(10)
	original := &service.APIKey{GroupID: &groupID}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(NewFixedSourceRoutingMiddleware(svc, &config.Config{}))
	router.GET("/", func(c *gin.Context) {
		key, _ := GetAPIKeyFromContext(c)
		require.Same(t, original, key)
		require.Nil(t, c.Request.Context().Value(ctxkey.FixedRouteAccountID))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://unmatched.example")
	req.RemoteAddr = "203.0.113.10:1234"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNoContent, resp.Code)
}
