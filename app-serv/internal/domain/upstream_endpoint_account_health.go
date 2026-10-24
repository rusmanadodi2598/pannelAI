// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_account_health.go
// @for       The account's own circuit breaker: when a failure parks the endpoint rather than a key inside it, and for how long.
// @uses      time.
// @reason    An OAuth or no-auth account is its own credential: it owns no key rows, so the key circuit had nothing to write and a dead token stayed in rotation, paying a full upstream round trip on every request that landed on it. This is the same breaker for that case only, and the boundary is deliberate: a keyed endpoint keeps parking the one credential that failed, because a vendor 429 on one key must not withhold the whole account (draft 050 T1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-14
package domain

import "time"

// ServesItsOwnCredential reports whether a failure of this endpoint is a failure of its
// credential. An account with keys is not: the key that answered is the unit spent and its
// siblings keep serving. An OAuth, service-account or no-auth row is: the token lives on
// the endpoint itself, so there is nothing smaller to park.
func (e UpstreamEndpoint) ServesItsOwnCredential() bool {
	return !e.AuthType().UsesStoredKey() && len(e.keys) == 0
}

// RecordAccountFailure parks the endpoint after a failure its own credential caused, and
// answers whether it parked. A class the credential did not cause records nothing; that
// reason still reaches the parity columns through RecordUpstreamError. The run is the
// account's own, so throttling one OAuth account escalates its ladder, not the run of
// every key beside it.
func (e *UpstreamEndpoint) RecordAccountFailure(
	class KeyFailureClass,
	policy KeyHealthPolicy,
	now time.Time,
) bool {
	if !class.ImplicatesCredential() {
		return false
	}
	e.consecutiveErrors++
	window := policy.WindowFor(class, e.consecutiveErrors)
	until := now.Add(window)
	e.rateLimitedUntil = &until
	e.updatedAt = now
	return true
}

// AccountConsecutiveErrors is the run RecordAccountFailure advances. The panel counts
// down the deadline instead: that is what decides whether the account serves.
func (e UpstreamEndpoint) AccountConsecutiveErrors() int { return e.consecutiveErrors }

// RecordAccountSuccess releases the park. It runs from RecordUpstreamSuccess, so a
// served call clears the account the same way it clears a key: recovery is proven by
// the upstream answering, not by a sweeper noticing (draft 050 §337).
func (e *UpstreamEndpoint) RecordAccountSuccess(now time.Time) {
	e.consecutiveErrors = 0
	e.rateLimitedUntil = nil
	e.updatedAt = now
}
