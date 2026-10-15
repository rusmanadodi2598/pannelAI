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
	// FailureClasses retunes what a recognised failure class does. It merges
	// over what is stored, so a PATCH that names one class leaves the others
	// where an earlier write put them.
	FailureClasses *FailureClassPolicies
	// Failover retunes the walk budget, each bound on its own for the same reason.
	Failover *FailoverBudgetPatch
}

// FailoverBudgetPatch is the walk-budget group of a routing PATCH: one pointer per
// bound, so an operator widening the attempt count leaves the clock alone.
type FailoverBudgetPatch struct {
	TotalMS     *int
	MaxAttempts *int
}

// applyTo writes the bounds the patch names, and only those.
func (p FailoverBudgetPatch) applyTo(dst *FailoverBudget) {
	applyInt(&dst.TotalMS, p.TotalMS)
	applyInt(&dst.MaxAttempts, p.MaxAttempts)
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
	if p.FailureClasses != nil {
		dst.FailureClasses = dst.FailureClasses.Merged(*p.FailureClasses)
	}
	if p.Failover != nil {
		p.Failover.applyTo(&dst.Failover)
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
	// An unset failover block is legal and normalizes to the documented budget; a
	// stored one outside its bounds is refused rather than quietly re-read as the
	// default, because a walk whose operator typed 5000 and got 8 has been told a
	// different story than the one it wrote.
	if s.Failover.TotalMS != 0 &&
		(s.Failover.TotalMS < MinFailoverTotalMS || s.Failover.TotalMS > MaxFailoverTotalMS) {
		return NewValidationError("routing.failover.total_ms must be between 1000 and 120000, or unset")
	}
	if s.Failover.MaxAttempts != 0 &&
		(s.Failover.MaxAttempts < MinFailoverAttempts || s.Failover.MaxAttempts > MaxFailoverAttempts) {
		return NewValidationError("routing.failover.max_attempts must be between 1 and 200, or unset")
	}
	if !s.ComboStrategy.IsValid() {
		return NewValidationError("routing.combo_strategy is invalid")
	}
	if s.FallbackStrategy != "" && !s.FallbackStrategy.IsValid() {
		return NewValidationError("routing.fallback_strategy must be one of fill-first, round-robin")
	}
	for id, override := range s.ProviderStrategies {
		scope := "routing.provider_strategies[" + id + "]"
		if override.FallbackStrategy == "" && override.StickyLimit == nil &&
			override.KeyPark.IsEmpty() && override.FailureClasses.IsEmpty() {
			return NewValidationError(scope +
				" must set fallback_strategy, sticky_limit, key_park, or failure_classes; delete the entry to inherit the default")
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
		if err := validateFailureClasses(scope, override.FailureClasses); err != nil {
			return err
		}
	}
	if err := validateKeyPark("routing", s.KeyPark); err != nil {
		return err
	}
	return validateFailureClasses("routing", s.FailureClasses)
}
