// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing_rest.go
// @for       How one classified failure becomes a rest: the instant a key parks to, the action the router takes with it, and what a write may configure for a class.
// @uses      time, internal/domain/failure_rest.go, internal/domain/failure_class_policy.go.
// @reason    The parking ladder answers "how long has this key failed in a row", which is not the question a vendor's stated reset asks. Resolving both here, in one precedence rule, is what keeps a park and a failover from disagreeing about the same failure.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-15
package domain

import (
	"strconv"
	"time"
)

// RestUntil answers the instant a key parks to after one classified failure:
//
//	COALESCE(the vendor's stated reset, now + clamp(Retry-After, ceiling), now + the class cooldown)
//
// each held inside the class's ceiling. A reset the upstream stated wins over
// everything the gateway could compute, because a park that ends before the
// vendor's own reset is a walk leg spent to be refused again; the ceiling only
// guards against a date that cannot be real.
//
// A class that rotates or stops rather than parking answers the zero time: the
// caller parks nothing for it, and a park already on the key is left to expire
// on its own rather than being shortened here.
func (p KeyHealthPolicy) RestUntil(class KeyFailureClass, n int, now time.Time) time.Time {
	if p.ActionFor(class) != FailureActionPark {
		return time.Time{}
	}
	policy := p.PolicyFor(p.Rest.Class)
	ceiling := policy.Ceiling(restCeilingFor(class))
	if !p.Rest.ResetAt.IsZero() {
		if ceiling > 0 {
			if capped := now.Add(ceiling); p.Rest.ResetAt.After(capped) {
				return capped
			}
		}
		return p.Rest.ResetAt
	}
	// The ladder is the answer when the vendor said nothing, and it is not capped
	// a second time: every ladder already stops at its own ceiling, and a window
	// an operator configured is a decision the class ceiling must not overrule.
	wait := p.WindowFor(class, n)
	switch {
	case p.Rest.RetryAfter > 0:
		// The vendor's wait keeps the ladder's first step as its floor, as it
		// always did: a header naming one second is a queue guess, not a promise.
		if floor := p.WindowFor(class, 1); p.Rest.RetryAfter < floor {
			wait = floor
		} else {
			wait = p.Rest.RetryAfter
		}
		wait = minDuration(wait, ceiling)
	case policy.Cooldown(0) > 0:
		wait = minDuration(policy.Cooldown(0), ceiling)
	}
	return now.Add(wait)
}

// minDuration holds a wait inside a ceiling, leaving it alone when the class sets
// no ceiling at all.
func minDuration(wait, ceiling time.Duration) time.Duration {
	if ceiling > 0 && wait > ceiling {
		return ceiling
	}
	return wait
}

// ActionFor answers what the router does with one failure: the recognised class's
// policy when there is one, and the parking class's own answer otherwise. The
// walk reads it to decide whether to spend another credential, and the park reads
// the same answer to decide whether to withhold this one, so the two cannot
// disagree about a single failure.
func (p KeyHealthPolicy) ActionFor(class KeyFailureClass) FailureAction {
	return p.PolicyFor(p.Rest.Class).ActionOrDefault(defaultActionFor(class))
}

// PolicyFor answers the policy a recognised failure runs under: the provider's
// override over the documented default. An unclassified failure answers an
// empty policy, which leaves every decision to the parking class's own default.
func (p KeyHealthPolicy) PolicyFor(class FailureClass) FailureClassPolicy {
	if class == "" {
		return FailureClassPolicy{}
	}
	return DefaultFailureClassPolicy(class).Merged(p.Rest.Policy).Merged(p.FailureClasses[class])
}

// defaultActionFor is what a parking class does when no override names an
// action. Every class in the ladder parks; a target that never received the
// credential and a gate that closes on this gateway rotate, because the next
// credential may serve both; and a request the upstream refused on its own shape
// stops, because the same body is refused everywhere and walking only spends the
// budget on refusals.
func defaultActionFor(class KeyFailureClass) FailureAction {
	switch class {
	case KeyFailureTarget, KeyFailureGate:
		return FailureActionRotate
	case KeyFailureRequest:
		return FailureActionStop
	default:
		return FailureActionPark
	}
}

// restCeilingFor is the longest rest a parking class may produce on its own
// reckoning. The rate-limit and transient ceilings are the ones those ladders
// already stopped at; the quota one is a period, because a vendor's reset is a
// promise rather than a guess.
func restCeilingFor(class KeyFailureClass) time.Duration {
	switch class {
	case KeyFailureRateLimit:
		return restCeilingRateLimit
	case KeyFailureAuth:
		return restCeilingAuth
	case KeyFailureBilling:
		return restCeilingBilling
	case KeyFailureQuota:
		return restCeilingQuota
	default:
		return restCeilingTransient
	}
}

// validateFailureClasses checks one entry's per-class policies. An unknown class
// or an unknown action is refused rather than ignored, because a typo there
// would silently restore the behaviour the entry was written to change.
func validateFailureClasses(scope string, classes FailureClassPolicies) error {
	for class, policy := range classes {
		if !class.Recognised() {
			return NewValidationError(scope + ".failure_classes." + string(class) + " is not a failure class")
		}
		if policy.Action != "" && !policy.Action.Valid() {
			return NewValidationError(scope + ".failure_classes." + string(class) +
				".action must be one of park, rotate, stop")
		}
		// A class that implicates no credential parks nothing, so the router reads
		// no window for it and an entry here would be looked at by nobody. Refused
		// at the write rather than stored as configuration that looks active.
		if !class.KeyClass().ImplicatesCredential() {
			return NewValidationError(scope + ".failure_classes." + string(class) +
				" has no window to retune: it implicates no credential, so the router parks nothing for it")
		}
		if policy.CeilingSeconds < 0 || policy.CeilingSeconds > MaxFailureClassSeconds {
			return NewValidationError(scope + ".failure_classes." + string(class) +
				".ceiling_seconds must be unset or between 1 and " + strconv.Itoa(MaxFailureClassSeconds))
		}
		if policy.CooldownSeconds < 0 || policy.CooldownSeconds > MaxFailureClassSeconds {
			return NewValidationError(scope + ".failure_classes." + string(class) +
				".default_cooldown_seconds must be unset or between 1 and " + strconv.Itoa(MaxFailureClassSeconds))
		}
	}
	return nil
}
