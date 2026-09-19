// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_failure.go
// @for       How one upstream failure is classified, recorded against the key's
//
//	circuit state, and reported to the client.
//
// @uses      internal/domain, context.
// @reason    SPEC-API-001 §7.5 makes the circuit state a consequence of every
//
//	call, and §8 fixes which failures are worth another account. Both
//	decisions are about one failure rather than about the pipeline, so
//	they live together here and the relay loop stays readable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
)

// recordFailure applies an upstream failure to the key's circuit state. A
// persistence failure is logged by the caller through the returned outcome and
// never replaces the upstream error the client must see.
func (e *Engine) recordFailure(ctx context.Context, selection Selection, cause error) {
	failure, ok := AsUpstreamError(cause)
	reason := "upstream call failed"
	if ok {
		reason = failure.Message
	}
	if err := e.selector.RecordFailure(ctx, selection, reason); err != nil {
		// reason: the client's error is the upstream failure, and reporting a
		// bookkeeping failure instead would hide the cause it came from.
		_ = err
	}
}

// translateCallError maps a transport failure onto the client-visible code,
// keeping the upstream's status class: a timeout is UPSTREAM_TIMEOUT and a
// rejected request is UPSTREAM_ERROR.
func (e *Engine) translateCallError(err error) error {
	if failure, ok := AsUpstreamError(err); ok {
		code := CodeUpstreamError
		if failure.Status == 429 {
			code = CodeRateLimited
		}
		if failure.Status == 401 || failure.Status == 403 {
			code = CodeUpstreamError
		}
		return wrapDataPlaneError(code, failure.Message, err)
	}
	if AsError(err).Code == CodeUpstreamTimeout {
		return err
	}
	return err
}

// failoverWorthy reports whether another combo member could plausibly succeed.
// A malformed request or an unroutable provider fails identically everywhere, so
// only an upstream-side failure is worth another account.
func failoverWorthy(code string) bool {
	switch code {
	case CodeUpstreamError, CodeUpstreamTimeout, CodeRateLimited, CodeNoProvider:
		return true
	default:
		return false
	}
}
