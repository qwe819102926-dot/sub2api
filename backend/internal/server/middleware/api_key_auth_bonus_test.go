//go:build unit

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type authBonusCreditRepo struct {
	service.UserRepository
	credit float64
	err    error
}

func (r *authBonusCreditRepo) GetBonusBalanceCredit(context.Context, int64) (float64, error) {
	return r.credit, r.err
}

func TestAPIKeyAuthBonusBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, google := range []bool{false, true} {
		name := "standard"
		if google {
			name = "google"
		}
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name             string
				principal, bonus float64
				err              error
				status           int
			}{
				{"bonus_only", 0, 0.5, nil, http.StatusOK},
				{"both_empty", 0, 0, nil, http.StatusForbidden},
				{"debt_exceeds_bonus", -1, 0.5, nil, http.StatusForbidden},
				{"bonus_lookup_failure", 0, 0, errors.New("db unavailable"), http.StatusServiceUnavailable},
				{"positive_principal_fast_path", 0.005, 0, errors.New("should not read bonus"), http.StatusOK},
			} {
				t.Run(tc.name, func(t *testing.T) {
					bonusRepo := &authBonusCreditRepo{credit: tc.bonus, err: tc.err}
					user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: tc.principal}
					keyRepo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
						return &service.APIKey{ID: 1, Key: "bonus-test", UserID: 7, User: user, Status: service.StatusActive}, nil
					}}
					cfg := &config.Config{RunMode: config.RunModeStandard}
					cfg.Billing.MinimumBalanceReserve = 0.01
					svc := service.NewAPIKeyService(keyRepo, bonusRepo, nil, nil, nil, nil, cfg)
					router := gin.New()
					if google {
						router.Use(APIKeyAuthGoogle(svc, cfg))
					} else {
						router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
					}
					router.POST("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
					request := func() int {
						w := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/test", nil)
						req.Header.Set("Authorization", "Bearer bonus-test")
						router.ServeHTTP(w, req)
						return w.Code
					}
					require.Equal(t, tc.status, request())
					if tc.name == "bonus_only" {
						bonusRepo.credit = 0
						require.Equal(t, http.StatusForbidden, request(), "spent bonus must not survive in the auth cache")
						bonusRepo.credit = 0.5
						require.Equal(t, http.StatusOK, request(), "a new grant must work immediately")
					}
				})
			}
		})
	}
}
