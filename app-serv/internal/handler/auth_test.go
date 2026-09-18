// Package handler tests dashboard authentication HTTP contracts.
//
// @file      internal/handler/auth_test.go
// @for       Table-driven auth endpoint tests through typed service contracts.
// @uses      context, net/http/httptest, testing, time, internal/domain/service.
// @reason    Strict TDD requires happy, validation, and authentication cases
//
//	for every P0 auth route without mocking handler output.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-17
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

type handlerAuthRepo struct {
	mu   sync.Mutex
	hash string
}

func (r *handlerAuthRepo) PasswordHash(context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hash, nil
}
func (r *handlerAuthRepo) BootstrapPassword(_ context.Context, hash string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hash != "" {
		return false, nil
	}
	r.hash = hash
	return true, nil
}
func (r *handlerAuthRepo) ChangePassword(_ context.Context, old, next string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hash != old {
		return domain.ErrPasswordChanged
	}
	r.hash = next
	return nil
}

type handlerSessionStore struct {
	mu     sync.Mutex
	active map[string]bool
}

func newHandlerSessionStore() *handlerSessionStore {
	return &handlerSessionStore{active: make(map[string]bool)}
}
func (s *handlerSessionStore) Create(_ context.Context, digest string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[digest] = true
	return nil
}
func (s *handlerSessionStore) Exists(_ context.Context, digest string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[digest], nil
}
func (s *handlerSessionStore) Revoke(_ context.Context, digest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, digest)
	return nil
}

type handlerLoginLimiter struct {
	mu     sync.Mutex
	locked map[string]time.Duration
	fails  map[string]int
}

func newHandlerLoginLimiter() *handlerLoginLimiter {
	return &handlerLoginLimiter{locked: make(map[string]time.Duration), fails: make(map[string]int)}
}
func (l *handlerLoginLimiter) Locked(_ context.Context, key string) (time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locked[key], nil
}
func (l *handlerLoginLimiter) RecordFailure(_ context.Context, key string, max int, lockout time.Duration) (time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key]++
	if l.fails[key] >= max {
		l.locked[key] = lockout
		return lockout, nil
	}
	return 0, nil
}
func (l *handlerLoginLimiter) Reset(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.locked, key)
	return nil
}

func newAuthHandlerForTest(t *testing.T, password string) (*AuthHandler, *handlerAuthRepo) {
	t.Helper()
	repo := &handlerAuthRepo{}
	svc, err := service.NewAuthService(service.AuthServiceDeps{
		Repo: repo, Sessions: newHandlerSessionStore(), Limiter: newHandlerLoginLimiter(), Hasher: service.BcryptHasher{},
		Secret: []byte(strings.Repeat("s", 32)), SessionTTL: time.Hour, LoginMaxFails: 3, LoginLockout: 15 * time.Minute,
		BootstrapPassword: password,
	})
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return NewAuthHandler(svc, SessionCookieOptions{TTL: time.Hour}), repo
}

func TestAuthHandlerLoginTable(t *testing.T) {
	cases := []struct {
		name, body string
		wantStatus int
		wantCookie bool
	}{
		{"valid password", `{"password":"correct"}`, http.StatusNoContent, true},
		{"missing password", `{}`, http.StatusBadRequest, false},
		{"malformed json", `{"password":`, http.StatusBadRequest, false},
		{"wrong password", `{"password":"wrong"}`, http.StatusUnauthorized, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newAuthHandlerForTest(t, "correct")
			r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(tc.body))
			r.RemoteAddr = "127.0.0.1:3456"
			w := httptest.NewRecorder()
			h.Login(w, r)
			if w.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			if (len(w.Result().Cookies()) > 0) != tc.wantCookie {
				t.Fatalf("cookie presence=%v want=%v", len(w.Result().Cookies()) > 0, tc.wantCookie)
			}
			if tc.wantCookie {
				cookie := w.Result().Cookies()[0]
				if cookie.Name != sessionCookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
					t.Fatalf("unexpected cookie: %+v", cookie)
				}
			}
		})
	}
}

func TestAuthHandlerSessionLifecycle(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, "correct")
	login := httptest.NewRecorder()
	h.Login(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`)))
	if login.Code != http.StatusNoContent {
		t.Fatalf("login status=%d", login.Code)
	}
	cookie := login.Result().Cookies()[0]

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	statusReq.AddCookie(cookie)
	status := httptest.NewRecorder()
	h.Status(status, statusReq)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"authenticated":true`) {
		t.Fatalf("status=%d body=%s", status.Code, status.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	logout := httptest.NewRecorder()
	h.Logout(logout, logoutReq)
	if logout.Code != http.StatusNoContent || logout.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout=%d cookie=%+v", logout.Code, logout.Result().Cookies())
	}

	protected := httptest.NewRecorder()
	h.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(protected, logoutReq)
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status=%d", protected.Code)
	}
}

func TestAuthHandlerChangePasswordTable(t *testing.T) {
	cases := []struct {
		name, current, next string
		want                int
	}{
		{"valid", "correct", "next-password", http.StatusNoContent},
		{"wrong current", "wrong", "next-password", http.StatusUnauthorized},
		{"empty new", "correct", "", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newAuthHandlerForTest(t, "correct")
			login := httptest.NewRecorder()
			h.Login(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`)))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(`{"current_password":"`+tc.current+`","new_password":"`+tc.next+`"}`))
			req.AddCookie(login.Result().Cookies()[0])
			w := httptest.NewRecorder()
			h.ChangePassword(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

var _ repository.AuthRepository = (*handlerAuthRepo)(nil)
