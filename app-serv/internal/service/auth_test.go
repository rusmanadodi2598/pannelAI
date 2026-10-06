// Package service tests dashboard authentication use cases.
//
// @file      internal/service/auth_test.go
// @for       The password, session and limiter doubles every auth test is built on.
// @uses      context, sync, testing, time, internal/domain, internal/repository.
// @reason    AuthService reads three ports and nothing else; stubbing them once here lets the flows in auth_flows_test.go be driven without a database, a session store, or a clock that waits for a lockout.
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
func (s *sessionFake) RevokeAll(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = make(map[string]bool)
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
