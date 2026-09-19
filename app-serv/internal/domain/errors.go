// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/errors.go
// @for       The AppError envelope and the sentinel errors layers match on.
// @uses      errors, standard library only.
// @reason    SPEC-API-001 §8 fixes the management error shape (code + English
//
//	message) and its HTTP mapping; centralizing it here keeps handlers
//	from inventing codes and keeps driver messages off the wire
//	(AGENTS.md §1.3).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-16
package domain

import (
	"errors"
	"time"
)

// AppError is the management-plane error contract (SPEC-API-001 §8).
// Code is machine-parseable; Message is always English (AGENTS.md §1.3).
type AppError struct {
	Code       string
	Message    string
	Wrapped    error
	RetryAfter time.Duration
}

func (e *AppError) Error() string {
	if e.Wrapped != nil {
		return e.Code + ": " + e.Message + ": " + e.Wrapped.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Wrapped }

// HTTPStatus maps an error code to the status documented in §8.
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case "VALIDATION_ERROR":
		return 400
	case "UNAUTHORIZED":
		return 401
	case "FORBIDDEN":
		return 403
	case "NOT_FOUND":
		return 404
	case "CONFLICT":
		return 409
	case "METHOD_NOT_ALLOWED":
		return 405
	case "RATE_LIMITED":
		return 429
	case "NO_PROVIDER_AVAILABLE":
		return 503
	case "UPSTREAM_ERROR":
		return 502
	case "UPSTREAM_TIMEOUT":
		return 504
	default:
		return 500
	}
}

func NewValidationError(message string) *AppError {
	return &AppError{Code: "VALIDATION_ERROR", Message: message}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{Code: "UNAUTHORIZED", Message: message}
}

func NewForbiddenError(message string) *AppError {
	return &AppError{Code: "FORBIDDEN", Message: message}
}

func NewNotFoundError(message string) *AppError {
	return &AppError{Code: "NOT_FOUND", Message: message}
}

func NewConflictError(message string) *AppError {
	return &AppError{Code: "CONFLICT", Message: message}
}

func NewRateLimitedError(message string) *AppError {
	return &AppError{Code: "RATE_LIMITED", Message: message}
}

func NewNoProviderAvailableError(message string) *AppError {
	return &AppError{Code: "NO_PROVIDER_AVAILABLE", Message: message}
}

func NewUpstreamError(message string) *AppError {
	return &AppError{Code: "UPSTREAM_ERROR", Message: message}
}

func NewUpstreamTimeoutError(message string) *AppError {
	return &AppError{Code: "UPSTREAM_TIMEOUT", Message: message}
}

func NewInternalError(message string) *AppError {
	return &AppError{Code: "INTERNAL_ERROR", Message: message}
}

// Sentinel errors let layers match on identity without re-declaring codes.
var (
	ErrGatewayKeyNotFound = NewNotFoundError("gateway key not found")
	ErrGatewayKeyExists   = NewConflictError("gateway key with this name already exists")
	ErrGatewayKeyRevoked  = NewConflictError("gateway key is already revoked")
	ErrUnauthenticated    = NewUnauthorizedError("authentication is required")
	ErrForbidden          = NewForbiddenError("access is denied")

	ErrEndpointNotFound = NewNotFoundError("upstream endpoint not found")
	ErrEndpointExists   = NewConflictError("an endpoint with this label already exists for this provider")
	ErrNodeNotFound     = NewNotFoundError("provider node not found")
	ErrNodePrefixTaken  = NewConflictError("this prefix is already in use")
	ErrNodeInUse        = NewConflictError("an endpoint still references this provider")

	ErrProxyNotFound = NewNotFoundError("proxy not found")
)

// AsAppError unwraps any error into an AppError, mapping unknown failures to
// INTERNAL_ERROR so handlers never leak raw driver messages to clients.
func AsAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return NewInternalError("an unexpected error occurred")
}
