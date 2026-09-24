// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_failure.go
// @for       How one upstream failure is classified, recorded against the key's
//
//	parked-or-usable state, and reported to the client.
//
// @uses      internal/domain, net/http, context.
// @reason    SPEC-API-001 §7.5 makes the health state a consequence of every
//
//	call, and §7.7 fixes which failures are worth another credential
//	and which stop the chain. Both are about one failure rather than
//	about the pipeline, so they live together here and the relay loop
//	stays readable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// FailureClass maps an upstream HTTP status onto the key-health class that
// decides its parking window, following the reference's classification: the
// credential's own failures, rate limits, transient outages, and everything
// else the request itself caused.
func FailureClass(status int) domain.KeyFailureClass {
	switch status {
	case http.StatusUnauthorized, http.StatusPaymentRequired,
		http.StatusForbidden, http.StatusNotFound:
		return domain.KeyFailureAuth
	case http.StatusTooManyRequests:
		return domain.KeyFailureRateLimit
	}
	if status >= 400 && status < 500 {
		return domain.KeyFailureRequest
	}
	return domain.KeyFailureTransient
}

// failureClass maps a call failure onto its class. An upstream rejection
// carries the status; a timeout or an unreachable upstream is transient.
func failureClass(err error) domain.KeyFailureClass {
	if failure, ok := AsUpstreamError(err); ok {
		return FailureClass(failure.Status)
	}
	return domain.KeyFailureTransient
}

// failureReason is the text recorded against the key: the upstream's own
// message when there is one, so the panel shows why the credential parked.
func failureReason(err error) string {
	if failure, ok := AsUpstreamError(err); ok {
		return failure.Message
	}
	return "upstream call failed"
}

// recordFailure applies an upstream failure to the key's health. A persistence
// failure is swallowed deliberately: the client's error is the upstream
// failure, and reporting a bookkeeping failure instead would hide the cause it
// came from.
func (e *Engine) recordFailure(ctx context.Context, selection Selection, cause error) {
	_ = e.selector.RecordFailure(ctx, selection, failureReason(cause), failureClass(cause))
}

// translateCallError maps a transport failure onto the client-visible code,
// keeping the upstream's status class: a timeout is UPSTREAM_TIMEOUT, a rate
// limit is RATE_LIMITED, a request-shaped refusal is UPSTREAM_REJECTED, and
// every other upstream failure is UPSTREAM_ERROR.
func (e *Engine) translateCallError(err error) error {
	if failure, ok := AsUpstreamError(err); ok {
		code := CodeUpstreamError
		switch {
		case failure.Status == http.StatusTooManyRequests:
			code = CodeRateLimited
		case FailureClass(failure.Status) == domain.KeyFailureRequest:
			// The upstream refused this request itself (context overflow, an
			// unsupported parameter): the same body would be refused by every
			// other credential, so the chain stops and the client sees the
			// refusal (draft 028 F2).
			code = CodeUpstreamRejected
		}
		return wrapDataPlaneError(code, failure.Message, err)
	}
	if AsError(err).Code == CodeUpstreamTimeout {
		return err
	}
	return err
}

// finalError combines an exhausted chain's failures the way the reference
// reports one: the first failure's status and code, because that is the class
// the client's retry logic saw first, with the last failure's message, because
// it is the most specific cause. A chain whose failures agree returns the last
// error itself, keeping its wrapped cause for the log.
func finalError(first, last error) error {
	head, tail := AsError(first), AsError(last)
	if head.Code == tail.Code && head.Status == tail.Status && head.Message == tail.Message {
		return last
	}
	return &Error{Code: head.Code, Message: tail.Message, Status: head.Status, Type: head.Type, RetryAfter: head.RetryAfter}
}

// failoverWorthy reports whether another credential or combo member could
// plausibly succeed. A request-shaped refusal fails identically everywhere, so
// only an upstream-side failure is worth another attempt.
func failoverWorthy(code string) bool {
	switch code {
	case CodeUpstreamError, CodeUpstreamTimeout, CodeRateLimited, CodeNoProvider:
		return true
	default:
		return false
	}
}
