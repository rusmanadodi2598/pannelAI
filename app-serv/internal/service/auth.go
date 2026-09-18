// Package service orchestrates app-serv use cases without HTTP concerns.
//
// @file      internal/service/auth.go
// @for       Dashboard login, logout, status, bootstrap, and password changes.
// @uses      internal/domain, internal/repository, crypto password hashing.
// @reason    SPEC-API-001 §7.2 requires revocable dashboard sessions and a
//
//	fail-closed password lifecycle over PostgreSQL and Redis.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// AuthStatus is the service-layer projection of dashboard session state.
type AuthStatus struct {
	Authenticated      bool
	RequireLogin       bool
	PasswordConfigured bool
}

// AuthService owns dashboard authentication and session orchestration.
type AuthService struct {
	repo              repository.AuthRepository
	sessions          repository.SessionStore
	limiter           repository.LoginLimiter
	hasher            PasswordHasher
	secret            []byte
	sessionTTL        time.Duration
	loginMaxFails     int
	loginLockout      time.Duration
	bootstrapPassword string
}

// AuthServiceDeps supplies persistence, crypto, and validated runtime settings.
type AuthServiceDeps struct {
	Repo              repository.AuthRepository
	Sessions          repository.SessionStore
	Limiter           repository.LoginLimiter
	Hasher            PasswordHasher
	Secret            []byte
	SessionTTL        time.Duration
	LoginMaxFails     int
	LoginLockout      time.Duration
	BootstrapPassword string
}

// NewAuthService validates dependencies and creates the auth use-case service.
func NewAuthService(deps AuthServiceDeps) (*AuthService, error) {
	if deps.Repo == nil || deps.Sessions == nil || deps.Limiter == nil || deps.Hasher == nil {
		return nil, fmt.Errorf("auth service: required dependency is nil")
	}
	if len(deps.Secret) < 32 || deps.SessionTTL <= 0 || deps.LoginMaxFails < 1 || deps.LoginLockout <= 0 {
		return nil, fmt.Errorf("auth service: invalid security configuration")
	}
	return &AuthService{
		repo: deps.Repo, sessions: deps.Sessions, limiter: deps.Limiter, hasher: deps.Hasher,
		secret: append([]byte(nil), deps.Secret...), sessionTTL: deps.SessionTTL,
		loginMaxFails: deps.LoginMaxFails, loginLockout: deps.LoginLockout,
		bootstrapPassword: deps.BootstrapPassword,
	}, nil
}

// Bootstrap stores the configured first password without overwriting an existing hash.
func (s *AuthService) Bootstrap(ctx context.Context) error {
	if s.bootstrapPassword == "" {
		return nil
	}
	existing, err := s.repo.PasswordHash(ctx)
	if err != nil {
		return fmt.Errorf("checking bootstrap state: %w", err)
	}
	if existing != "" {
		return nil
	}
	hash, err := s.hasher.Hash(s.bootstrapPassword)
	if err != nil {
		return fmt.Errorf("hashing bootstrap password: %w", err)
	}
	if _, err := s.repo.BootstrapPassword(ctx, hash); err != nil {
		return fmt.Errorf("bootstrapping password: %w", err)
	}
	return nil
}

// Status reports password setup and whether the supplied session remains active.
func (s *AuthService) Status(ctx context.Context, token string) (AuthStatus, error) {
	hash, err := s.repo.PasswordHash(ctx)
	if err != nil {
		return AuthStatus{}, fmt.Errorf("reading auth status: %w", err)
	}
	status := AuthStatus{RequireLogin: true, PasswordConfigured: hash != ""}
	if token == "" {
		return status, nil
	}
	digest, parseErr := domain.ParseSessionToken(token, s.secret)
	if parseErr == nil {
		active, err := s.sessions.Exists(ctx, digest)
		if err != nil {
			return AuthStatus{}, fmt.Errorf("checking session status: %w", err)
		}
		status.Authenticated = active
	}
	return status, nil
}

// Login verifies the password and creates a Redis-tracked signed session token.
func (s *AuthService) Login(ctx context.Context, clientKey, password string) (string, error) {
	if remaining, err := s.limiter.Locked(ctx, clientKey); err != nil {
		return "", fmt.Errorf("checking login lockout: %w", err)
	} else if remaining > 0 {
		return "", rateLimitError(remaining)
	}
	hash, err := s.repo.PasswordHash(ctx)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	if hash == "" {
		return "", domain.ErrPasswordNotConfigured
	}
	if err := s.hasher.Compare(hash, password); err != nil {
		lockedFor, recordErr := s.limiter.RecordFailure(ctx, clientKey, s.loginMaxFails, s.loginLockout)
		if recordErr != nil {
			return "", fmt.Errorf("recording login failure: %w", recordErr)
		}
		if lockedFor > 0 {
			return "", rateLimitError(lockedFor)
		}
		return "", domain.ErrInvalidCredentials
	}
	if err := s.limiter.Reset(ctx, clientKey); err != nil {
		return "", fmt.Errorf("resetting login limiter: %w", err)
	}
	token, digest, err := domain.NewSessionToken(s.secret)
	if err != nil {
		return "", fmt.Errorf("creating session token: %w", err)
	}
	if err := s.sessions.Create(ctx, digest, s.sessionTTL); err != nil {
		return "", fmt.Errorf("storing session: %w", err)
	}
	return token, nil
}

func rateLimitError(remaining time.Duration) error {
	return domain.NewRateLimitedAfter("too many attempts; try again later", remaining)
}
