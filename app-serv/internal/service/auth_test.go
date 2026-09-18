// Package service tests dashboard authentication use cases.
//
// @file      internal/service/auth_test.go
// @for       Table-driven verification of password, session, and lockout rules.
// @uses      context, sync, testing, time, internal/domain/repository.
// @reason    Strict TDD requires auth behavior to generalize across success,
//
//	validation, lockout, reset, bootstrap, and concurrent state cases.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

type authRepoFake struct {
	mu   sync.Mutex
	hash string
}

func (r *authRepoFake) PasswordHash(context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hash, nil
}

func (r *authRepoFake) BootstrapPassword(_ context.Context, hash string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hash != "" {
		return false, nil
	}
	r.hash = hash
	return true, nil
}

func (r *authRepoFake) ChangePassword(_ context.Context, expected, next string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hash != expected {
		return domain.ErrPasswordChanged
	}
	r.hash = next
	return nil
}

type sessionFake struct {
	mu     sync.Mutex
	active map[string]bool
}

func newSessionFake() *sessionFake { return &sessionFake{active: make(map[string]bool)} }
func (s *sessionFake) Create(_ context.Context, digest string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[digest] = true
	return nil
}
func (s *sessionFake) Exists(_ context.Context, digest string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[digest], nil
}
func (s *sessionFake) Revoke(_ context.Context, digest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, digest)
	return nil
}

type limiterFake struct {
	mu      sync.Mutex
	fails   map[string]int
	locked  map[string]time.Duration
	resets  int
	maxFail int
}

func newLimiterFake() *limiterFake {
	return &limiterFake{fails: make(map[string]int), locked: make(map[string]time.Duration)}
}
func (l *limiterFake) Locked(_ context.Context, key string) (time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locked[key], nil
}
func (l *limiterFake) RecordFailure(_ context.Context, key string, max int, lockout time.Duration) (time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key]++
	l.maxFail = max
	if l.fails[key] >= max {
		l.locked[key] = lockout
		return lockout, nil
	}
	return 0, nil
}
func (l *limiterFake) Reset(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.locked, key)
	l.resets++
	return nil
}

func newAuthForTest(t *testing.T, password string) (*AuthService, *authRepoFake, *sessionFake, *limiterFake) {
	t.Helper()
	repo := &authRepoFake{}
	limiter := newLimiterFake()
	sessions := newSessionFake()
	svc, err := NewAuthService(AuthServiceDeps{
		Repo: repo, Sessions: sessions, Limiter: limiter, Hasher: BcryptHasher{},
		Secret: []byte(strings.Repeat("s", 32)), SessionTTL: time.Hour,
		LoginMaxFails: 3, LoginLockout: 15 * time.Minute,
		BootstrapPassword: password,
	})
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return svc, repo, sessions, limiter
}

func TestAuthLoginTable(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  string
	}{
		{name: "correct password", password: "correct-123", wantErr: ""},
		{name: "wrong password", password: "wrong-123", wantErr: "UNAUTHORIZED"},
		{name: "empty password", password: "", wantErr: "UNAUTHORIZED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _ := newAuthForTest(t, "correct-123")
			token, err := svc.Login(context.Background(), "127.0.0.1", tc.password)
			if tc.wantErr == "" {
				if err != nil || token == "" {
					t.Fatalf("Login() token=%q err=%v", token, err)
				}
				return
			}
			if err == nil || domain.AsAppError(err).Code != tc.wantErr {
				t.Fatalf("Login() err=%v, want %s", err, tc.wantErr)
			}
		})
	}
}

func TestAuthLockoutAndReset(t *testing.T) {
	svc, _, _, limiter := newAuthForTest(t, "correct-123")
	for i := 0; i < 2; i++ {
		if _, err := svc.Login(context.Background(), "client", "bad"); err == nil {
			t.Fatal("failed login unexpectedly succeeded")
		}
	}
	if _, err := svc.Login(context.Background(), "client", "bad"); domain.AsAppError(err).Code != "RATE_LIMITED" {
		t.Fatalf("third failure error=%v, want RATE_LIMITED", err)
	}
	if _, err := svc.Login(context.Background(), "client", "correct-123"); domain.AsAppError(err).Code != "RATE_LIMITED" {
		t.Fatalf("locked login error=%v, want RATE_LIMITED", err)
	}
	limiter.mu.Lock()
	limiter.locked["client"] = 0
	limiter.mu.Unlock()
	if _, err := svc.Login(context.Background(), "client", "correct-123"); err != nil {
		t.Fatalf("login after lock expiry: %v", err)
	}
	if limiter.resets == 0 {
		t.Fatal("successful login did not reset limiter")
	}
}

func TestAuthSessionLifecycle(t *testing.T) {
	svc, _, sessions, _ := newAuthForTest(t, "correct-123")
	token, err := svc.Login(context.Background(), "client", "correct-123")
	if err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status(context.Background(), token)
	if err != nil || !status.Authenticated || !status.PasswordConfigured || !status.RequireLogin {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if err := svc.Authenticate(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	status, err = svc.Status(context.Background(), token)
	if err != nil || status.Authenticated {
		t.Fatalf("revoked status=%+v err=%v", status, err)
	}
	if len(sessions.active) != 0 {
		t.Fatal("session store retained revoked session")
	}
}
