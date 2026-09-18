// Package router tests versioned auth routes and management protection.
//
// @file      internal/router/router_auth_test.go
// @for       Table-driven authentication and gateway-key route contract tests.
// @uses      context, net/http/httptest, testing, time, internal/handler/service.
// @reason    P0 exit criteria require happy, validation, and auth coverage per
//
//	route; the real Go 1.22 mux must prove session gating at /api/v1.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-09-17
package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

type routerAuthRepo struct {
	mu   sync.Mutex
	hash string
}

func (r *routerAuthRepo) PasswordHash(context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hash, nil
}
func (r *routerAuthRepo) BootstrapPassword(_ context.Context, hash string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hash != "" {
		return false, nil
	}
	r.hash = hash
	return true, nil
}
func (r *routerAuthRepo) ChangePassword(_ context.Context, old, next string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hash != old {
		return domain.ErrPasswordChanged
	}
	r.hash = next
	return nil
}

type routerSessions struct {
	mu     sync.Mutex
	active map[string]bool
}

func newRouterSessions() *routerSessions { return &routerSessions{active: make(map[string]bool)} }
func (s *routerSessions) Create(_ context.Context, digest string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[digest] = true
	return nil
}
func (s *routerSessions) Exists(_ context.Context, digest string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[digest], nil
}
func (s *routerSessions) Revoke(_ context.Context, digest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, digest)
	return nil
}

type routerLimiter struct {
	mu   sync.Mutex
	fail map[string]int
}

func (l *routerLimiter) Locked(context.Context, string) (time.Duration, error) { return 0, nil }
func (l *routerLimiter) RecordFailure(_ context.Context, key string, max int, lockout time.Duration) (time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fail[key]++
	if l.fail[key] >= max {
		return lockout, nil
	}
	return 0, nil
}
func (l *routerLimiter) Reset(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fail, key)
	return nil
}

func newAuthenticatedRouter(t *testing.T) (*Mux, *handler.AuthHandler) {
	t.Helper()
	authRepo := &routerAuthRepo{}
	authSvc, err := service.NewAuthService(service.AuthServiceDeps{
		Repo: authRepo, Sessions: newRouterSessions(), Limiter: &routerLimiter{fail: make(map[string]int)}, Hasher: service.BcryptHasher{},
		Secret: []byte(strings.Repeat("s", 32)), SessionTTL: time.Hour, LoginMaxFails: 3, LoginLockout: time.Minute,
	})
	if err != nil {
		t.Fatalf("auth service: %v", err)
	}
	if _, err := authRepo.BootstrapPassword(context.Background(), mustHash(t, "correct")); err != nil {
		t.Fatal(err)
	}
	keySvc, err := service.NewGatewayKeyService(service.GatewayKeyServiceDeps{Repo: newMemKeyRepo(), Prefix: "sk-"})
	if err != nil {
		t.Fatalf("key service: %v", err)
	}
	authHandler := handler.NewAuthHandler(authSvc, handler.SessionCookieOptions{TTL: time.Hour})
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{Info: schema.SystemInfo{Version: "test", Commit: "test"}, Health: service.NewHealthService(service.HealthServiceDeps{})}),
		Auth:   authHandler, GatewayKey: handler.NewGatewayKeyHandler(keySvc),
	}), authHandler
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := service.BcryptHasher{}.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func TestVersionedRoutesRequireSession(t *testing.T) {
	mux, _ := newAuthenticatedRouter(t)
	cases := []struct{ name, method, path, body string }{
		{"list", http.MethodGet, "/api/v1/gateway-keys", ""},
		{"create", http.MethodPost, "/api/v1/gateway-keys", `{"name":"x"}`},
		{"detail", http.MethodGet, "/api/v1/gateway-keys/gky_unknown", ""},
		{"update", http.MethodPatch, "/api/v1/gateway-keys/gky_unknown", `{"name":"x"}`},
		{"revoke", http.MethodDelete, "/api/v1/gateway-keys/gky_unknown", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d want 401 body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAuthAndManagementLifecycleThroughV1Mux(t *testing.T) {
	mux, _ := newAuthenticatedRouter(t)
	login := httptest.NewRecorder()
	mux.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`)))
	if login.Code != http.StatusNoContent {
		t.Fatalf("login=%d body=%s", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	statusReq.AddCookie(cookie)
	status := httptest.NewRecorder()
	mux.ServeHTTP(status, statusReq)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"authenticated":true`) {
		t.Fatalf("status=%d body=%s", status.Code, status.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(`{"name":"v1-client"}`))
	createReq.AddCookie(cookie)
	create := httptest.NewRecorder()
	mux.ServeHTTP(create, createReq)
	if create.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s", create.Code, create.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	logout := httptest.NewRecorder()
	mux.ServeHTTP(logout, logoutReq)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout=%d body=%s", logout.Code, logout.Body.String())
	}

	denied := httptest.NewRecorder()
	mux.ServeHTTP(denied, createReq)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout create=%d want 401", denied.Code)
	}
}

var _ repository.AuthRepository = (*routerAuthRepo)(nil)
