// Package handler adapts HTTP requests to app-serv services.
//
// @file      internal/handler/auth.go
// @for       Dashboard authentication HTTP endpoints and session guard.
// @uses      internal/domain, internal/schema, internal/service, net/http.
// @reason    SPEC-API-001 §7.2 defines the public auth contract while the
//
//	gateway-key management surface must reject unauthenticated calls.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-17
package handler

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// AuthHandler serves dashboard auth and owns cookie deployment settings.
type AuthHandler struct {
	auth   *service.AuthService
	cookie SessionCookieOptions
}

// NewAuthHandler validates dependencies and returns an auth HTTP handler.
func NewAuthHandler(auth *service.AuthService, cookie SessionCookieOptions) *AuthHandler {
	return &AuthHandler{auth: auth, cookie: cookie}
}

// Login serves POST /api/v1/auth/login and sets the session cookie on success.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req schema.LoginRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidatePasswordBytes(req.Password); err != nil {
		schema.WriteError(w, err)
		return
	}
	token, err := h.auth.Login(r.Context(), clientAddress(r), req.Password)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	setSessionCookie(w, token, h.cookie)
	w.WriteHeader(http.StatusNoContent)
}

// Logout serves POST /api/v1/auth/logout and revokes the current session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.Logout(r.Context(), sessionToken(r)); err != nil {
		schema.WriteError(w, err)
		return
	}
	clearSessionCookie(w, h.cookie.Secure)
	w.WriteHeader(http.StatusNoContent)
}

// Status serves GET /api/v1/auth/status.
func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	status, err := h.auth.Status(r.Context(), sessionToken(r))
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.AuthStatusResponse{
		Authenticated: status.Authenticated, RequireLogin: status.RequireLogin,
		PasswordConfigured: status.PasswordConfigured,
	})
}

// ChangePassword serves POST /api/v1/auth/change-password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req schema.ChangePasswordRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidatePasswordBytes(req.CurrentPassword, req.NewPassword); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := h.auth.ChangePassword(r.Context(), sessionToken(r), req.CurrentPassword, req.NewPassword); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RequireSession rejects protected routes before their handlers run.
func (h *AuthHandler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h.auth.Authenticate(r.Context(), sessionToken(r)); err != nil {
			schema.WriteError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

// RetryAfterSeconds converts a duration into the HTTP Retry-After value.
func RetryAfterSeconds(d time.Duration) string {
	return strconv.FormatInt(maxInt64(1, int64((d+time.Second-1)/time.Second)), 10)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
