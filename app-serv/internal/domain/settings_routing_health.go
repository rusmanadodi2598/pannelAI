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
)

// KeyParkWindows is the parking budget as the routing group stores it: seconds,
// where zero means "keep the documented default" rather than "park for nothing".
type KeyParkWindows struct {
	AuthSeconds      int `json:"auth_seconds,omitempty"`
	RateLimitSeconds int `json:"rate_limit_seconds,omitempty"`
	TransientSeconds int `json:"transient_seconds,omitempty"`
	BillingSeconds   int `json:"billing_seconds,omitempty"`
}

// IsEmpty reports whether the entry names no window at all, which is what makes a
// provider override with nothing in it dead configuration.
func (w KeyParkWindows) IsEmpty() bool {
	return w.AuthSeconds == 0 && w.RateLimitSeconds == 0 &&
		w.TransientSeconds == 0 && w.BillingSeconds == 0
}

// KeyParkPatch is the parking group of a routing PATCH: one pointer per window, so
// a caller that retunes one class leaves the other two where they are.
type KeyParkPatch struct {
	AuthSeconds      *int
	RateLimitSeconds *int
	TransientSeconds *int
	BillingSeconds   *int
}

// applyTo writes the windows the patch names, and only those.
func (p KeyParkPatch) applyTo(dst *KeyParkWindows) {
	applyInt(&dst.AuthSeconds, p.AuthSeconds)
	applyInt(&dst.RateLimitSeconds, p.RateLimitSeconds)
	applyInt(&dst.TransientSeconds, p.TransientSeconds)
	applyInt(&dst.BillingSeconds, p.BillingSeconds)
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

// KeyHealthPolicy is the resolved parking budget for one provider. The zero value
// is usable: it answers the documented windows, which is what keeps the health
// model callable from a path that reads no settings.
type KeyHealthPolicy struct {
	AuthWindow      time.Duration
	RateLimitWindow time.Duration
	TransientWindow time.Duration
	BillingWindow   time.Duration
}

// DefaultKeyHealthPolicy answers the §7.5 windows a provider gets when nobody
// configures any.
func DefaultKeyHealthPolicy() KeyHealthPolicy {
	return KeyHealthPolicy{
		AuthWindow:      keyAuthBase,
		RateLimitWindow: keyRateLimitBase,
		TransientWindow: keyTransientCooldown,
		BillingWindow:   keyBillingBase,
	}
}

// WindowFor reports how long a failure of this class parks a key that has now
// failed it n times running. Each ladder stops at its ceiling and the
// ceilings are not configurable: a park an operator can set to never expire is
// the one-way grave draft 050 T2 had to dig an account out of.
func (p KeyHealthPolicy) WindowFor(class KeyFailureClass, n int) time.Duration {
	switch class {
	case KeyFailureAuth:
		return backoff(durationOr(p.AuthWindow, keyAuthBase), n, keyAuthMax)
	case KeyFailureRateLimit:
		return backoff(durationOr(p.RateLimitWindow, keyRateLimitBase), n, keyRateLimitMax)
	case KeyFailureBilling:
		return backoff(durationOr(p.BillingWindow, keyBillingBase), n, keyBillingMax)
	default:
		return durationOr(p.TransientWindow, keyTransientCooldown)
	}
}

// durationOr keeps the documented default for a window the policy does not name.
func durationOr(set, def time.Duration) time.Duration {
	if set <= 0 {
		return def
	}
	return set
}

// HealthFor resolves the parking budget for one provider: the provider's override
// over the group's windows. It never fails, on RotationFor's rule that an
// unvalidated document still has to answer a usable policy.
func (s RoutingSettings) HealthFor(providerID string) KeyHealthPolicy {
	windows := s.KeyPark
	if override, ok := s.ProviderStrategies[providerID]; ok {
		windows = windows.Merged(override.KeyPark)
	}
	return windows.Policy()
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
