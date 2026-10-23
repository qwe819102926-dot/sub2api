package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AuthHandler) GetRegistrationIPMonitor(c *gin.Context) {
	page, _ := response.ParsePagination(c)
	r, err := h.authService.GetRegistrationIPReport(c.Request.Context(), c.Query("search"), c.Query("ip"), page)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, r)
}
func (h *AuthHandler) SetRegistrationIPMonitorConfig(c *gin.Context) {
	var req service.RegistrationIPMonitorConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid configuration")
		return
	}
	if err := h.settingSvc.SetRegistrationIPMonitorConfig(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, req)
}
func (h *AuthHandler) UnblockRegistrationIP(c *gin.Context) {
	var req struct {
		IP string `json:"ip" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid IP")
		return
	}
	if err := h.authService.UnblockRegistrationIP(c.Request.Context(), req.IP); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}
