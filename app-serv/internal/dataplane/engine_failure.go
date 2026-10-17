// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_failure.go
// @for       How one upstream failure is classified, recorded against the key's parked-or-usable state, and reported to the client.
// @uses      internal/domain, internal/provider, net/http, context, time.
// @reason    SPEC-API-001 §7.5 makes the health state a consequence of every call, and §7.7 fixes which failures are worth another credential and which stop the chain. Both are about one failure rather than about the pipeline, so they live together here and the relay loop stays readable.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package dataplane

import (
	"context"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// FailureClass maps an upstream HTTP status onto the key-health class that
// decides its parking window: the credential's own failures, rate limits, billing,
// and transient outages implicate the key, while a 404 is this target not serving
// the model and every other 4xx is the request itself.
func FailureClass(status int) domain.KeyFailureClass {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.KeyFailureAuth
	case http.StatusPaymentRequired:
		// Not Auth: the credential is valid and the account behind it has no credit,
		// a hours-long condition an operator fixes by buying credit.
		return domain.KeyFailureBilling
	case http.StatusTooManyRequests:
		return domain.KeyFailureRateLimit
	case http.StatusNotFound:
		// Not Auth: another credential on the same body may well serve this
		// model, so the chain must keep walking while the key stays clean.
		return domain.KeyFailureTarget
	}
	if status >= 400 && status < 500 {
		return domain.KeyFailureRequest
	}
	return domain.KeyFailureTransient
}

// failureClass maps a call failure onto its class. An upstream rejection
// carries the status, and the class the transport already recognised where it
// recognised one; a failure that never produced an upstream answer never reached
// the credential, so it cannot say anything about it.
func failureClass(err error) domain.KeyFailureClass {
	if failure, ok := AsUpstreamError(err); ok {
		if failure.Class != "" {
			return failure.Class.KeyClass()
		}
		return FailureClass(failure.Status)
	}
	return domain.KeyFailureTarget
}

// failureRest is what the classified failure tells the health model: the wait the
// vendor named, the reset it stated, and which signature recognised it. A call
// that never reached the upstream is a target failure and parks nothing, which
// is what it did before this classification existed.
func failureRest(err error) domain.RestDirective {
	failure, ok := AsUpstreamError(err)
	if !ok {
		return domain.RestDirective{Class: domain.FailureTargetUnreachable, Source: domain.ResetSourceDefault}
	}
	return failure.Rest()
}

// failureReason is the text recorded against the key: the upstream's own
// message when there is one, so the panel shows why the credential parked.
func failureReason(err error) string {
	if failure, ok := AsUpstreamError(err); ok {
		return failure.Message
	}
	return "upstream call failed"
}

// failureRetryAfter is the wait the vendor named on this response, or zero: two 429s
// share a class and rarely a wait, so the header is read rather than inferred.
func failureRetryAfter(err error) time.Duration {
	failure, ok := AsUpstreamError(err)
	if !ok {
		return 0
	}
	return provider.RetryAfter(failure.Header)
}

// recordFailure applies an upstream failure to the key's health and answers what
// the walk does next. A persistence failure is swallowed deliberately: the
// client's error is the upstream failure, and reporting a bookkeeping failure
// instead would hide the cause it came from.
//
// A caller that hung up writes nothing: the read died because the request died, and
// recording it would let one client closing a tab park an account for everyone else.
// It answers no action, leaving the walk to the code's own rule.
func (e *Engine) recordFailure(ctx context.Context, selection Selection, cause error) domain.FailureAction {
	if isClientCancellation(cause) {
		return ""
	}
	action, err := e.selector.RecordFailure(ctx, selection, failureReason(cause),
		failureClass(cause), failureRest(cause))
	if err != nil {
		logx.Degraded(e.logger, "key health could not be recorded after an upstream failure", err,
			"endpoint_id", selection.Endpoint.ID(), "key_id", selection.Key.ID(),
			"upstream_error", failureReason(cause))
	}
	return action
}

// keepsWalking reports whether the chain spends another credential on a failed
// call. The classified failure answers first, so an operator who set a class to
// stop is not walked over: stop ends the chain, rotate keeps it going whatever the
// code says. A call with nothing classified (a network failure, a client that hung
// up) falls back to the code's own rule, which is what decided the walk before the
// classes existed.
func keepsWalking(action domain.FailureAction, code string) bool {
	switch action {
	case domain.FailureActionStop:
		return false
	case domain.FailureActionRotate:
		return true
	default:
		return failoverWorthy(code)
	}
}

// translateCallError maps a transport failure onto the client-visible code,
// following the class the refusal was recognised as rather than the status that
// carried it: a timeout is UPSTREAM_TIMEOUT, a rate limit is RATE_LIMITED, a
// request-shaped refusal is UPSTREAM_REJECTED, and every other upstream failure
// is UPSTREAM_ERROR.
func (e *Engine) translateCallError(err error) error {
	if failure, ok := AsUpstreamError(err); ok {
		code := CodeUpstreamError
		class := failureClass(err)
		switch class {
		case domain.KeyFailureQuota:
			// A spent allowance is reported as the account's own condition and
			// not as the throttle its status claims: a client that backs off for
			// five minutes and retries is spending a request the vendor has
			// already refused for the rest of the period.
			code = CodeBillingExhausted
		case domain.KeyFailureRateLimit:
			// The class decides what the client is told: Qoder wraps its queue in a
			// 403, and a 403 would tell the client its credential broke.
			code = CodeRateLimited
		case domain.KeyFailureBilling:
			// Named rather than folded into UPSTREAM_ERROR, so a client's retry policy
			// and the panel both see nothing can serve until credit is bought.
			code = CodeBillingExhausted
		case domain.KeyFailureRequest:
			// The upstream refused this request itself (context overflow, an
			// unsupported parameter): the same body would be refused by every
			// other credential, so the chain stops and the client sees the
			// refusal.
			code = CodeUpstreamRejected
		}
		failureErr := wrapDataPlaneError(code, failure.Message, err)
		// The vendor's wait is handed on, bounded by the same ceiling the park
		// uses: a client told to come back in a day by a gateway that parks the
		// credential for five minutes is being lied to twice about one number.
		if wait := failureRetryAfter(err); wait > 0 {
			failureErr.RetryAfter = clampRetrySeconds(wait)
		}
		return failureErr
	}
	return err
}

// clampRetrySeconds is the seconds a client is told to wait, never past the ceiling
// the parking ladder stops at, so the advice and the real park cannot drift apart.
func clampRetrySeconds(wait time.Duration) int {
	ceiling := int(domain.RateLimitCeiling / time.Second)
	seconds := int(wait.Seconds())
	if seconds > ceiling {
		return ceiling
	}
	return seconds
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
// only an upstream-side failure is worth another attempt. A bill exhausted on this
// account says nothing about the next one, so BILLING_EXHAUSTED keeps walking too.
func failoverWorthy(code string) bool {
	switch code {
	case CodeUpstreamError, CodeUpstreamTimeout, CodeRateLimited, CodeNoProvider, CodeBillingExhausted:
		return true
	default:
		return false
	}
}
