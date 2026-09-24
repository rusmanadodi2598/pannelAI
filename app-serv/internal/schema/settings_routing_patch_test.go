// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_routing_patch_test.go
// @for       The routing group's PATCH DTO: the per-key tags, the entry rule of
//
//	the provider override map, the lowering into the domain mutation,
//	and the response mapping of the two rotation keys.
//
// @uses      encoding/json, testing, internal/domain.
// @reason    SPEC-API-001 §7.14 makes this DTO the one door a rotation setting
//
//	enters through. AGENTS.md §2.1 requires the boundary proven: a value
//	outside the closed set must be refused before it reaches the
//	aggregate, and the whole-map replacement is a wire semantic the
//	panel's write depends on.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-24
package schema

import (
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// limitPtr is this file's pointer helper for the DTO literals.
func limitPtr(v int) *int { return &v }

// TestValidateStruct_GuardsTheRotationValues pins the tag layer: an unknown
// strategy and an out-of-range limit are refused at the boundary, in both the
// global key and a provider entry.
func TestValidateStruct_GuardsTheRotationValues(t *testing.T) {
	valid := PatchSettingsRequest{Routing: &RoutingSettingsPatch{
		FallbackStrategy:   strPtr("round-robin"),
		StickyLimit:        limitPtr(3),
		ProviderStrategies: &map[string]ProviderStrategyPatch{"alpha": {FallbackStrategy: strPtr("fill-first"), StickyLimit: limitPtr(1)}},
	}}
	if err := ValidateStruct(valid); err != nil {
		t.Fatalf("a well-formed routing patch must be accepted: %v", err)
	}

	cases := []struct {
		name  string
		patch PatchSettingsRequest
	}{
		{"an unknown global strategy", PatchSettingsRequest{Routing: &RoutingSettingsPatch{FallbackStrategy: strPtr("least-used")}}},
		{"a global limit above the bound", PatchSettingsRequest{Routing: &RoutingSettingsPatch{StickyLimit: limitPtr(domain.MaxStickyLimit + 1)}}},
		{"a global limit of zero", PatchSettingsRequest{Routing: &RoutingSettingsPatch{StickyLimit: limitPtr(0)}}},
		{"an unknown strategy in an entry", PatchSettingsRequest{Routing: &RoutingSettingsPatch{
			ProviderStrategies: &map[string]ProviderStrategyPatch{"alpha": {FallbackStrategy: strPtr("fill_first")}},
		}}},
		{"an entry limit above the bound", PatchSettingsRequest{Routing: &RoutingSettingsPatch{
			ProviderStrategies: &map[string]ProviderStrategyPatch{"alpha": {StickyLimit: limitPtr(101)}},
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

// TestValidatePatch_RejectsAnEmptyOverrideEntry pins the rule no tag can state:
// an entry that configures nothing is refused, because it would be configuration
// nobody reads; the reference's panel deletes the entry instead of writing one.
func TestValidatePatch_RejectsAnEmptyOverrideEntry(t *testing.T) {
	empty := map[string]ProviderStrategyPatch{"alpha": {}}
	if err := ValidatePatch(PatchSettingsRequest{Routing: &RoutingSettingsPatch{ProviderStrategies: &empty}}); err == nil {
		t.Fatal("an override entry with neither field must be refused")
	}

	partial := map[string]ProviderStrategyPatch{
		"alpha": {FallbackStrategy: strPtr("fill-first")},
		"beta":  {StickyLimit: limitPtr(2)},
	}
	if err := ValidatePatch(PatchSettingsRequest{Routing: &RoutingSettingsPatch{ProviderStrategies: &partial}}); err != nil {
		t.Fatalf("an entry setting one field must be accepted: %v", err)
	}
	if err := ValidatePatch(PatchSettingsRequest{Routing: &RoutingSettingsPatch{StickyLimit: limitPtr(4)}}); err != nil {
		t.Fatalf("a patch without the map must be accepted: %v", err)
	}
}

// TestToSettingsPatch_LowersTheWholeOverrideMap pins the boundary's translation:
// the strategy becomes the domain's own type, and the map arrives whole so the
// aggregate can replace rather than merge.
func TestToSettingsPatch_LowersTheWholeOverrideMap(t *testing.T) {
	entries := map[string]ProviderStrategyPatch{"alpha": {FallbackStrategy: strPtr("round-robin"), StickyLimit: limitPtr(4)}}
	patch := ToSettingsPatch(PatchSettingsRequest{Routing: &RoutingSettingsPatch{
		FallbackStrategy: strPtr("round-robin"), ProviderStrategies: &entries,
	}})

	if patch.Routing == nil {
		t.Fatal("routing patch = nil, want the lowered group")
	}
	if patch.Routing.FallbackStrategy == nil || *patch.Routing.FallbackStrategy != domain.RotationRoundRobin {
		t.Fatalf("fallback_strategy = %v, want the parsed round-robin", patch.Routing.FallbackStrategy)
	}
	if patch.Routing.ProviderStrategies == nil || len(*patch.Routing.ProviderStrategies) != 1 {
		t.Fatalf("provider_strategies = %v, want the one lowered entry", patch.Routing.ProviderStrategies)
	}
	entry := (*patch.Routing.ProviderStrategies)["alpha"]
	if entry.FallbackStrategy != domain.RotationRoundRobin || entry.StickyLimit == nil || *entry.StickyLimit != 4 {
		t.Fatalf("entry = %+v, want round-robin with sticky 4", entry)
	}

	if ToSettingsPatch(PatchSettingsRequest{}).Routing != nil {
		t.Fatal("an absent routing group must lower to nil, not to an empty patch")
	}
}

// TestSettingsResponseFrom_CarriesTheRotationKeys pins the read shape: the
// mapper renders the document the service hands it, and an empty override map
// renders as {} rather than null. The read path's normalization of a pre-change
// row is the service's rule, proven in settings_rotation_test.go.
func TestSettingsResponseFrom_CarriesTheRotationKeys(t *testing.T) {
	empty := domain.Settings{Routing: domain.RoutingSettings{
		ComboStrategy: domain.ComboFallback, StickyLimit: 3, FallbackStrategy: domain.RotationFillFirst,
	}}
	response := SettingsResponseFrom(empty)

	if response.Routing.FallbackStrategy != string(domain.RotationFillFirst) {
		t.Fatalf("fallback_strategy = %q, want fill-first", response.Routing.FallbackStrategy)
	}
	body, err := json.Marshal(response.Routing)
	if err != nil {
		t.Fatalf("marshalling the routing group: %v", err)
	}
	var rendered map[string]json.RawMessage
	if err := json.Unmarshal(body, &rendered); err != nil {
		t.Fatalf("decoding the routing group: %v", err)
	}
	if string(rendered["provider_strategies"]) != "{}" {
		t.Fatalf("provider_strategies = %s, want {}", rendered["provider_strategies"])
	}

	withOverride := domain.Settings{Routing: domain.RoutingSettings{
		ComboStrategy: domain.ComboFallback, StickyLimit: 3, FallbackStrategy: domain.RotationRoundRobin,
		ProviderStrategies: map[string]domain.ProviderStrategy{"alpha": {FallbackStrategy: domain.RotationFillFirst}},
	}}
	mapped := SettingsResponseFrom(withOverride).Routing
	if mapped.FallbackStrategy != string(domain.RotationRoundRobin) {
		t.Fatalf("fallback_strategy = %q, want the stored round-robin", mapped.FallbackStrategy)
	}
	if mapped.ProviderStrategies["alpha"].FallbackStrategy != string(domain.RotationFillFirst) {
		t.Fatalf("entry = %+v, want the stored fill-first", mapped.ProviderStrategies["alpha"])
	}
	if mapped.ProviderStrategies["alpha"].StickyLimit != nil {
		t.Fatal("an absent entry limit must stay absent on the wire, so the panel renders what is stored")
	}
}
