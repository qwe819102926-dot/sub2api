package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_GetLotteryHistoryRejectsInvalidUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, id := range []string{"invalid", "0", "-1"} {
		handler := NewUserHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil)
		router := gin.New()
		router.GET("/users/:id/lottery-history", handler.GetLotteryHistory)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users/"+id+"/lottery-history", nil))
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	}
}

func TestUserHandler_GetLotteryHistoryMissingUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := newStubAdminService()
	stub.getUserErr = service.ErrUserNotFound
	handler := NewUserHandler(stub, nil, nil, nil, nil, nil, nil)
	handler.SetPaymentService(&service.PaymentService{})
	router := gin.New()
	router.GET("/users/:id/lottery-history", handler.GetLotteryHistory)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users/7/lottery-history", nil))
	require.Equal(t, http.StatusNotFound, recorder.Code)
}
