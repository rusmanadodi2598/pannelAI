// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing_patch.go
// @for       The routing group's PATCH mutation: what a patch writes and the rules a stored group must satisfy.
// @uses      Nothing outside the package: the group's own types, applyInt, and NewValidationError.
// @reason    The routing group is read by the data plane on every selection and written by one settings PATCH. Keeping the write rules apart from the policy types is what holds AGENTS.md §1.1's line budget while both stay one reviewable concern, and it mirrors the schema layer's own settings_routing_patch.go.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

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
	// KeyPark retunes the group's parking windows. Each window is optional on
	// its own, because a PATCH that names one must not reset the other two.
	KeyPark *KeyParkPatch
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
	if p.KeyPark != nil {
		p.KeyPark.applyTo(&dst.KeyPark)
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
		scope := "routing.provider_strategies[" + id + "]"
		if override.FallbackStrategy == "" && override.StickyLimit == nil && override.KeyPark.IsEmpty() {
			return NewValidationError(scope +
				" must set fallback_strategy, sticky_limit, or key_park; delete the entry to inherit the default")
		}
		if override.FallbackStrategy != "" && !override.FallbackStrategy.IsValid() {
			return NewValidationError(scope +
				"].fallback_strategy must be one of fill-first, round-robin")
		}
		if override.StickyLimit != nil && (*override.StickyLimit < 1 || *override.StickyLimit > MaxStickyLimit) {
			return NewValidationError(scope + "].sticky_limit must be between 1 and 100")
		}
		if err := validateKeyPark(scope, override.KeyPark); err != nil {
			return err
		}
	}
	return validateKeyPark("routing", s.KeyPark)
}
