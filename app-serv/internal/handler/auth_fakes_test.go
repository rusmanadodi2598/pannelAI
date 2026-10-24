// Package handler tests dashboard authentication HTTP contracts.
//
// @file      internal/handler/auth_fakes_test.go
// @for       In-memory auth repository, session store, and limiter used by handler tests.
// @uses      context, sync, testing, time, internal/domain/repository/service.
// @reason    Handler tests need deterministic, epoch-aware state without a live Redis or PostgreSQL.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-24
package handler

import (
	"context"
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
	epoch  int64
	active map[string]int64
}

func newHandlerSessionStore() *handlerSessionStore {
	return &handlerSessionStore{active: make(map[string]int64)}
}
func (s *handlerSessionStore) Create(_ context.Context, digest string, _ time.Duration, epoch int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[digest] = epoch
	return nil
}
func (s *handlerSessionStore) Epoch(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch, nil
}
func (s *handlerSessionStore) Exists(_ context.Context, digest string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	epoch, ok := s.active[digest]
	return ok && epoch == s.epoch, nil
}
func (s *handlerSessionStore) Revoke(_ context.Context, digest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, digest)
	return nil
}
func (s *handlerSessionStore) RevokeAll(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch++
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
	return NewAuthHandler(svc, SessionCookieOptions{TTL: time.Hour}, nil), repo
}

var _ repository.AuthRepository = (*handlerAuthRepo)(nil)
