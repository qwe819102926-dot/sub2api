package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestSchedulingDiagnosticsValidateGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &AccountHandler{}
	router.GET("/diagnostics", h.SchedulingDiagnostics)
	for _, group := range []string{"-1", "0", "garbage", "99999999999999999999999"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/diagnostics?group_id="+group, nil))
		require.Equal(t, 400, w.Code)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/diagnostics?group_id=1", nil))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"data":[]`)
}
