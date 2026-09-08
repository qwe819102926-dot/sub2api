package admin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type fixedSourceRoutingAdminReader interface {
	GetGroup(ctx context.Context, id int64) (*service.Group, error)
	GetAccount(ctx context.Context, id int64) (*service.Account, error)
}

// GetFixedSourceRoutingSettings returns the fixed source routing configuration.
func (h *SettingHandler) GetFixedSourceRoutingSettings(c *gin.Context) {
	settings, err := h.settingService.GetFixedSourceRoutingSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// UpdateFixedSourceRoutingSettings validates and stores the complete configuration.
func (h *SettingHandler) UpdateFixedSourceRoutingSettings(c *gin.Context) {
	var settings service.FixedSourceRoutingSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h.fixedSourceRoutingReader == nil {
		response.Error(c, http.StatusServiceUnavailable, "Fixed source routing validation is unavailable")
		return
	}
	if err := h.validateFixedSourceRoutingSettings(c.Request.Context(), &settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.settingService.SetFixedSourceRoutingSettings(c.Request.Context(), &settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	updated, err := h.settingService.GetFixedSourceRoutingSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *SettingHandler) validateFixedSourceRoutingSettings(ctx context.Context, settings *service.FixedSourceRoutingSettings) error {
	seen := make(map[int64]struct{}, len(settings.Routes))
	for _, route := range settings.Routes {
		if route.SourceGroupID <= 0 || route.TargetGroupID <= 0 || route.AccountID <= 0 {
			return fmt.Errorf("every route requires a source group, target group, and account")
		}
		if _, ok := seen[route.SourceGroupID]; ok {
			return fmt.Errorf("source group %d has more than one fixed route", route.SourceGroupID)
		}
		seen[route.SourceGroupID] = struct{}{}

		sourceGroup, err := h.fixedSourceRoutingReader.GetGroup(ctx, route.SourceGroupID)
		if err != nil || sourceGroup == nil {
			return fmt.Errorf("source group %d does not exist", route.SourceGroupID)
		}
		targetGroup, err := h.fixedSourceRoutingReader.GetGroup(ctx, route.TargetGroupID)
		if err != nil || targetGroup == nil {
			return fmt.Errorf("target group %d does not exist", route.TargetGroupID)
		}
		if !targetGroup.IsActive() {
			return fmt.Errorf("target group %d is not active", route.TargetGroupID)
		}
		account, err := h.fixedSourceRoutingReader.GetAccount(ctx, route.AccountID)
		if err != nil || account == nil {
			return fmt.Errorf("account %d does not exist", route.AccountID)
		}
		if !containsInt64(account.GroupIDs, route.TargetGroupID) {
			return fmt.Errorf("account %d is not bound to target group %d", route.AccountID, route.TargetGroupID)
		}
	}
	return nil
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
