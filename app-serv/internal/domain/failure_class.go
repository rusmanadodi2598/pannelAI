// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/failure_class.go
// @for       The failure taxonomy an upstream refusal is classified into, and the per-class ceilings and defaults that vocabulary implies.
// @uses      time.
// @reason    A status alone cannot say what failed: one vendor's 429 is a short throttle and another's is a spent weekly allowance whose own body names the reset. The class is what decides the park, so the vocabulary lives in the domain, while the windows it implies and the body patterns that recognise it live beside it and with the connectors that know the vendor.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-15
package domain

import "time"

// FailureClass is what an upstream failure is, finer than the status that
// carried it. The class decides the park: a short throttle returns a key to
// rotation in minutes, a spent allowance stays parked until the vendor's own
// reset, and a gate that refuses this gateway altogether parks nothing.
//
// The values are stored verbatim in upstream_keys.last_error_class, so a rename
// here is a migration, not a cosmetic edit.
type FailureClass string

const (
	// FailureRateLimitTemporary is a throttle the vendor expects to lift on its
	// own: "under heavy load", a per-minute cap. A Retry-After header is honoured
	// and the park keeps the 5-minute ceiling it had.
	FailureRateLimitTemporary FailureClass = "RATE_LIMIT_TEMPORARY"
	// FailureQuotaExhausted is an allowance that is spent for the period: the
	// key cannot serve again until the vendor's own reset, which its body
	// usually names. It rotates immediately and parks to that reset.
	FailureQuotaExhausted FailureClass = "QUOTA_EXHAUSTED"
	// FailureAuthInvalid is a credential the upstream refused: 401/403 with no
	// quota or billing signal in the body.
	FailureAuthInvalid FailureClass = "AUTH_INVALID"
	// FailureBillingExhausted is an account with no credit left.
	FailureBillingExhausted FailureClass = "BILLING_EXHAUSTED"
	// FailureServerTransient is an upstream outage: a 5xx that says nothing
	// about the credential.
	FailureServerTransient FailureClass = "SERVER_TRANSIENT"
	// FailureTargetUnreachable covers a call that never reached the upstream and
	// a 404: the target caused it, nothing was sent with this credential, so the
	// chain rotates and the key is left alone.
	FailureTargetUnreachable FailureClass = "TARGET_UNREACHABLE"
	// FailureGateBlocked is an upstream that refuses this gateway rather than
	// this credential: a free tier reachable only from the vendor's own client.
	// It parks nothing and retries nothing, because no key on the account helps.
	FailureGateBlocked FailureClass = "GATE_BLOCKED"
	// FailureRequestInvalid is a request the upstream refused on its own shape,
	// which fails identically on every credential: the chain stops.
	FailureRequestInvalid FailureClass = "REQUEST_INVALID"
	// FailureUnknown is a failure no signature and no status rule recognised.
	FailureUnknown FailureClass = "UNKNOWN"
)

// FailureAction is what the router does with a failure of one class.
type FailureAction string

const (
	// FailureActionPark parks the credential until its rest and rotates.
	FailureActionPark FailureAction = "park"
	// FailureActionRotate moves to the next credential and parks nothing.
	FailureActionRotate FailureAction = "rotate"
	// FailureActionStop ends the chain: the same body fails everywhere.
	FailureActionStop FailureAction = "stop"
)

// The per-class ceilings and cooldown defaults. A ceiling bounds a wait the
// gateway computed itself; it does not shorten a reset the upstream stated,
// which is why the quota ceiling is a period rather than a few minutes.
const (
	restCeilingRateLimit = 5 * time.Minute
	restCeilingTransient = 30 * time.Second
	restCeilingAuth      = time.Hour
	restCeilingBilling   = 6 * time.Hour
	// restCeilingQuota bounds a parsed reset too, so a nonsense date (a year, a
	// zero clock) cannot bury a key. A vendor's free allowance really is a
	// period long, so the cap is a month rather than an hour.
	restCeilingQuota = 30 * 24 * time.Hour
)

// The stored bounds of one configured class policy, in seconds.
const (
	MinFailureClassSeconds = 1
	MaxFailureClassSeconds = int(restCeilingQuota / time.Second)
)

// FailureClassForStatus is the conservative mapping used when no signature
// matched: it answers what the status alone can carry, which is today's
// behaviour. A 404 is a target failure rather than a malformed request because
// another credential on the same body may serve the model, and the chain has
// always kept walking for it.
func FailureClassForStatus(status int) FailureClass {
	switch {
	case status == 429:
		return FailureRateLimitTemporary
	case status == 401 || status == 403:
		return FailureAuthInvalid
	case status == 402:
		return FailureBillingExhausted
	case status >= 500:
		return FailureServerTransient
	case status == 404:
		return FailureTargetUnreachable
	case status >= 400:
		return FailureRequestInvalid
	case status == 0:
		// No status at all: the upstream never answered, which is the target's
		// own failure rather than anything the credential did.
		return FailureTargetUnreachable
	default:
		return FailureUnknown
	}
}

// KeyClass maps a failure onto the parking class that owns its window. Two
// failure classes may share one ladder: a spent allowance and a spent account
// are both the account's own run, and an unrecognised failure is treated as the
// transient outage it most resembles.
func (c FailureClass) KeyClass() KeyFailureClass {
	switch c {
	case FailureRateLimitTemporary:
		return KeyFailureRateLimit
	case FailureQuotaExhausted:
		return KeyFailureQuota
	case FailureAuthInvalid:
		return KeyFailureAuth
	case FailureBillingExhausted:
		return KeyFailureBilling
	case FailureTargetUnreachable:
		return KeyFailureTarget
	case FailureGateBlocked:
		return KeyFailureGate
	case FailureRequestInvalid:
		return KeyFailureRequest
	default:
		return KeyFailureTransient
	}
}

// Valid reports whether the action is one the router can execute.
func (a FailureAction) Valid() bool {
	switch a {
	case FailureActionPark, FailureActionRotate, FailureActionStop:
		return true
	default:
		return false
	}
}

// Recognised reports whether the class is one the gateway can produce, which is
// what lets a settings write refuse a typo instead of storing an entry no
// failure will ever match.
func (c FailureClass) Recognised() bool {
	switch c {
	case FailureRateLimitTemporary, FailureQuotaExhausted, FailureAuthInvalid,
		FailureBillingExhausted, FailureServerTransient, FailureTargetUnreachable,
		FailureGateBlocked, FailureRequestInvalid, FailureUnknown:
		return true
	default:
		return false
	}
}

// Retryable reports whether the same key may be asked again inside one call.
// Only a throttle and an outage can plausibly answer differently a moment
// later; a spent account, a refused credential, and a closed gate cannot.
func (c FailureClass) Retryable() bool {
	return c == FailureRateLimitTemporary || c == FailureServerTransient
}

// Label is the token stored in last_error_class. An unclassified failure stores
// UNKNOWN rather than an empty string, so the panel can tell "classified as
// nothing" from "never classified".
func (c FailureClass) Label() string {
	if c == "" {
		return string(FailureUnknown)
	}
	return string(c)
}
