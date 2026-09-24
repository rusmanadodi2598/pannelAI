// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_routing_patch.go
// @for       The routing group's PATCH DTO, its lowering into the domain
//
//	mutation, and the per-entry rules of the provider override map.
//
// @uses      internal/domain.
// @reason    SPEC-API-001 §7.14 validates a PATCH per key. The provider
//
//	override map is the one value whose entries carry their own rules,
//	so keeping its DTO and its checks together is what stops the
//	settings patch file from growing past the line budget while the
//	group stays one reviewable unit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-24
package schema

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"

// RoutingSettingsPatch updates the routing group. A non-positive sticky limit
// is rejected rather than clamped: a limit of zero would rotate on every
// request, which is not what a caller sending 0 can have meant. The provider
// override map is a whole replacement, so an empty object clears every
// override and an absent key inherits the global default.
type RoutingSettingsPatch struct {
	ComboStrategy      *string                           `json:"combo_strategy,omitempty" validate:"omitempty,oneof=fallback round_robin fusion"`
	ComboStickyLimit   *int                              `json:"combo_sticky_limit,omitempty" validate:"omitempty,min=1,max=100"`
	StickyLimit        *int                              `json:"sticky_limit,omitempty" validate:"omitempty,min=1,max=100"`
	FallbackStrategy   *string                           `json:"fallback_strategy,omitempty" validate:"omitempty,oneof=fill-first round-robin"`
	ProviderStrategies *map[string]ProviderStrategyPatch `json:"provider_strategies,omitempty" validate:"omitempty,dive"`
}

// ProviderStrategyPatch is one provider's override as the wire carries it.
type ProviderStrategyPatch struct {
	FallbackStrategy *string `json:"fallback_strategy,omitempty" validate:"omitempty,oneof=fill-first round-robin"`
	StickyLimit      *int    `json:"sticky_limit,omitempty" validate:"omitempty,min=1,max=100"`
}

// validateProviderStrategies applies the entry rules the struct tags cannot
// express: every entry must set at least one field, because an empty entry is
// configuration nobody reads. The reference's panel deletes the entry instead
// of writing an empty one, which is the same rule expressed in its own way.
func validateProviderStrategies(in *RoutingSettingsPatch) error {
	if in == nil || in.ProviderStrategies == nil {
		return nil
	}
	for id, override := range *in.ProviderStrategies {
		if override.FallbackStrategy == nil && override.StickyLimit == nil {
			return domain.NewValidationError("provider_strategies[" + id +
				"] must set fallback_strategy or sticky_limit; delete the entry to inherit the default")
		}
	}
	return nil
}

// routingPtr lowers the routing group into the domain mutation object. The DTO
// layer has already rejected an out-of-set strategy or an out-of-range limit,
// so an unparseable value cannot reach here.
func routingPtr(in *RoutingSettingsPatch) *domain.RoutingSettingsPatch {
	if in == nil {
		return nil
	}
	patch := &domain.RoutingSettingsPatch{
		ComboStickyLimit: in.ComboStickyLimit,
		StickyLimit:      in.StickyLimit,
	}
	if in.ComboStrategy != nil {
		if parsed, err := domain.ParseComboStrategy(*in.ComboStrategy); err == nil {
			patch.ComboStrategy = &parsed
		}
	}
	if in.FallbackStrategy != nil {
		if parsed, err := domain.ParseCredentialRotation(*in.FallbackStrategy); err == nil {
			patch.FallbackStrategy = &parsed
		}
	}
	if in.ProviderStrategies != nil {
		replaced := make(map[string]domain.ProviderStrategy, len(*in.ProviderStrategies))
		for id, override := range *in.ProviderStrategies {
			entry := domain.ProviderStrategy{StickyLimit: override.StickyLimit}
			if override.FallbackStrategy != nil {
				if parsed, err := domain.ParseCredentialRotation(*override.FallbackStrategy); err == nil {
					entry.FallbackStrategy = parsed
				}
			}
			replaced[id] = entry
		}
		patch.ProviderStrategies = &replaced
	}
	return patch
}

// strategiesOrEmpty renders the override map, never as null, so a panel
// round-trip cannot turn "no overrides" into a missing member.
func strategiesOrEmpty(in map[string]domain.ProviderStrategy) map[string]ProviderStrategyResponse {
	out := make(map[string]ProviderStrategyResponse, len(in))
	for id, override := range in {
		out[id] = ProviderStrategyResponse{
			FallbackStrategy: string(override.FallbackStrategy),
			StickyLimit:      override.StickyLimit,
		}
	}
	return out
}
