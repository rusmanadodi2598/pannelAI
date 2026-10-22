// Package domain holds the shared error envelope and value rules.
//
// @file      internal/domain/errors.go
// @for       The structured error shape every handler may write.
// @uses      fmt.
// @reason    AGENTS.md §1.3 requires one machine-readable code and one English message per client error, so handlers map to a single type instead of inventing payloads.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-21
package domain

// Error is the only shape a handler may write to a client response body.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// NewValidationError reports input the service refused.
func NewValidationError(message string) *Error {
	return &Error{Code: "BOT_VALIDATION", Message: message}
}

// NewInternalError reports a failure whose detail stays in the logs.
func NewInternalError(message string) *Error {
	return &Error{Code: "BOT_INTERNAL", Message: message}
}
