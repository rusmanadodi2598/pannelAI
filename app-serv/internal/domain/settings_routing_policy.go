// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing_policy.go
// @for       The resolved parking budget one provider gets, the ladders it walks, and the operator window it sizes.
// @uses      time.
// @reason    Stored seconds become durations exactly once, at the point a failure is about to be parked, and the same resolved value answers two questions: how long the quota ladder may grow, and how long an operator's park holds. Keeping both readings beside the ladder they feed is what stops the two from drifting into separate numbers for one vendor's reset. Split from settings_routing_health.go so each stays inside the AGENTS.md §1.1 line budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-16
package domain

import "time"

// KeyHealthPolicy is the resolved parking budget for one provider. The zero value
// is usable: it answers the documented windows, which is what keeps the health
// model callable from a path that reads no settings.
type KeyHealthPolicy struct {
	AuthWindow      time.Duration
	RateLimitWindow time.Duration
	TransientWindow time.Duration
	BillingWindow   time.Duration
	// QuotaWindow is how long this provider's allowance takes to come back: the ceiling the
	// quota ladder walks up to, and the window an operator's disable parks for. One value,
	// because a vendor that resets every five hours is the same fact asked two ways.
	QuotaWindow time.Duration
	// RetryAfterHint is the wait the vendor itself named for this failure, or zero. It rides
	// the budget because it answers the budget's own question: how long to stay away.
	RetryAfterHint time.Duration
	// Rest is what the classifier recognised about this one failure: the class, the
	// vendor's own reset and wait, and where each came from. It rides the budget for
	// the same reason the hint does, and the zero value is the pre-classification
	// behaviour every caller that has nothing to report still gets.
	Rest RestDirective
	// FailureClasses is the per-class action and windows resolved for this provider:
	// the provider's override over the group's over the documented defaults. A nil
	// map answers the documented default for every class.
	FailureClasses FailureClassPolicies
}

// WithRetryAfter returns the same budget with the vendor's own wait applied.
func (p KeyHealthPolicy) WithRetryAfter(hint time.Duration) KeyHealthPolicy {
	p.RetryAfterHint = hint
	return p
}

// WithRest returns the same budget with one classified failure applied: the
// vendor's wait, so the ladder still reads what it always read, and the rest
// the classification earned, which outranks it.
func (p KeyHealthPolicy) WithRest(rest RestDirective) KeyHealthPolicy {
	p.Rest = rest
	p.RetryAfterHint = rest.RetryAfter
	return p
}

// DisableWindow answers how long an operator's park on this provider's account lasts
// before the row releases itself. It is the provider's own reset cadence rather than a
// number chosen beside it: a lane that says its allowance returns in five hours says the
// same thing about a manual park.
func (p KeyHealthPolicy) DisableWindow() time.Duration {
	if p.QuotaWindow <= 0 {
		return DisabledWindowDefault
	}
	return p.QuotaWindow
}

// DefaultKeyHealthPolicy answers the §7.5 windows a provider gets when nobody
// configures any.
func DefaultKeyHealthPolicy() KeyHealthPolicy {
	return KeyHealthPolicy{
		AuthWindow:      keyAuthBase,
		RateLimitWindow: keyRateLimitBase,
		TransientWindow: keyTransientCooldown,
		BillingWindow:   keyBillingBase,
		QuotaWindow:     keyQuotaCeiling,
	}
}

// WindowFor reports how long a failure of this class parks a key that has now failed it n
// times running. Each ladder stops at a ceiling that is not configurable: a park an operator
// can set never to expire is the one-way grave draft 050 T2 had to dig an account out of.
//
// A rate limit honors the vendor's own wait where it named one: a 429 saying "come back in
// two minutes" answered with the ladder's two-second first step is a key put back into
// rotation to be throttled again. The ladder's ceiling bounds the hint too, because the wait
// a vendor names for its own queue is a guess, not a promise.
func (p KeyHealthPolicy) WindowFor(class KeyFailureClass, n int) time.Duration {
	switch class {
	case KeyFailureAuth:
		return backoff(durationOr(p.AuthWindow, keyAuthBase), n, keyAuthMax)
	case KeyFailureRateLimit:
		base := durationOr(p.RateLimitWindow, keyRateLimitBase)
		if p.RetryAfterHint > 0 {
			return clampDuration(p.RetryAfterHint, base, keyRateLimitMax)
		}
		return backoff(base, n, keyRateLimitMax)
	case KeyFailureBilling:
		return backoff(durationOr(p.BillingWindow, keyBillingBase), n, keyBillingMax)
	case KeyFailureQuota:
		return quotaStep(n, p.QuotaWindow)
	default:
		return durationOr(p.TransientWindow, keyTransientCooldown)
	}
}

// clampDuration pulls a vendor's named wait inside the ladder it belongs to: never
// shorter than that ladder's first step, never longer than its ceiling.
func clampDuration(wait, floor, ceiling time.Duration) time.Duration {
	if wait < floor {
		return floor
	}
	if wait > ceiling {
		return ceiling
	}
	return wait
}

// durationOr keeps the documented default for a window the policy does not name.
func durationOr(set, def time.Duration) time.Duration {
	if set <= 0 {
		return def
	}
	return set
}
