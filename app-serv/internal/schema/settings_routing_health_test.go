// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_routing_health_test.go
// @for       The key-park PATCH DTO: its bounds, its lowering into the domain mutation, and the two ways the response renders the budget.
// @uses      encoding/json, testing, internal/domain.
// @reason    SPEC-API-001 §7.14 makes this DTO the only door a parking window enters through, and draft 050 T8 opened it. The group and a provider entry mean different things by the same key name, so the read directions are pinned here: one answers what the router will do, the other answers what was stored.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-13
package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestValidateStruct_GuardsTheKeyParkBounds pins the tag layer: a window outside
// its range is refused at the boundary, in the group and in a provider entry, and
// zero is not a legal thing to ask for.
func TestValidateStruct_GuardsTheKeyParkBounds(t *testing.T) {
	accepted := PatchSettingsRequest{Routing: &RoutingSettingsPatch{
		KeyPark: &KeyParkPatch{AuthSeconds: limitPtr(domain.MaxKeyParkAuthSeconds)},
		ProviderStrategies: &map[string]ProviderStrategyPatch{
			"alpha": {KeyPark: &KeyParkPatch{TransientSeconds: limitPtr(45)}},
		},
	}}
	if err := ValidateStruct(accepted); err != nil {
		t.Fatalf("a well-formed parking patch must be accepted: %v", err)
	}

	cases := []struct {
		name  string
		patch PatchSettingsRequest
	}{
		{"a group window past its ceiling", PatchSettingsRequest{Routing: &RoutingSettingsPatch{
			KeyPark: &KeyParkPatch{RateLimitSeconds: limitPtr(domain.MaxKeyParkRateLimit + 1)},
		}}},
		{"a group window of zero", PatchSettingsRequest{Routing: &RoutingSettingsPatch{
			KeyPark: &KeyParkPatch{AuthSeconds: limitPtr(0)},
		}}},
		{"a billing window past its ceiling", PatchSettingsRequest{Routing: &RoutingSettingsPatch{
			KeyPark: &KeyParkPatch{BillingSeconds: limitPtr(domain.MaxKeyParkBilling + 1)},
		}}},
		{"an entry window past its ceiling", PatchSettingsRequest{Routing: &RoutingSettingsPatch{
			ProviderStrategies: &map[string]ProviderStrategyPatch{
				"alpha": {KeyPark: &KeyParkPatch{TransientSeconds: limitPtr(domain.MaxKeyParkTransient + 1)}},
			},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateStruct(tc.patch); err == nil {
				t.Fatalf("ValidateStruct = nil for %q, want a rejection", tc.name)
			}
		})
	}
}

// TestValidatePatch_KeyParkAloneMakesAnEntryLive keeps the entry rule and the new
// key in agreement: an override that only retunes a window is configuration.
func TestValidatePatch_KeyParkAloneMakesAnEntryLive(t *testing.T) {
	live := RoutingSettingsPatch{ProviderStrategies: &map[string]ProviderStrategyPatch{
		"alpha": {KeyPark: &KeyParkPatch{AuthSeconds: limitPtr(60)}},
	}}
	if err := validateProviderStrategies(&live); err != nil {
		t.Fatalf("validateProviderStrategies(key_park only) error = %v, want the entry accepted", err)
	}
	dead := RoutingSettingsPatch{ProviderStrategies: &map[string]ProviderStrategyPatch{"alpha": {}}}
	if err := validateProviderStrategies(&dead); err == nil {
		t.Fatal("validateProviderStrategies(empty entry) = nil error, want a rejection")
	}
}

// TestRoutingPtr_LowersKeyParkPerLevel checks the two directions a parking entry
// has to survive: the group's own windows become a patch that names pointers, and
// a provider entry's windows become stored seconds with the unnamed ones left at
// zero so the entry still inherits them.
func TestRoutingPtr_LowersKeyParkPerLevel(t *testing.T) {
	parked := limitPtr(90)
	transient := limitPtr(7)
	lowered := routingPtr(&RoutingSettingsPatch{
		KeyPark: &KeyParkPatch{AuthSeconds: parked},
		ProviderStrategies: &map[string]ProviderStrategyPatch{
			"alpha": {KeyPark: &KeyParkPatch{TransientSeconds: transient}},
			"beta":  {},
		},
	})
	if lowered.KeyPark == nil || lowered.KeyPark.AuthSeconds == nil || *lowered.KeyPark.AuthSeconds != 90 {
		t.Fatalf("group key_park = %+v, want auth_seconds 90", lowered.KeyPark)
	}
	if lowered.KeyPark.RateLimitSeconds != nil {
		t.Fatal("an unnamed rate limit must stay unset, not become zero")
	}
	entries := *lowered.ProviderStrategies
	if entries["alpha"].KeyPark != (domain.KeyParkWindows{TransientSeconds: 7}) {
		t.Fatalf("alpha key_park = %+v, want only transient_seconds", entries["alpha"].KeyPark)
	}
	if !entries["beta"].KeyPark.IsEmpty() {
		t.Fatalf("beta key_park = %+v, want every window inherited", entries["beta"].KeyPark)
	}
}

// TestSettingsResponseFrom_RendersTheBudgetBothWays is the panel's contract: the
// group answers the lengths the router will use even when nothing was configured,
// and a provider entry answers only what it actually stores.
func TestSettingsResponseFrom_RendersTheBudgetBothWays(t *testing.T) {
	settings := domain.Settings{}
	settings.Routing = domain.RoutingSettings{
		ComboStrategy: domain.ComboFallback, ComboStickyLimit: 1, StickyLimit: domain.DefaultStickyLimit,
		FallbackStrategy: domain.RotationFillFirst,
		ProviderStrategies: map[string]domain.ProviderStrategy{
			"alpha": {KeyPark: domain.KeyParkWindows{TransientSeconds: 45}},
		},
	}

	rendered := SettingsResponseFrom(settings)
	if got := rendered.Routing.KeyPark; got.AuthSeconds != 120 || got.RateLimitSeconds != 2 ||
		got.TransientSeconds != 30 || got.BillingSeconds != 900 {
		t.Fatalf("group key_park = %+v, want the effective 120/2/900/30", got)
	}
	entry := rendered.Routing.ProviderStrategies["alpha"]
	if entry.KeyPark == nil {
		t.Fatal("the provider entry lost its key_park")
	}
	if *entry.KeyPark != (KeyParkResponse{TransientSeconds: 45}) {
		t.Fatalf("entry key_park = %+v, want the stored transient_seconds alone", *entry.KeyPark)
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, absent := range []string{"auth_seconds", "rate_limit_seconds"} {
		if strings.Contains(string(encoded), absent) {
			t.Fatalf("entry rendered %q, which it never stored: %s", absent, encoded)
		}
	}
}
