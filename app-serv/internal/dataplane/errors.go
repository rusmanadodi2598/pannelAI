// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/errors.go
// @for       The data plane error vocabulary and its OpenAI-envelope status.
// @uses      internal/domain.
// @reason    SPEC-API-001 §8 makes PROVIDER_NOT_ROUTABLE a data-plane code and §4
//
//	fixes the envelope: a CLI tool reads the OpenAI shape, not the
//	management one. The management AppError maps a closed code set to
//	statuses, and these codes are deliberately outside it, so the data
//	plane carries its own small type rather than widening a contract the
//	panel depends on.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"errors"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Data plane error codes. The first five are the OpenAI-compatible names a CLI
// tool already branches on; the last two are the named cases SPEC-API-001 §8
// adds so an unanswerable provider is not reported as an upstream outage.
const (
	CodeValidation          = "VALIDATION_ERROR"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeRateLimited         = "RATE_LIMITED"
	CodeInternal            = "INTERNAL_ERROR"
	CodeNoProvider          = "NO_PROVIDER_AVAILABLE"
	CodeUpstreamError       = "UPSTREAM_ERROR"
	CodeUpstreamTimeout     = "UPSTREAM_TIMEOUT"
	CodeModelNotFound       = "MODEL_NOT_FOUND"
	CodeProviderNotRoutable = "PROVIDER_NOT_ROUTABLE"
)

// Error is a data plane failure: the machine code, the English message, and the
// HTTP status that code answers with.
type Error struct {
	Code    string
	Message string
	Status  int
	// Type is the OpenAI error `type` field, which clients group on.
	Type    string
	Wrapped error
	// RetryAfter, when positive, is sent as the Retry-After header.
	RetryAfter int
}

// Error renders the code, the message, and the wrapped cause for logs. The
// wrapped chain never reaches a client (AGENTS.md §1.3).
func (e *Error) Error() string {
	if e.Wrapped != nil {
		return e.Code + ": " + e.Message + ": " + e.Wrapped.Error()
	}
	return e.Code + ": " + e.Message
}

// Unwrap exposes the cause to errors.Is/errors.As without putting it on the wire.
func (e *Error) Unwrap() error { return e.Wrapped }

// OpenAIStatus returns the HTTP status the OpenAI envelope is written with.
func (e *Error) OpenAIStatus() int {
	if e.Status > 0 {
		return e.Status
	}
	return http.StatusInternalServerError
}

// dataPlaneError builds an error from the code's documented status and type, so
// no call site picks a status by hand.
func dataPlaneError(code, message string) *Error {
	return &Error{Code: code, Message: message, Status: statusFor(code), Type: typeFor(code)}
}

// wrapDataPlaneError attaches a cause, keeping the code and status.
func wrapDataPlaneError(code, message string, cause error) *Error {
	failure := dataPlaneError(code, message)
	failure.Wrapped = cause
	return failure
}

// statusFor maps a data plane code to its HTTP status.
//
// MODEL_NOT_FOUND is 404 and PROVIDER_NOT_ROUTABLE is 400: the spec fixes the
// first through the §7.15 resolution order and names the second as "actionable
// rather than a 502 that reads like an upstream outage". A request naming a
// provider whose protocol the gateway does not translate is a request-level
// defect the client can fix by naming another provider, which is what 400 says;
// 503 would tell the client to retry something that can never succeed.
func statusFor(code string) int {
	switch code {
	case CodeValidation, CodeModelNotFound, CodeProviderNotRoutable:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeNoProvider:
		return http.StatusServiceUnavailable
	case CodeUpstreamError:
		return http.StatusBadGateway
	case CodeUpstreamTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// typeFor maps a data plane code to the OpenAI error `type`, following the
// reference's client-facing classification so a client's existing retry logic
// keeps working.
func typeFor(code string) string {
	switch code {
	case CodeValidation, CodeModelNotFound, CodeProviderNotRoutable:
		return "invalid_request_error"
	case CodeUnauthorized:
		return "authentication_error"
	case CodeRateLimited:
		return "rate_limit_error"
	default:
		return "server_error"
	}
}

// AsError converts any error into a data plane error, so the handler always has
// a code and a status to render. An error the data plane never produced is
// reported as INTERNAL_ERROR with its detail kept for the log only.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var failure *Error
	if errors.As(err, &failure) {
		return failure
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return wrapDataPlaneError(codeForAppError(appErr), appErr.Message, err)
	}
	return wrapDataPlaneError(CodeInternal, "an unexpected error occurred", err)
}

// codeForAppError translates the management vocabulary into the data plane one,
// so a shared dependency (a repository, the settings reader) can return its own
// error type without the data plane leaking a management code to a CLI tool.
func codeForAppError(appErr *domain.AppError) string {
	switch appErr.Code {
	case "VALIDATION_ERROR":
		return CodeValidation
	case "UNAUTHORIZED", "FORBIDDEN":
		return CodeUnauthorized
	case "NOT_FOUND":
		return CodeModelNotFound
	case "RATE_LIMITED":
		return CodeRateLimited
	case "NO_PROVIDER_AVAILABLE":
		return CodeNoProvider
	case "UPSTREAM_ERROR":
		return CodeUpstreamError
	case "UPSTREAM_TIMEOUT":
		return CodeUpstreamTimeout
	default:
		return CodeInternal
	}
}

// timeoutError reports an outbound call that ran out of time, whether the
// deadline came from the context or from the idle guard closing a stalled
// stream. Both are the same thing to a client: the upstream did not answer.
func timeoutError(cause error) error {
	return wrapDataPlaneError(CodeUpstreamTimeout, "the upstream did not answer in time", cause)
}

// internalError reports a failure the client cannot act on. The cause is kept for
// the log and never rendered.
func internalError(message string, cause error) error {
	if cause == nil {
		return dataPlaneError(CodeInternal, message)
	}
	return wrapDataPlaneError(CodeInternal, message, cause)
}
