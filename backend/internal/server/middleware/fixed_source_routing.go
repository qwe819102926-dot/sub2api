package middleware

import (
	"context"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// NewFixedSourceRoutingMiddleware detects configured browser origins or trusted
// client IPs and routes matching source groups to a pinned account.
func NewFixedSourceRoutingMiddleware(settingService *service.SettingService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if settingService == nil {
			c.Next()
			return
		}
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.GroupID == nil {
			c.Next()
			return
		}
		settings, err := settingService.GetFixedSourceRoutingSettings(c.Request.Context())
		if err != nil || settings == nil || !settings.Enabled || !matchesFixedSource(c, settings, cfg) {
			c.Next()
			return
		}
		var route *service.FixedSourceRoute
		for i := range settings.Routes {
			candidate := &settings.Routes[i]
			if candidate.SourceGroupID == *apiKey.GroupID {
				route = candidate
				break
			}
		}
		if route == nil {
			c.Next()
			return
		}
		targetGroup, err := settingService.GetFixedSourceRoutingTargetGroup(c.Request.Context(), route.TargetGroupID)
		if err != nil || targetGroup == nil || !targetGroup.IsActive() {
			AbortWithError(c, 503, "FIXED_ROUTE_UNAVAILABLE", "The configured fixed route group is unavailable")
			return
		}

		// API keys returned by authentication are cache-backed. Never mutate them.
		keyCopy := *apiKey
		targetGroupID := targetGroup.ID
		keyCopy.GroupID = &targetGroupID
		keyCopy.Group = targetGroup
		c.Set(string(ContextKeyAPIKey), &keyCopy)
		setGroupContext(c, targetGroup)
		ctx := context.WithValue(c.Request.Context(), ctxkey.FixedRouteAccountID, route.AccountID)
		ctx = context.WithValue(ctx, ctxkey.FixedRouteSourceGroupID, route.SourceGroupID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func matchesFixedSource(c *gin.Context, settings *service.FixedSourceRoutingSettings, cfg *config.Config) bool {
	if settings == nil {
		return false
	}
	if domain := fixedSourceRequestDomain(c); domain != "" {
		for _, configured := range settings.Domains {
			if domain == configured || "www."+domain == configured || domain == "www."+configured {
				return true
			}
		}
	}
	trustForwarded := cfg != nil && cfg.TrustForwardedIPForAPIKeyACL()
	return ip.MatchesAnyPattern(ip.GetSecurityClientIP(c, trustForwarded), settings.IPs)
}

func fixedSourceRequestDomain(c *gin.Context) string {
	if c == nil {
		return ""
	}
	for _, header := range []string{"Origin", "Referer"} {
		value := strings.TrimSpace(c.GetHeader(header))
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		domain := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
		return strings.TrimPrefix(domain, "www.")
	}
	return ""
}
