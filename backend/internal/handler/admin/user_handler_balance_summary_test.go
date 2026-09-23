package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_GetBalanceSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := newStubAdminService()
	stub.balanceSummary = &service.UserBalanceSummary{TotalBalance: 88.5, TotalBonusBalance: 12.25}
	handler := NewUserHandler(stub, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/users/balance-summary", handler.GetBalanceSummary)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/balance-summary", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Code int                        `json:"code"`
		Data service.UserBalanceSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, 88.5, body.Data.TotalBalance)
	require.Equal(t, 12.25, body.Data.TotalBonusBalance)
}
