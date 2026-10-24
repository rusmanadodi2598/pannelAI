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
	// keyQuotaCeiling is the last rung of the quota ladder and the window a provider
	// with no stated reset period is assumed to have: the free-tier and event cadence
	// most accounts here actually run on.
	keyQuotaCeiling = 24 * time.Hour
)

// quotaLadder is the park a spent allowance earns, walked in steps rather than
// by doubling. Doubling from fifteen minutes needs eight strikes to reach a day,
// and every strike is a walk leg the gateway already knows cannot be served;
// a vendor's period is a known length, so the park grows to it in four.
var quotaLadder = []time.Duration{
	15 * time.Minute,
	time.Hour,
	6 * time.Hour,
	keyQuotaCeiling,
}

// quotaStep answers the n-th step of that ladder, holding to the last one past
// the end rather than escalating beyond a day. A provider whose allowance comes
// back sooner caps the walk: parking a five-hour reset for a day would keep a
// usable credential out of rotation for nineteen hours it never earned.
func quotaStep(n int, ceiling time.Duration) time.Duration {
	if n < 1 {
		n = 1
	}
	if n > len(quotaLadder) {
		n = len(quotaLadder)
	}
	window := quotaLadder[n-1]
	if ceiling > 0 && ceiling < window {
		return ceiling
	}
	return window
}

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
// An operator's disable works the same way and for the same reason, with one
// difference: a park with no deadline is permanent, because a switched-off key
// has no vendor reset for a window to point at.
func (k UpstreamKey) Available(now time.Time) bool {
	if k.status == UpstreamKeyDisabled {
		return k.disabledUntil != nil && !now.Before(*k.disabledUntil)
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
	k.lastErrorClass = policy.Rest.Label(class)
	k.resetSource = policy.Rest.SourceLabel()
	k.signatureID = policy.Rest.SignatureID
	k.updatedAt = now
	if !class.ImplicatesCredential() {
		return
	}
	k.consecutiveErrors++
	k.bumpRun(class)
	// The rest is absolute where the vendor stated one, so a spent allowance
	// stays out of rotation until its own reset instead of re-entering it every
	// cycle to be refused again. A class an operator retuned to rotate rather
	// than park answers the zero time and parks nothing at all.
	until := policy.RestUntil(class, k.runFor(class), now)
	if until.IsZero() {
		return
	}
	k.status = UpstreamKeyError
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
	case KeyFailureBilling, KeyFailureQuota:
		// A spent allowance advances the account's own run: it is the same
		// account-level condition a 402 names, measured on the same column, and
		// giving it a counter of its own would need a column this table does not
		// have for a condition that is no different to act on.
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
	case KeyFailureBilling, KeyFailureQuota:
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
	k.resetSource = ""
	k.signatureID = ""
	k.rateLimitedUntil = nil
	if k.status == UpstreamKeyError {
		k.status = UpstreamKeyActive
	}
	used := now
	k.lastUsedAt = &used
	k.updatedAt = now
}
