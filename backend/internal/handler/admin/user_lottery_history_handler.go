package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetLotteryHistory returns a user's individual draw results and lifetime totals.
// GET /api/v1/admin/users/:id/lottery-history
func (h *UserHandler) GetLotteryHistory(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	if h.paymentService == nil {
		response.Error(c, 503, "lottery service not available")
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	page, pageSize := response.ParsePagination(c)
	history, err := h.paymentService.GetUserRechargeLotteryHistory(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, history)
}
