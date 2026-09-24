// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/exports.go
// @for       The exported error constructors and engine accessors a caller in
//
//	another package needs.
//
// @uses      internal/domain.
// @reason    The service layer reports data plane failures with the data plane's
//
//	own codes rather than the management vocabulary (SPEC-API-001 §4,
//	§8), so the constructors are exported instead of the callers
//	re-declaring a code string. The accessors expose the collaborators the
//	engine owns so a second use case (embeddings) reuses the same selector
//	and the same health bookkeeping instead of a parallel one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ValidationError reports a request the data plane cannot serve because of its
// content, using the validation code rather than the management one.
func ValidationError(message string) error {
	return dataPlaneError(CodeValidation, message)
}

// ProviderNotRoutable reports a provider whose wire format the gateway does not
// translate (SPEC-API-001 §8).
func ProviderNotRoutable(message string) error {
	return dataPlaneError(CodeProviderNotRoutable, message)
}

// UpstreamRejected reports a non-2xx answer from a media upstream, keeping the
// status class: a 429 is RATE_LIMITED and anything else is UPSTREAM_ERROR.
func UpstreamRejected(status int, message string) error {
	code := CodeUpstreamError
	if status == 429 {
		code = CodeRateLimited
	}
	return wrapDataPlaneError(code, message, nil)
}

// InternalError reports a failure the client cannot act on.
func InternalError(message string, cause error) error { return internalError(message, cause) }

// Selector exposes the selector, so another use case routes through the same
// ordering and the same circuit state the chat plane uses.
func (e *Engine) Selector() *Selector { return e.selector }

// RecordSuccess applies a served call to a key's health, through the engine so a
// caller does not reach into the selector.
func (e *Engine) RecordSuccess(ctx context.Context, selection Selection) error {
	return e.selector.RecordSuccess(ctx, selection)
}

// RecordFailure applies a failed call to a key's health.
func (e *Engine) RecordFailure(ctx context.Context, selection Selection, reason string, class domain.KeyFailureClass) error {
	return e.selector.RecordFailure(ctx, selection, reason, class)
}

// ErrUnauthorized is the data plane's answer to a missing or invalid credential,
// so a caller outside this package does not construct the code itself.
func ErrUnauthorized() error {
	return dataPlaneError(CodeUnauthorized, "a valid gateway key is required")
}

// ErrNotFound is the data plane's MODEL_NOT_FOUND answer.
func ErrNotFound(message string) error { return dataPlaneError(CodeModelNotFound, message) }
