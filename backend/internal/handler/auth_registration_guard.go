package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const registrationCookieName = "sub2api_registration"

// SetRegistrationGuard is called once during route setup, before serving traffic.
func (h *AuthHandler) SetRegistrationGuard(guard *service.RegistrationGuard) {
	h.registrationGuard = guard
}

func setRegistrationCookie(c *gin.Context, ticket string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: registrationCookieName, Value: ticket,
		Path: "/api/v1/auth", MaxAge: maxAge,
		HttpOnly: true, Secure: isRequestHTTPS(c), SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) beginBrowserRegistration(c *gin.Context, email string) (func(bool), error) {
	if err := service.ValidateRegistrationBrowser(c.Request.UserAgent()); err != nil {
		return nil, err
	}
	if h.registrationGuard == nil || h.authService == nil {
		return nil, service.ErrServiceUnavailable
	}
	ctx := c.Request.Context()
	if !h.authService.IsEmailVerifyEnabled(ctx) {
		// Email verification may be intentionally disabled; retain the captcha
		// check in Register, and apply the same longer-window registration quotas.
		if err := h.registrationGuard.ReserveAttempt(ctx, email, middleware2.SecurityClientIP(c)); err != nil {
			return nil, err
		}
		return func(bool) {}, nil
	}
	ticket, _ := c.Cookie(registrationCookieName)
	lease, err := h.registrationGuard.Acquire(ctx, ticket, email, c.Request.UserAgent())
	if err != nil {
		return nil, err
	}
	return func(success bool) {
		// Clean up even when the browser disconnects. Never log the cookie or
		// lease: they are bearer credentials for the pending registration.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if err := h.registrationGuard.Finish(cleanupCtx, ticket, lease, success); err != nil {
			slog.Error("registration session cleanup failed", "error", err)
		}
	}, nil
}
