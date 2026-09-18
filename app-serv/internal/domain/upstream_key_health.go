// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_health.go
// @for       The circuit-breaker state of an upstream key: whether routing may
//
//	spend it, and what a failure or a success does to that answer.
//
// @uses      time.
// @reason    SPEC-API-001 §7.5 defines the health model and the router's
//
//	correctness depends on it: skipping a failing key is what makes
//	fallback work, so the threshold, the window, and the reset live
//	together here rather than at each call site.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import "time"

// Available reports whether the router may pick this key at the given instant.
//
// A key the circuit breaker tripped becomes usable again once its backoff
// expires, without an explicit reset: the window *is* the state, so nothing has
// to sweep expired keys, and a key that recovers on its own is picked up by the
// next request rather than sitting idle until a worker notices.
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

// RecordFailure counts a failed attempt and trips the circuit once the
// threshold is reached, setting the backoff window from the given instant.
func (k *UpstreamKey) RecordFailure(reason string, now time.Time) {
	k.consecutiveErrors++
	k.lastError = reason
	k.updatedAt = now
	if k.consecutiveErrors < keyCircuitThreshold {
		return
	}
	k.status = UpstreamKeyError
	until := now.Add(keyCircuitBackoff)
	k.rateLimitedUntil = &until
}

// RecordSuccess clears the health state after a request the upstream accepted:
// the counter, the recorded error, and the backoff all reset, and a key that
// was tripped returns to active.
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

// CircuitThreshold exposes the failure count that trips a key, so the panel can
// explain the rule without duplicating the constant.
func CircuitThreshold() int { return keyCircuitThreshold }

// CircuitBackoff exposes the window a tripped key is skipped for.
func CircuitBackoff() time.Duration { return keyCircuitBackoff }
