// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/auth.go
// @for       Password, session, and rate-limit persistence boundaries.
// @uses      context and time from the standard library.
// @reason    Auth orchestration must remain independent of PostgreSQL and Redis
//
//	implementations while preserving bounded external calls.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-17
package repository

import (
	"context"
	"time"
)

// AuthRepository stores the singleton dashboard password hash.
type AuthRepository interface {
	PasswordHash(ctx context.Context) (string, error)
	BootstrapPassword(ctx context.Context, hash string) (bool, error)
	ChangePassword(ctx context.Context, expectedHash, newHash string) error
}

// SessionStore tracks revocable dashboard sessions.
type SessionStore interface {
	Create(ctx context.Context, digest string, ttl time.Duration) error
	Exists(ctx context.Context, digest string) (bool, error)
	Revoke(ctx context.Context, digest string) error
}

// LoginLimiter tracks failed attempts and temporary lockouts per client.
type LoginLimiter interface {
	Locked(ctx context.Context, clientKey string) (time.Duration, error)
	RecordFailure(ctx context.Context, clientKey string, maxFailures int, lockout time.Duration) (time.Duration, error)
	Reset(ctx context.Context, clientKey string) error
}

// RateLimiter enforces the configured gateway request budget.
type RateLimiter interface {
	Allow(ctx context.Context, clientKey string, limit int, window time.Duration) (time.Duration, error)
}
