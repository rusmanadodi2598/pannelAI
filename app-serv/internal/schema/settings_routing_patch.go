// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_routing_patch.go
// @for       The routing group's PATCH DTO, its lowering into the domain mutation, and the per-entry rules of the provider override map.
// @uses      internal/domain, time.
// @reason    SPEC-API-001 §7.14 validates a PATCH per key. The provider override map is the one value whose entries carry their own rules, so keeping its DTO and its checks together is what stops the settings patch file from growing past the line budget while the group stays one reviewable unit.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-09-24
package schema

import (
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

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
	KeyPark            *KeyParkPatch                     `json:"key_park,omitempty"`
	// Failover bounds one request's whole credential walk. The bounds sit here
	// rather than in a provider entry because the chain it cuts belongs to the
	// request: a combo, its accounts, and their keys.
	Failover *FailoverBudgetPatch `json:"failover,omitempty"`
}

// FailoverBudgetPatch is the walk budget as the wire carries it. Each bound is
// optional on its own, because widening the attempt count must not reset the clock.
// The ceilings match the domain's: the total stops where one transport call is
// already allowed to run, and an attempt count that could walk every account of
// every provider is not a bound.
type FailoverBudgetPatch struct {
	TotalMS     *int `json:"total_ms,omitempty" validate:"omitempty,min=1000,max=120000"`
	MaxAttempts *int `json:"max_attempts,omitempty" validate:"omitempty,min=1,max=200"`
}

// KeyParkPatch is one level of the parking budget: the group's own windows or a
// provider's override. Each window is optional on its own, because retuning one
// failure class must not reset the other two. The bounds mirror the domain's
// (MaxKeyParkAuthSeconds, MaxKeyParkRateLimit, MaxKeyParkTransient), which is the
// authority a stored document is checked against.
type KeyParkPatch struct {
	AuthSeconds      *int `json:"auth_seconds,omitempty" validate:"omitempty,min=1,max=900"`
	RateLimitSeconds *int `json:"rate_limit_seconds,omitempty" validate:"omitempty,min=1,max=300"`
	TransientSeconds *int `json:"transient_seconds,omitempty" validate:"omitempty,min=1,max=300"`
	BillingSeconds   *int `json:"billing_seconds,omitempty" validate:"omitempty,min=1,max=3600"`
}

// ProviderStrategyPatch is one provider's override as the wire carries it.
type ProviderStrategyPatch struct {
	FallbackStrategy *string       `json:"fallback_strategy,omitempty" validate:"omitempty,oneof=fill-first round-robin"`
	StickyLimit      *int          `json:"sticky_limit,omitempty" validate:"omitempty,min=1,max=100"`
	KeyPark          *KeyParkPatch `json:"key_park,omitempty"`
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
		if override.FallbackStrategy == nil && override.StickyLimit == nil && override.KeyPark == nil {
			return domain.NewValidationError("provider_strategies[" + id +
				"] must set fallback_strategy, sticky_limit, or key_park; delete the entry to inherit the default")
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
			entry := domain.ProviderStrategy{StickyLimit: override.StickyLimit, KeyPark: keyPark(override.KeyPark)}
			if override.FallbackStrategy != nil {
				if parsed, err := domain.ParseCredentialRotation(*override.FallbackStrategy); err == nil {
					entry.FallbackStrategy = parsed
				}
			}
			replaced[id] = entry
		}
		patch.ProviderStrategies = &replaced
	}
	if in.KeyPark != nil {
		parked := keyParkPatch(*in.KeyPark)
		patch.KeyPark = &parked
	}
	if in.Failover != nil {
		patch.Failover = &domain.FailoverBudgetPatch{
			TotalMS:     in.Failover.TotalMS,
			MaxAttempts: in.Failover.MaxAttempts,
		}
	}
	return patch
}

// keyParkPatch lowers one wire parking entry into the domain mutation.
func keyParkPatch(in KeyParkPatch) domain.KeyParkPatch {
	return domain.KeyParkPatch{
		AuthSeconds:      in.AuthSeconds,
		RateLimitSeconds: in.RateLimitSeconds,
		TransientSeconds: in.TransientSeconds,
		BillingSeconds:   in.BillingSeconds,
	}
}

// keyPark lowers a provider entry's override into stored windows. A nil entry
// names no window, which is what an absent key_park means: inherit them all.
func keyPark(in *KeyParkPatch) domain.KeyParkWindows {
	windows := domain.KeyParkWindows{}
	if in == nil {
		return windows
	}
	if in.AuthSeconds != nil {
		windows.AuthSeconds = *in.AuthSeconds
	}
	if in.RateLimitSeconds != nil {
		windows.RateLimitSeconds = *in.RateLimitSeconds
	}
	if in.TransientSeconds != nil {
		windows.TransientSeconds = *in.TransientSeconds
	}
	if in.BillingSeconds != nil {
		windows.BillingSeconds = *in.BillingSeconds
	}
	return windows
}

// strategiesOrEmpty renders the override map, never as null, so a panel
// round-trip cannot turn "no overrides" into a missing member.
func strategiesOrEmpty(in map[string]domain.ProviderStrategy) map[string]ProviderStrategyResponse {
	out := make(map[string]ProviderStrategyResponse, len(in))
	for id, override := range in {
		entry := ProviderStrategyResponse{
			FallbackStrategy: string(override.FallbackStrategy),
			StickyLimit:      override.StickyLimit,
		}
		if !override.KeyPark.IsEmpty() {
			parked := keyParkResponse(override.KeyPark)
			entry.KeyPark = &parked
		}
		out[id] = entry
	}
	return out
}

// keyParkResponse renders stored seconds, so a provider override reads back as the
// windows it names and nothing else.
func keyParkResponse(in domain.KeyParkWindows) KeyParkResponse {
	return KeyParkResponse{
		AuthSeconds:      in.AuthSeconds,
		RateLimitSeconds: in.RateLimitSeconds,
		TransientSeconds: in.TransientSeconds,
		BillingSeconds:   in.BillingSeconds,
	}
}

// effectiveKeyPark renders the group's windows as the durations the router will
// actually park with, which is what a read of a document that never set them has
// to answer.
func effectiveKeyPark(in domain.KeyParkWindows) KeyParkResponse {
	policy := in.Policy()
	return KeyParkResponse{
		AuthSeconds:      int(policy.AuthWindow / time.Second),
		RateLimitSeconds: int(policy.RateLimitWindow / time.Second),
		TransientSeconds: int(policy.TransientWindow / time.Second),
		BillingSeconds:   int(policy.BillingWindow / time.Second),
	}
}

// effectiveFailover answers the walk budget the router really applies, so a read of
// a document that predates the keys names the bound rather than a zero an operator
// would read as "no walk is allowed".
func effectiveFailover(in domain.FailoverBudget) FailoverBudgetResponse {
	budget := in.Normalized()
	return FailoverBudgetResponse{
		TotalMS:     budget.TotalMS,
		MaxAttempts: budget.MaxAttempts,
	}
}
