package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

// SchedulingDiagnostics is registered only under the authenticated admin router.
func (h *AccountHandler) SchedulingDiagnostics(c *gin.Context) {
	var group int64
	if raw := c.Query("group_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		group = id
	}
	response.Success(c, service.RecentSchedulerDiagnostics(group))
}
