// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing.go
// @for       The routing group's credential rotation vocabulary and the resolved policy.
// @uses      strings.
// @reason    SPEC-API-001 §7.14 fixes the group's keys and §7.5 what the policy
//
//	means; one resolution rule makes the override win over the global
//	default for the router, the read, and the write.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-24
package domain

import "strings"

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
// Both fields are optional: an absent field inherits the global default. An
// entry that sets neither is refused by validateRouting rather than stored as
// dead configuration.
type ProviderStrategy struct {
	FallbackStrategy CredentialRotation `json:"fallback_strategy,omitempty"`
	StickyLimit      *int               `json:"sticky_limit,omitempty"`
}

// RotationPolicy is the credential walk resolved for one provider: the mode the
// selector executes and the sticky limit round-robin reads.
type RotationPolicy struct {
	Strategy    CredentialRotation
	StickyLimit int
}

// UsesRotation reports whether the policy advances the shared cursor.
func (p RotationPolicy) UsesRotation() bool { return p.Strategy.UsesRotation() }

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

// RoutingSettingsPatch updates the routing group.
type RoutingSettingsPatch struct {
	ComboStrategy    *ComboStrategy
	ComboStickyLimit *int
	StickyLimit      *int
	FallbackStrategy *CredentialRotation
	// ProviderStrategies replaces the whole override map when present, so an
	// empty map is a real value (clear every override) rather than "leave
	// unchanged", which the nil pointer already means.
	ProviderStrategies *map[string]ProviderStrategy
}

// applyRouting writes the routing group from a patch.
func applyRouting(dst *RoutingSettings, p *RoutingSettingsPatch) {
	if p.ComboStrategy != nil {
		dst.ComboStrategy = *p.ComboStrategy
	}
	applyInt(&dst.ComboStickyLimit, p.ComboStickyLimit)
	applyInt(&dst.StickyLimit, p.StickyLimit)
	if p.FallbackStrategy != nil {
		dst.FallbackStrategy = *p.FallbackStrategy
	}
	if p.ProviderStrategies != nil {
		replaced := make(map[string]ProviderStrategy, len(*p.ProviderStrategies))
		for id, override := range *p.ProviderStrategies {
			replaced[id] = override
		}
		dst.ProviderStrategies = replaced
	}
}

// validateRouting checks the routing group's rules. The per-field bounds live
// in the schema tags; these are the cross-key ones and the entry rule the tags
// cannot state.
func validateRouting(s RoutingSettings) error {
	if s.ComboStickyLimit < 1 {
		return NewValidationError("routing.combo_sticky_limit must be at least 1")
	}
	if s.StickyLimit < 1 {
		return NewValidationError("routing.sticky_limit must be at least 1")
	}
	if s.StickyLimit > MaxStickyLimit {
		return NewValidationError("routing.sticky_limit must be at most 100")
	}
	if !s.ComboStrategy.IsValid() {
		return NewValidationError("routing.combo_strategy is invalid")
	}
	if s.FallbackStrategy != "" && !s.FallbackStrategy.IsValid() {
		return NewValidationError("routing.fallback_strategy must be one of fill-first, round-robin")
	}
	for id, override := range s.ProviderStrategies {
		if override.FallbackStrategy == "" && override.StickyLimit == nil {
			return NewValidationError("routing.provider_strategies[" + id +
				"] must set fallback_strategy or sticky_limit; delete the entry to inherit the default")
		}
		if override.FallbackStrategy != "" && !override.FallbackStrategy.IsValid() {
			return NewValidationError("routing.provider_strategies[" + id +
				"].fallback_strategy must be one of fill-first, round-robin")
		}
		if override.StickyLimit != nil && (*override.StickyLimit < 1 || *override.StickyLimit > MaxStickyLimit) {
			return NewValidationError("routing.provider_strategies[" + id + "].sticky_limit must be between 1 and 100")
		}
	}
	return nil
}
