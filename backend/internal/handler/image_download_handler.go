package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GeneratedImageDownloadHandler proxies a generated image into a browser download.
type GeneratedImageDownloadHandler struct {
	service *service.GeneratedImageDownloadService
}

func NewGeneratedImageDownloadHandler(svc *service.GeneratedImageDownloadService) *GeneratedImageDownloadHandler {
	return &GeneratedImageDownloadHandler{service: svc}
}

func (h *GeneratedImageDownloadHandler) Download(c *gin.Context) {
	if h == nil || h.service == nil {
		writeGeneratedImageDownloadError(c, http.StatusInternalServerError, "Image download is unavailable")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		writeGeneratedImageDownloadError(c, http.StatusBadRequest, "Image URL is required")
		return
	}
	result, err := h.service.Download(c.Request.Context(), req.URL)
	if err != nil {
		status := http.StatusBadGateway
		message := "Failed to download image"
		if errors.Is(err, service.ErrGeneratedImageURLRejected) {
			status = http.StatusBadRequest
			message = "Image URL is not allowed"
		}
		writeGeneratedImageDownloadError(c, status, message)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.Filename))
	c.Data(http.StatusOK, result.ContentType, result.Data)
}

func writeGeneratedImageDownloadError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    "invalid_request_error",
			"message": message,
		},
	})
}
