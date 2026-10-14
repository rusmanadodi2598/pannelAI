// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing.go
// @for       The routing group's credential rotation vocabulary and the resolved policy.
// @uses      strings.
// @reason    SPEC-API-001 §7.14 fixes the group's keys and §7.5 what the policy means; one resolution rule makes the override win over the global default for the router, the read, and the write.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-24
package domain

import (
	"strings"
	"time"
)

// CredentialRotation is how a provider's credential walk orders its endpoints
// and the keys inside them. The set is closed and mirrors the reference's
// fallbackStrategy values.
type CredentialRotation string

const (
	// RotationFillFirst serves priority order and starts every request from the
	// first usable endpoint, picking the first healthy key by priority. It is
	// the documented default, as in the reference.
	RotationFillFirst CredentialRotation = "fill-first"
	// RotationRoundRobin advances the shared cursor, keeps one endpoint for
	// sticky_limit consecutive requests, and rotates keys least-recently-used
	// first.
	RotationRoundRobin CredentialRotation = "round-robin"
)

// DefaultStickyLimit is the documented §7.14 sticky default, the value the
// reference uses when neither the provider override nor the global setting
// carries one.
const DefaultStickyLimit = 3

// MaxStickyLimit bounds a stored sticky limit, matching the §7.14 PATCH rule.
const MaxStickyLimit = 100

// ParseCredentialRotation validates a wire value before it reaches the
// aggregate.
func ParseCredentialRotation(value string) (CredentialRotation, error) {
	mode := CredentialRotation(strings.TrimSpace(value))
	if !mode.IsValid() {
		return "", NewValidationError("invalid credential rotation: " + value)
	}
	return mode, nil
}

// IsValid reports whether the mode is one the selector can execute. The zero
// value is invalid, which is what lets a read path tell "unset" from a choice.
func (m CredentialRotation) IsValid() bool {
	switch m {
	case RotationFillFirst, RotationRoundRobin:
		return true
	default:
		return false
	}
}

// UsesRotation reports whether the mode reads the cursor and the sticky limit:
// fill-first reads neither, so a rotation write for it is state nobody reads.
func (m CredentialRotation) UsesRotation() bool { return m == RotationRoundRobin }

// ProviderStrategy is one provider's override of the global rotation policy.
// Every field is optional: an absent one inherits the global default. An entry
// that sets nothing is refused by validateRouting rather than stored as dead
// configuration.
type ProviderStrategy struct {
	FallbackStrategy CredentialRotation `json:"fallback_strategy,omitempty"`
	StickyLimit      *int               `json:"sticky_limit,omitempty"`
	// KeyPark retunes the parking windows for this provider alone. An empty entry
	// inherits every window from the group.
	KeyPark KeyParkWindows `json:"key_park,omitempty"`
}

// RotationPolicy is the credential walk resolved for one provider: the mode the
// selector executes and the sticky limit round-robin reads.
type RotationPolicy struct {
	Strategy    CredentialRotation
	StickyLimit int
}

// UsesRotation reports whether the policy advances the shared cursor.
func (p RotationPolicy) UsesRotation() bool { return p.Strategy.UsesRotation() }

// FailoverBudget bounds one client request's whole walk over the credentials it may try.
// The chain is combinatorial: a combo of 64 members, each with up to 100 accounts, each
// holding up to 100 keys. Each attempt carried its own 120-second budget with no total
// above it, so a request could outlive its client by hours.
//
// Both bounds stop the walk and report what it already learned; neither turns an answer
// the client received into a failure, because a stream that has spoken is committed.
type FailoverBudget struct {
	// TotalMS is the wall clock the PRE-COMMIT walk may spend.
	TotalMS int `json:"total_ms"`
	// MaxAttempts is how many credentials one request may spend, the bound a deadline
	// cannot give: a provider that answers instantly with an error on every one of its keys.
	MaxAttempts int `json:"max_attempts"`
}

// The §7.14 failover defaults and their stored bounds. The total ceiling is the transport's
// own per-credential budget, so one request never gets more time than a single call may take;
// the attempt ceiling walks a large provider without ever permitting the full chain.
const (
	DefaultFailoverTotalMS  = 30_000
	MinFailoverTotalMS      = 1_000
	MaxFailoverTotalMS      = 120_000
	DefaultFailoverAttempts = 8
	MinFailoverAttempts     = 1
	MaxFailoverAttempts     = 200
)

// Normalized fills the two keys a stored row written before they existed cannot carry. A
// group decode replaces the routing document wholesale, so an absent key arrives as a zero,
// and a walk handed zero attempts answers NO_PROVIDER_AVAILABLE for a provider that has keys.
func (b FailoverBudget) Normalized() FailoverBudget {
	if b.TotalMS < MinFailoverTotalMS || b.TotalMS > MaxFailoverTotalMS {
		b.TotalMS = DefaultFailoverTotalMS
	}
	if b.MaxAttempts < MinFailoverAttempts || b.MaxAttempts > MaxFailoverAttempts {
		b.MaxAttempts = DefaultFailoverAttempts
	}
	return b
}

// Total answers the budget as the duration a walk measures against.
func (b FailoverBudget) Total() time.Duration {
	return time.Duration(b.Normalized().TotalMS) * time.Millisecond
}

// Attempts answers the budget as a count.
func (b FailoverBudget) Attempts() int { return b.Normalized().MaxAttempts }

// RoutingSettings is the §7.14 routing group. The combo keys are the defaults a
// new combo starts from; the credential keys are the policy the data plane's
// selector resolves per provider.
type RoutingSettings struct {
	ComboStrategy    ComboStrategy `json:"combo_strategy"`
	ComboStickyLimit int           `json:"combo_sticky_limit"`
	StickyLimit      int           `json:"sticky_limit"`
	// FallbackStrategy is the global credential rotation default. An empty
	// stored value means "unset" and resolves to fill-first.
	FallbackStrategy CredentialRotation `json:"fallback_strategy,omitempty"`
	// ProviderStrategies holds the per-provider overrides, keyed by provider
	// id. It is written whole, like the reference's providerStrategies map.
	ProviderStrategies map[string]ProviderStrategy `json:"provider_strategies,omitempty"`
	// KeyPark holds the global key-parking windows. It is the §7.5 budget the
	// ladder starts from before any provider override.
	KeyPark KeyParkWindows `json:"key_park,omitempty"`
	// Failover bounds one request's whole credential walk: global rather than per-provider,
	// because the chain it cuts belongs to the request (a combo, its accounts, their keys).
	Failover FailoverBudget `json:"failover,omitempty"`
}

// defaultRoutingSettings is the §7.14 routing default: combo fallback with a
// sticky of 1, credential fill-first with a sticky of 3.
func defaultRoutingSettings() RoutingSettings {
	return RoutingSettings{
		ComboStrategy:      ComboFallback,
		ComboStickyLimit:   1,
		StickyLimit:        DefaultStickyLimit,
		FallbackStrategy:   RotationFillFirst,
		ProviderStrategies: map[string]ProviderStrategy{},
		Failover: FailoverBudget{
			TotalMS:     DefaultFailoverTotalMS,
			MaxAttempts: DefaultFailoverAttempts,
		},
	}
}

// Normalized fills the keys a stored row written before this change cannot
// carry. The group decode replaces the group wholesale, so an absent key
// arrives as its zero value rather than as the documented default, and a read
// that answered "unset" would fail the panel's own form validation.
func (s RoutingSettings) Normalized() RoutingSettings {
	if !s.FallbackStrategy.IsValid() {
		s.FallbackStrategy = RotationFillFirst
	}
	if s.ProviderStrategies == nil {
		s.ProviderStrategies = map[string]ProviderStrategy{}
	}
	if s.StickyLimit < 1 {
		s.StickyLimit = DefaultStickyLimit
	}
	s.Failover = s.Failover.Normalized()
	return s
}

// RotationFor resolves the credential walk for one provider: the per-provider
// override over the global default, with the documented fallbacks. It never
// fails: an unvalidated document still answers a usable policy.
func (s RoutingSettings) RotationFor(providerID string) RotationPolicy {
	policy := RotationPolicy{Strategy: s.FallbackStrategy, StickyLimit: s.StickyLimit}
	if override, ok := s.ProviderStrategies[providerID]; ok {
		if override.FallbackStrategy != "" {
			policy.Strategy = override.FallbackStrategy
		}
		// A stored limit below 1 inherits the global one, mirroring the
		// reference's falsy-or chain; the write path refuses to store one.
		if override.StickyLimit != nil && *override.StickyLimit >= 1 {
			policy.StickyLimit = *override.StickyLimit
		}
	}
	if !policy.Strategy.IsValid() {
		policy.Strategy = RotationFillFirst
	}
	if policy.StickyLimit < 1 {
		policy.StickyLimit = DefaultStickyLimit
	}
	return policy
}
