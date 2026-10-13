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
	// KeyFailureAuth covers 401/402/403: the credential or its account.
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
)

// ImplicatesCredential reports whether a failure of this class says anything
// about the credential that served it. A new class opts in here rather than by
// being absent from a blacklist, because the default for a class nobody thought
// about is to leave the credential alone.
func (c KeyFailureClass) ImplicatesCredential() bool {
	return c == KeyFailureAuth || c == KeyFailureRateLimit || c == KeyFailureTransient
}

// keyTransientCooldown is how long an outage-shaped failure parks a key: long
// enough to skip the overloaded instance, short enough that a recovered one is
// picked up by the next request. It is flat on purpose: a target that answered
// 5xx a moment ago is not a credential that is dead, and only the latter earns
// an escalating window (draft 050 D3a).
const keyTransientCooldown = 30 * time.Second

// The two doubling ladders. The rate-limit ladder is the reference's
// BACKOFF_CONFIG; the credential ladder is draft 050 T5, added because a dead
// credential was parked for the same flat 2 minutes forever and re-spent on
// every round.
const (
	keyAuthBase      = 2 * time.Minute
	keyAuthMax       = time.Hour
	keyRateLimitBase = 2 * time.Second
	keyRateLimitMax  = 5 * time.Minute
	keyLadderMaxN    = 15
)

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
func (k *UpstreamKey) RecordFailure(reason string, class KeyFailureClass, now time.Time) {
	k.lastError = reason
	k.updatedAt = now
	if !class.ImplicatesCredential() {
		return
	}
	k.consecutiveErrors++
	window := keyTransientCooldown
	switch class {
	case KeyFailureAuth:
		window = backoff(keyAuthBase, k.consecutiveErrors, keyAuthMax)
	case KeyFailureRateLimit:
		window = backoff(keyRateLimitBase, k.consecutiveErrors, keyRateLimitMax)
	}
	k.status = UpstreamKeyError
	until := now.Add(window)
	k.rateLimitedUntil = &until
}

// RecordSuccess clears the health state after a request the upstream accepted:
// the counter, the recorded error, and the park all reset, and a key that was
// parked returns to active.
func (k *UpstreamKey) RecordSuccess(now time.Time) {
	k.consecutiveErrors = 0
	k.lastError = ""
	k.rateLimitedUntil = nil
	if k.status == UpstreamKeyError {
		k.status = UpstreamKeyActive
	}
	used := now
	k.lastUsedAt = &used
	k.updatedAt = now
}
