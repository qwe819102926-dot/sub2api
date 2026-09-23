package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeneratedImageDownloadHandlerWritesAttachment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/html" {
			_, _ = w.Write([]byte("<html><body>not an image</body></html>"))
			return
		}
		_, _ = w.Write(png)
	}))
	t.Cleanup(upstream.Close)

	router := gin.New()
	handler := NewGeneratedImageDownloadHandler(service.NewGeneratedImageDownloadServiceForTest(&http.Client{}))
	router.POST("/v1/images/download", handler.Download)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/images/download", strings.NewReader(`{"url":"`+upstream.URL+`/ok"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "generated.png")
	require.Equal(t, string(png), recorder.Body.String())

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/v1/images/download", strings.NewReader(`{"url":"`+upstream.URL+`/html"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Failed to download image")
}

func TestGeneratedImageDownloadHandlerRejectsMissingAndPrivateURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/images/download", NewGeneratedImageDownloadHandler(service.NewGeneratedImageDownloadService()).Download)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/images/download", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Image URL is required")

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/v1/images/download", strings.NewReader(`{"url":"http://169.254.169.254/latest/meta-data"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Image URL is not allowed")
}
