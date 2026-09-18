// Package domain holds entities, value objects, and gateway business errors.
//
// @file      internal/domain/auth.go
// @for       Authentication and session error vocabulary for the management plane.
// @uses      standard library time and internal domain error constructors.
// @reason    Auth handlers need stable English error codes without exposing
//
//	bcrypt, PostgreSQL, or Redis implementation details.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-17
package domain

import "time"

// ErrPasswordNotConfigured indicates that first-boot password setup is pending.
var ErrPasswordNotConfigured = NewUnauthorizedError("dashboard password is not configured")

// ErrInvalidCredentials avoids revealing whether a password hash exists.
var ErrInvalidCredentials = NewUnauthorizedError("invalid credentials")

// ErrSessionInvalid indicates a missing, malformed, expired, or revoked session.
var ErrSessionInvalid = NewUnauthorizedError("authentication is required")

// ErrPasswordChanged indicates a concurrent password update won the compare-and-set.
var ErrPasswordChanged = NewConflictError("password was changed by another request")

// NewRateLimitedAfter creates a structured rate-limit error with Retry-After metadata.
func NewRateLimitedAfter(message string, retryAfter time.Duration) *AppError {
	err := NewRateLimitedError(message)
	err.RetryAfter = retryAfter
	return err
}
