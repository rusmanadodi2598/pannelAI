// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing_health.go
// @for       The key-parking windows the routing group configures and how they resolve for one provider.
// @uses      strconv, time.
// @reason    SPEC-API-001 §7.5 fixed what a park means but left its lengths as compile-time constants, so an operator facing a vendor with a harsh rate limit could not retune a window without a rebuild; the windows are routing policy and belong beside the rest of it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

import (
	"strconv"
	"time"
)

// The stored bounds of a configured parking window, in seconds. The upper bound
// on the credential window keeps a retuned park shorter than the hour ceiling the
// ladder stops at.
const (
	MinKeyParkSeconds     = 1
	MaxKeyParkAuthSeconds = 900
	MaxKeyParkRateLimit   = 300
	MaxKeyParkTransient   = 300
	// MaxKeyParkBilling holds a configured billing base to an hour, so the ladder's
	// second step is still inside two hours and it reaches its ceiling by doubling
	// rather than starting past it.
	MaxKeyParkBilling = 3600
	// MaxKeyParkQuota is the quota ladder's own ceiling: a cap configured above the
	// last rung changes nothing, so the bound is the rung rather than a number that
	// would let a write store a no-op.
	MaxKeyParkQuota = 86400
)

// KeyParkWindows is the parking budget as the routing group stores it: seconds,
// where zero means "keep the documented default" rather than "park for nothing".
type KeyParkWindows struct {
	AuthSeconds      int `json:"auth_seconds,omitempty"`
	RateLimitSeconds int `json:"rate_limit_seconds,omitempty"`
	TransientSeconds int `json:"transient_seconds,omitempty"`
	BillingSeconds   int `json:"billing_seconds,omitempty"`
	// QuotaSeconds is how long a provider's allowance actually takes to come back. One
	// field answers two questions because they are the same fact: it caps the quota
	// ladder, and it is the window an operator's disable parks for on a provider whose
	// reset is shorter than a day.
	QuotaSeconds int `json:"quota_seconds,omitempty"`
}

// IsEmpty reports whether the entry names no window at all, which is what makes a
// provider override with nothing in it dead configuration.
func (w KeyParkWindows) IsEmpty() bool {
	return w.AuthSeconds == 0 && w.RateLimitSeconds == 0 &&
		w.TransientSeconds == 0 && w.BillingSeconds == 0 && w.QuotaSeconds == 0
}

// KeyParkPatch is the parking group of a routing PATCH: one pointer per window, so
// a caller that retunes one class leaves the other two where they are.
type KeyParkPatch struct {
	AuthSeconds      *int
	RateLimitSeconds *int
	TransientSeconds *int
	BillingSeconds   *int
	QuotaSeconds     *int
}

// applyTo writes the windows the patch names, and only those.
func (p KeyParkPatch) applyTo(dst *KeyParkWindows) {
	applyInt(&dst.AuthSeconds, p.AuthSeconds)
	applyInt(&dst.RateLimitSeconds, p.RateLimitSeconds)
	applyInt(&dst.TransientSeconds, p.TransientSeconds)
	applyInt(&dst.BillingSeconds, p.BillingSeconds)
	applyInt(&dst.QuotaSeconds, p.QuotaSeconds)
}

// Merged replaces the windows this entry names with the override's own, field by
// field like RotationFor's or-chain, so a provider can retune one class and
// inherit the other two.
func (w KeyParkWindows) Merged(over KeyParkWindows) KeyParkWindows {
	if over.AuthSeconds != 0 {
		w.AuthSeconds = over.AuthSeconds
	}
	if over.RateLimitSeconds != 0 {
		w.RateLimitSeconds = over.RateLimitSeconds
	}
	if over.TransientSeconds != 0 {
		w.TransientSeconds = over.TransientSeconds
	}
	if over.BillingSeconds != 0 {
		w.BillingSeconds = over.BillingSeconds
	}
	if over.QuotaSeconds != 0 {
		w.QuotaSeconds = over.QuotaSeconds
	}
	return w
}

// Policy converts stored seconds into the durations the health model spends,
// clamping each to its bound. A value outside the range a write path no longer
// accepts is pulled to the nearest edge rather than dropped, so a document that
// predates the bounds still parks keys within a sane window.
func (w KeyParkWindows) Policy() KeyHealthPolicy {
	return KeyHealthPolicy{
		AuthWindow:      secondsWindow(w.AuthSeconds, keyAuthBase, MaxKeyParkAuthSeconds),
		RateLimitWindow: secondsWindow(w.RateLimitSeconds, keyRateLimitBase, MaxKeyParkRateLimit),
		TransientWindow: secondsWindow(w.TransientSeconds, keyTransientCooldown, MaxKeyParkTransient),
		BillingWindow:   secondsWindow(w.BillingSeconds, keyBillingBase, MaxKeyParkBilling),
		QuotaWindow:     secondsWindow(w.QuotaSeconds, keyQuotaCeiling, MaxKeyParkQuota),
	}
}

// secondsWindow answers one configured window in seconds, with the documented
// default for an unset one.
func secondsWindow(seconds int, def time.Duration, max int) time.Duration {
	if seconds < MinKeyParkSeconds {
		return def
	}
	if seconds > max {
		return time.Duration(max) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// HealthFor resolves the parking budget for one provider: the provider's override
// over the group's windows. It never fails, on RotationFor's rule that an
// unvalidated document still has to answer a usable policy.
func (s RoutingSettings) HealthFor(providerID string) KeyHealthPolicy {
	windows := s.KeyPark
	classes := s.FailureClasses
	if override, ok := s.ProviderStrategies[providerID]; ok {
		windows = windows.Merged(override.KeyPark)
		classes = classes.Merged(override.FailureClasses)
	}
	policy := windows.Policy()
	policy.FailureClasses = classes
	return policy
}

// validateKeyPark checks one entry's windows against the stored bounds. An unset
// window is legal at every level, because inheriting is the common case.
func validateKeyPark(scope string, w KeyParkWindows) error {
	fields := []struct {
		name  string
		value int
		max   int
	}{
		{"auth_seconds", w.AuthSeconds, MaxKeyParkAuthSeconds},
		{"rate_limit_seconds", w.RateLimitSeconds, MaxKeyParkRateLimit},
		{"quota_seconds", w.QuotaSeconds, MaxKeyParkQuota},
		{"transient_seconds", w.TransientSeconds, MaxKeyParkTransient},
		{"billing_seconds", w.BillingSeconds, MaxKeyParkBilling},
	}
	for _, field := range fields {
		if field.value < 0 || field.value > field.max {
			return NewValidationError(scope + ".key_park." + field.name +
				" must be unset or between 1 and " + strconv.Itoa(field.max))
		}
	}
	return nil
}
