// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_health.go
// @for       The parked-or-usable state of an upstream key: which failures park it, for how long, and what a success resets.
// @uses      time.
// @reason    SPEC-API-001 §7.5 defines the health model and the router's correctness depends on it: parking a failing key is what makes failover work, so the classes, the windows, and the reset live together here rather than at each call site.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-17
package domain

import "time"

// KeyFailureClass is the class an upstream failure belongs to. The class decides
// how long the key is parked (SPEC-API-001 §7.5), following the reference's
// classification: a credential-shaped failure parks long, a rate limit backs
// off exponentially, a transient outage parks briefly, and a failure the
// credential did not cause parks nothing.
type KeyFailureClass int

const (
	// KeyFailureAuth covers 401/403: the credential itself was refused.
	KeyFailureAuth KeyFailureClass = iota
	// KeyFailureRateLimit covers 429.
	KeyFailureRateLimit
	// KeyFailureTransient covers an upstream 5xx: the target is unhealthy.
	KeyFailureTransient
	// KeyFailureRequest covers every other 4xx: the request is the cause.
	KeyFailureRequest
	// KeyFailureTarget covers an upstream 404 and a call that never reached the
	// upstream: the target or the network caused it, and another credential on
	// the same body could still succeed.
	KeyFailureTarget
	// KeyFailureBilling covers an upstream 402: the credential is valid and the
	// account behind it has no credit, a condition measured in hours rather than
	// in minutes, and one the operator fixes by topping the account up.
	KeyFailureBilling
)

// ImplicatesCredential reports whether a failure of this class says anything
// about the credential that served it. A new class opts in here rather than by
// being absent from a blacklist, because the default for a class nobody thought
// about is to leave the credential alone. A bill does implicate it: the account
// behind that key cannot serve, so the key leaves rotation on its own ladder.
func (c KeyFailureClass) ImplicatesCredential() bool {
	return c == KeyFailureAuth || c == KeyFailureRateLimit || c == KeyFailureTransient || c == KeyFailureBilling
}

// String names the class for a store that has to route one increment by it, and
// for a log line. The values are stable: the key-health statement compares
// against them, so a rename here is a migration, not a cosmetic edit.
func (c KeyFailureClass) String() string {
	switch c {
	case KeyFailureAuth:
		return "auth"
	case KeyFailureRateLimit:
		return "ratelimit"
	case KeyFailureTransient:
		return "transient"
	case KeyFailureRequest:
		return "request"
	case KeyFailureTarget:
		return "target"
	case KeyFailureBilling:
		return "billing"
	default:
		return "unknown"
	}
}

// keyTransientCooldown is the §7.5 flat window for an outage-shaped failure: long
// enough to skip the overloaded instance, short enough that a recovered one is
// picked up by the next request. It stays flat because a target that answered 5xx
// a moment ago is not a credential that is dead, and only the latter earns an
// escalating window (draft 050 D3a).
const keyTransientCooldown = 30 * time.Second

// The documented default of each ladder and its ceiling. The bases are what the
// routing group may retune (KeyParkWindows); the ceilings are not, because a park
// that can be configured never to expire buries an account nothing then heals.
const (
	keyAuthBase      = 2 * time.Minute
	keyAuthMax       = time.Hour
	keyRateLimitBase = 2 * time.Second
	keyRateLimitMax  = 5 * time.Minute
	keyBillingBase   = 15 * time.Minute
	keyBillingMax    = 6 * time.Hour
	keyLadderMaxN    = 15
)

// RateLimitCeiling is the §7.5 ceiling the rate-limit ladder stops at. The wait a client is
// told to wait is bounded by the same number the credential is parked for: a gateway saying
// "come back in a day" that returns the key to rotation in five minutes is lying about its
// own health model.
const RateLimitCeiling = keyRateLimitMax

// backoff reports the window the n-th consecutive failure of a ladder parks a
// key for. The shift is guarded: a level past the cap, or a shift that
// overflows the duration, answers the cap rather than a negative window.
func backoff(base time.Duration, n int, ceiling time.Duration) time.Duration {
	if n < 1 {
		n = 1
	}
	if n > keyLadderMaxN {
		n = keyLadderMaxN
	}
	window := base << uint(n-1)
	if window <= 0 || window > ceiling {
		return ceiling
	}
	return window
}

// Available reports whether the router may pick this key at the given instant.
//
// A parked key becomes usable again once its window expires, without an
// explicit reset: the window *is* the state, so nothing has to sweep expired
// parks, and a key that recovers on its own is picked up by the next request
// rather than sitting idle until a worker notices.
//
// A disabled key stays unavailable regardless of the window, because that state
// comes from an operator rather than from health tracking.
func (k UpstreamKey) Available(now time.Time) bool {
	if k.status == UpstreamKeyDisabled {
		return false
	}
	if k.rateLimitedUntil == nil {
		return k.status == UpstreamKeyActive
	}
	return !now.Before(*k.rateLimitedUntil)
}

// RecordFailure applies one upstream failure. The first failure parks the key
// for its class's window, so a dead credential stops serving immediately
// instead of after three strikes. A failure the credential did not cause records
// the reason but parks nothing, because the key is not what failed. The
// credential and rate-limit windows grow with the streak and stop at their
// ceilings; the transient one stays flat.
//
// The streak is the class's own run, not the total: one shared counter let a run of 429s
// raise the credential ladder, so a throttled account was parked for strikes it never earned.
func (k *UpstreamKey) RecordFailure(reason string, class KeyFailureClass, policy KeyHealthPolicy, now time.Time) {
	k.lastError = reason
	k.lastErrorClass = class.String()
	k.updatedAt = now
	if !class.ImplicatesCredential() {
		return
	}
	k.consecutiveErrors++
	k.bumpRun(class)
	window := policy.WindowFor(class, k.runFor(class))
	k.status = UpstreamKeyError
	until := now.Add(window)
	k.rateLimitedUntil = &until
}

// bumpRun advances the streak of the class that just failed. Transient stays outside the
// set because its window is flat: counting a target's 5xx run would escalate a window the
// ladder exists to keep fixed (draft 050 D3a).
func (k *UpstreamKey) bumpRun(class KeyFailureClass) {
	switch class {
	case KeyFailureAuth:
		k.consecutiveAuth++
	case KeyFailureRateLimit:
		k.consecutiveRateLimit++
	case KeyFailureBilling:
		k.consecutiveBilling++
	}
}

// runFor reports the streak a failure of this class parks against. Auth, rate limit and
// billing keep their own; every other class answers the total, whose window is flat anyway.
func (k UpstreamKey) runFor(class KeyFailureClass) int {
	switch class {
	case KeyFailureAuth:
		return k.consecutiveAuth
	case KeyFailureRateLimit:
		return k.consecutiveRateLimit
	case KeyFailureBilling:
		return k.consecutiveBilling
	default:
		return k.consecutiveErrors
	}
}

// RunFor reports the streak of one class, for the persistence layer and the panel.
func (k UpstreamKey) RunFor(class KeyFailureClass) int { return k.runFor(class) }

// RecordSuccess clears the health state after a request the upstream accepted:
// the counter, the recorded error, and the park all reset, and a key that was
// parked returns to active.
//
// Every class run resets, not only the served one: a credential that just answered is
// demonstrably whole, and a stale run would park its next failure at the old length.
func (k *UpstreamKey) RecordSuccess(now time.Time) {
	k.consecutiveErrors = 0
	k.consecutiveAuth = 0
	k.consecutiveRateLimit = 0
	k.consecutiveBilling = 0
	k.lastError = ""
	k.lastErrorClass = ""
	k.rateLimitedUntil = nil
	if k.status == UpstreamKeyError {
		k.status = UpstreamKeyActive
	}
	used := now
	k.lastUsedAt = &used
	k.updatedAt = now
}
