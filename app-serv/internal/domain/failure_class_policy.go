// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/failure_class_policy.go
// @for       The per-class action and windows an operator may retune, the table the routing group stores, and how one entry layers over the documented default.
// @uses      time, internal/domain/failure_class.go for the classes it is written against.
// @reason    The class says what a failure was; this file is what the gateway decided to do about it. Keeping the two apart means a vendor's stated ceiling and an operator's override are merged in one place, and a provider that names a single class inherits every other one.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-15
package domain

import "time"

// FailureClassPolicy is one class's action and windows as an operator may
// retune them. A zero field inherits the documented default, which is what
// keeps a provider override that names only the action legal.
type FailureClassPolicy struct {
	Action FailureAction `json:"action,omitempty"`
	// CeilingSeconds is the longest rest a computed wait may produce, from the
	// failure onwards. Zero inherits the class's default ceiling.
	CeilingSeconds int `json:"ceiling_seconds,omitempty"`
	// CooldownSeconds is the park used when neither a reset nor a Retry-After
	// was found. Zero keeps the class's ladder, which is how a rate limit still
	// escalates.
	CooldownSeconds int `json:"default_cooldown_seconds,omitempty"`
}

// IsEmpty reports whether the entry names nothing, which is what makes a
// provider override with no field in it dead configuration.
func (p FailureClassPolicy) IsEmpty() bool {
	return p.Action == "" && p.CeilingSeconds == 0 && p.CooldownSeconds == 0
}

// Merged overlays the fields the override names onto this entry, field by field
// like KeyParkWindows.Merged.
func (p FailureClassPolicy) Merged(over FailureClassPolicy) FailureClassPolicy {
	if over.Action != "" {
		p.Action = over.Action
	}
	if over.CeilingSeconds != 0 {
		p.CeilingSeconds = over.CeilingSeconds
	}
	if over.CooldownSeconds != 0 {
		p.CooldownSeconds = over.CooldownSeconds
	}
	return p
}

// ActionOrDefault answers the action this class takes, or the default when the
// entry names none.
func (p FailureClassPolicy) ActionOrDefault(def FailureAction) FailureAction {
	if p.Action == "" {
		return def
	}
	return p.Action
}

// Ceiling answers this entry's ceiling, or the default one. It clamps a stored
// value that predates the bounds the way the parking windows do.
func (p FailureClassPolicy) Ceiling(def time.Duration) time.Duration {
	return classWindowSeconds(p.CeilingSeconds, def)
}

// Cooldown answers this entry's flat cooldown, or zero for "keep the ladder".
func (p FailureClassPolicy) Cooldown(def time.Duration) time.Duration {
	if p.CooldownSeconds <= 0 {
		return def
	}
	return classWindowSeconds(p.CooldownSeconds, def)
}

// classWindowSeconds reads one configured window in seconds, holding it inside
// the stored bounds.
func classWindowSeconds(seconds int, def time.Duration) time.Duration {
	if seconds < MinFailureClassSeconds {
		return def
	}
	if seconds > MaxFailureClassSeconds {
		return time.Duration(MaxFailureClassSeconds) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// DefaultFailureClassPolicy is the documented answer for one class. The
// ceilings are what a vendor's own Retry-After is held to; the cooldowns are
// zero wherever a ladder already escalates, so a rate limit keeps backing off
// instead of parking flat.
func DefaultFailureClassPolicy(class FailureClass) FailureClassPolicy {
	switch class {
	case FailureRateLimitTemporary:
		return FailureClassPolicy{Action: FailureActionPark, CeilingSeconds: int(restCeilingRateLimit / time.Second)}
	case FailureQuotaExhausted:
		return FailureClassPolicy{Action: FailureActionPark, CeilingSeconds: int(restCeilingQuota / time.Second)}
	case FailureAuthInvalid:
		return FailureClassPolicy{Action: FailureActionPark, CeilingSeconds: int(restCeilingAuth / time.Second)}
	case FailureBillingExhausted:
		return FailureClassPolicy{Action: FailureActionPark, CeilingSeconds: int(restCeilingBilling / time.Second)}
	case FailureServerTransient, FailureUnknown:
		return FailureClassPolicy{Action: FailureActionPark,
			CeilingSeconds:  int(restCeilingTransient / time.Second),
			CooldownSeconds: int(restCeilingTransient / time.Second)}
	case FailureTargetUnreachable:
		return FailureClassPolicy{Action: FailureActionRotate}
	default:
		return FailureClassPolicy{Action: FailureActionStop}
	}
}

// FailureClassPolicies is the per-class override table as the routing group
// stores it, keyed by class name. An absent class inherits its default.
type FailureClassPolicies map[FailureClass]FailureClassPolicy

// IsEmpty reports whether the table names no class at all.
func (m FailureClassPolicies) IsEmpty() bool { return len(m) == 0 }

// Merged overlays another table's entries onto this one, so a provider can
// retune one class and inherit the rest.
func (m FailureClassPolicies) Merged(over FailureClassPolicies) FailureClassPolicies {
	if len(over) == 0 {
		return m
	}
	merged := make(FailureClassPolicies, len(m)+len(over))
	for class, policy := range m {
		merged[class] = policy
	}
	for class, policy := range over {
		merged[class] = merged[class].Merged(policy)
	}
	return merged
}

// PolicyFor answers the policy one class runs under: this table's entry over
// the documented default.
func (m FailureClassPolicies) PolicyFor(class FailureClass) FailureClassPolicy {
	return DefaultFailureClassPolicy(class).Merged(m[class])
}
