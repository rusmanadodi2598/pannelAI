// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/retry.go
// @for       The retry decision for one upstream target and its backoff.
// @uses      internal/provider, internal/registry, net/http, math/rand/v2, time.
// @reason    SPEC-API-001 §4 fixes the policy (default three attempts on
//
//	429/5xx/network with exponential backoff and jitter, per-provider
//	override, and never more than one retry of a non-idempotent POST).
//	Keeping it a pure function of the attempt, the status, the header,
//	the registry entry, and the plugin is what makes the policy
//	testable per status without a network or a clock, which the
//	acceptance criteria require.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"math"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Retry defaults, applied when a registry entry declares none (SPEC-API-001 §4).
const (
	// DefaultAttempts is the documented `max_attempts: 3` default.
	DefaultAttempts = 3
	// MaxPOSTRetries caps retries of a request that is not idempotent. One: the
	// upstream may already have begun the work the first request paid for, so a
	// second retry would bill twice for one client call.
	MaxPOSTRetries = 1
	// backoffBase is the first backoff window; each further retry doubles it.
	backoffBase = 200 * time.Millisecond
	// backoffCeiling bounds one wait so a long chain cannot park a request for
	// minutes on end.
	backoffCeiling = 5 * time.Second
)

// Attempt describes what one upstream call did, which is all the retry decision
// reads.
type Attempt struct {
	// Retries is how many retries have already been performed for this target.
	Retries int
	// Status is the upstream HTTP status, or 0 when the call produced no
	// response at all (a transport failure or a timeout).
	Status int
	// Header is the upstream response header, for an upstream Retry-After hint.
	Header http.Header
	// Idempotent reports whether repeating the request is safe. A chat
	// completion is not: the upstream may have started generating.
	Idempotent bool
}

// RetryDecision is the answer: whether to try the same target again, and how
// long to wait first.
type RetryDecision struct {
	Retry bool
	After time.Duration
}

// DecideRetry applies the retry policy for one target (SPEC-API-001 §4).
//
// The plugin answers "is this outcome worth retrying at all" and the registry
// entry answers "how many attempts this provider allows", so a provider that
// reports its own transient status and a provider capped at a single attempt both
// work without a branch here. A transport failure presents the transient status
// the plugin classifies, because it carries no status of its own and is exactly
// the case §4 lists alongside 429 and 5xx.
func DecideRetry(entry registry.Provider, plugin provider.Plugin, attempt Attempt) RetryDecision {
	if plugin == nil {
		return RetryDecision{}
	}
	if attempt.Retries >= retryBudget(entry, attempt) {
		return RetryDecision{}
	}

	status := attempt.Status
	if status == 0 {
		status = http.StatusServiceUnavailable
	}
	decision := plugin.ShouldRetry(status, attempt.Header)
	if !decision.Retry {
		return RetryDecision{}
	}
	if decision.After > 0 {
		// An upstream that stated when to come back is obeyed: waiting less
		// re-hits the limit, waiting more idles a slot.
		return RetryDecision{Retry: true, After: decision.After}
	}
	return RetryDecision{Retry: true, After: Backoff(attempt.Retries)}
}

// retryBudget reports how many retries this attempt is allowed.
//
// `Attempts` is a total attempt count, so one attempt means zero retries: that is
// what lets an entry declare `retry: 1` as "do not retry". An entry that declares
// nothing gets the documented default of three attempts, and a non-idempotent
// request is capped below whatever the entry allows.
func retryBudget(entry registry.Provider, attempt Attempt) int {
	attempts := entry.Transport.Retry.Attempts(attempt.Status)
	if attempts <= 0 {
		attempts = entry.Transport.Retry.DefaultAttempts
	}
	if attempts <= 0 {
		attempts = DefaultAttempts
	}
	budget := attempts - 1
	if !attempt.Idempotent && budget > MaxPOSTRetries {
		budget = MaxPOSTRetries
	}
	if budget < 0 {
		return 0
	}
	return budget
}

// Backoff returns the wait before retry number `retries` (zero-based), growing
// exponentially from backoffBase and capped at backoffCeiling.
//
// Full jitter rather than a fixed step: a fleet retrying a shared upstream in
// lockstep re-creates the overload it is backing off from, and drawing the wait
// uniformly from the whole window is what spreads those retries out. The
// function is exported so a test can assert the window rather than a wall-clock
// value.
func Backoff(retries int) time.Duration {
	if retries < 0 {
		retries = 0
	}
	window := float64(backoffBase) * math.Pow(2, float64(retries))
	if window > float64(backoffCeiling) {
		window = float64(backoffCeiling)
	}
	if window <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(window) + 1)) //nolint:gosec // jitter, not a secret
}

// RetryableStatuses lists the statuses §4 names as retryable, so a test can
// assert the policy against the documented set instead of against the plugin's
// own answer.
func RetryableStatuses() []int {
	return []int{
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}
}
