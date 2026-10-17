package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *PaymentHandler) GetSubscriptionResetQuote(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}
	quote, err := h.paymentService.GetSubscriptionResetQuote(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quote)
}
