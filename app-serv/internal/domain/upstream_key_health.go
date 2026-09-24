// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_health.go
// @for       The parked-or-usable state of an upstream key: which failures park
//
//	it, for how long, and what a success resets.
//
// @uses      time.
// @reason    SPEC-API-001 §7.5 defines the health model and the router's
//
//	correctness depends on it: parking a failing key is what makes
//	failover work, so the classes, the windows, and the reset live
//	together here rather than at each call site.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import "time"

// KeyFailureClass is the class an upstream failure belongs to. The class decides
// how long the key is parked (SPEC-API-001 §7.5), following the reference's
// classification: a credential-shaped failure parks long, a rate limit backs
// off exponentially, a transient outage parks briefly, and a request-shaped
// refusal parks nothing because the same body would fail identically on any key.
type KeyFailureClass int

const (
	// KeyFailureAuth covers 401/402/403/404: the credential or its account.
	KeyFailureAuth KeyFailureClass = iota
	// KeyFailureRateLimit covers 429.
	KeyFailureRateLimit
	// KeyFailureTransient covers 5xx, network, and timeout failures.
	KeyFailureTransient
	// KeyFailureRequest covers every other 4xx: the request is the cause.
	KeyFailureRequest
)

// keyAuthCooldown is how long a credential-shaped failure parks a key.
const keyAuthCooldown = 2 * time.Minute

// keyTransientCooldown is how long an outage-shaped failure parks a key: long
// enough to skip the overloaded instance, short enough that a recovered one is
// picked up by the next request.
const keyTransientCooldown = 30 * time.Second

// The rate-limit backoff ladder: the first 429 parks 2s and each consecutive
// one doubles the window, capped at 5 minutes (the reference's BACKOFF_CONFIG).
const (
	keyRateLimitBase = 2 * time.Second
	keyRateLimitMax  = 5 * time.Minute
	keyRateLimitMaxN = 15
)

// rateLimitBackoff reports the window the n-th consecutive rate limit parks a
// key for. The shift is guarded: a level past the cap, or a shift that
// overflows the duration, answers the cap rather than a negative window.
func rateLimitBackoff(n int) time.Duration {
	if n < 1 {
		n = 1
	}
	if n > keyRateLimitMaxN {
		n = keyRateLimitMaxN
	}
	window := keyRateLimitBase << uint(n-1)
	if window <= 0 || window > keyRateLimitMax {
		return keyRateLimitMax
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
// instead of after three strikes. A request-shaped failure records the reason
// but parks nothing, because the request is the cause and no key can serve it.
func (k *UpstreamKey) RecordFailure(reason string, class KeyFailureClass, now time.Time) {
	k.lastError = reason
	k.updatedAt = now
	if class == KeyFailureRequest {
		return
	}
	k.consecutiveErrors++
	window := keyTransientCooldown
	switch class {
	case KeyFailureAuth:
		window = keyAuthCooldown
	case KeyFailureRateLimit:
		window = rateLimitBackoff(k.consecutiveErrors)
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

// MarkUsed records that the router picked this key, without touching health.
func (k *UpstreamKey) MarkUsed(now time.Time) {
	used := now
	k.lastUsedAt = &used
	k.updatedAt = now
}
