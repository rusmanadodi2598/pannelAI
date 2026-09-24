// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing_test.go
// @for       The credential rotation vocabulary, the per-provider override
//
//	resolution, the backfill of rows written before the keys existed,
//	and the routing group's validation rules.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.5 and §7.14 make the override-over-global rule the
//
//	one the data plane's selector depends on. AGENTS.md §2.1 requires
//	it pinned beside the implementation: the resolution is where a
//	panel write and the router's walk must agree, and the degradation
//	rules are what keep a rotation setting from failing a request.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-24
package domain

import (
	"reflect"
	"testing"
)

// TestCredentialRotation_Vocabulary pins the closed set: the two modes the
// selector executes, the parse that guards the wire, and the fact that only
// round-robin reads rotation state.
func TestCredentialRotation_Vocabulary(t *testing.T) {
	for _, value := range []string{"fill-first", "round-robin"} {
		mode, err := ParseCredentialRotation(value)
		if err != nil {
			t.Fatalf("ParseCredentialRotation(%q) error = %v", value, err)
		}
		if string(mode) != value {
			t.Fatalf("parsed mode = %q, want %q", mode, value)
		}
	}
	for _, value := range []string{"", "round_robin", "fill_first", "least-used", "FILL-FIRST"} {
		if _, err := ParseCredentialRotation(value); err == nil {
			t.Fatalf("ParseCredentialRotation(%q) = nil error, want a rejection", value)
		}
	}
	if !RotationRoundRobin.UsesRotation() {
		t.Fatal("round-robin must read the cursor and the sticky limit")
	}
	if RotationFillFirst.UsesRotation() {
		t.Fatal("fill-first must read no rotation state")
	}
	if (RotationPolicy{Strategy: RotationRoundRobin, StickyLimit: 1}).UsesRotation() != true {
		t.Fatal("a round-robin policy must report that it rotates")
	}
}

// TestRoutingSettings_RotationForResolvesOverrideOverGlobal pins the reference's
// resolution rule: the provider's own entry wins field by field, an absent entry
// or field inherits the global, and an unreadable value degrades to fill-first
// instead of failing the selector's request.
func TestRoutingSettings_RotationForResolvesOverrideOverGlobal(t *testing.T) {
	limit := func(v int) *int { return &v }
	global := RoutingSettings{
		StickyLimit:        5,
		FallbackStrategy:   RotationRoundRobin,
		ProviderStrategies: map[string]ProviderStrategy{},
	}

	cases := []struct {
		name    string
		entries map[string]ProviderStrategy
		want    RotationPolicy
	}{
		{"no entry inherits the global", nil, RotationPolicy{RotationRoundRobin, 5}},
		{"another provider's entry is not consulted",
			map[string]ProviderStrategy{"other": {FallbackStrategy: RotationFillFirst}}, RotationPolicy{RotationRoundRobin, 5}},
		{"an override takes the strategy", map[string]ProviderStrategy{"alpha": {FallbackStrategy: RotationFillFirst}},
			RotationPolicy{RotationFillFirst, 5}},
		{"an override takes the limit", map[string]ProviderStrategy{"alpha": {StickyLimit: limit(9)}},
			RotationPolicy{RotationRoundRobin, 9}},
		{"both fields of an override win", map[string]ProviderStrategy{"alpha": {FallbackStrategy: RotationFillFirst, StickyLimit: limit(1)}},
			RotationPolicy{RotationFillFirst, 1}},
		{"an override with no limit keeps the global's", map[string]ProviderStrategy{"alpha": {FallbackStrategy: RotationRoundRobin}},
			RotationPolicy{RotationRoundRobin, 5}},
		{"an unreadable stored limit inherits the global's", map[string]ProviderStrategy{"alpha": {StickyLimit: limit(0)}},
			RotationPolicy{RotationRoundRobin, 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := global
			settings.ProviderStrategies = tc.entries
			if got := settings.RotationFor("alpha"); got != tc.want {
				t.Fatalf("RotationFor(alpha) = %+v, want %+v", got, tc.want)
			}
		})
	}

	degraded := RoutingSettings{StickyLimit: 0, FallbackStrategy: "least-used"}
	if got := degraded.RotationFor("alpha"); got != (RotationPolicy{RotationFillFirst, DefaultStickyLimit}) {
		t.Fatalf("unreadable stored values = %+v, want fill-first with the documented sticky", got)
	}
}

// TestRoutingSettings_NormalizedBackfillsPreChangeRows pins the read path for a
// stored row written before §7.14 carried these keys: the group decode replaces
// the group wholesale, so the absent keys arrive as zero values and a read that
// answered them as-is would fail the panel's own form validation.
func TestRoutingSettings_NormalizedBackfillsPreChangeRows(t *testing.T) {
	var predates RoutingSettings
	got := predates.Normalized()
	if got.FallbackStrategy != RotationFillFirst {
		t.Fatalf("fallback_strategy = %q, want fill-first", got.FallbackStrategy)
	}
	if got.StickyLimit != DefaultStickyLimit {
		t.Fatalf("sticky_limit = %d, want %d", got.StickyLimit, DefaultStickyLimit)
	}
	if got.ProviderStrategies == nil {
		t.Fatal("provider_strategies = nil, want an empty map so the read renders {}")
	}

	stored := RoutingSettings{StickyLimit: 7, FallbackStrategy: RotationRoundRobin,
		ProviderStrategies: map[string]ProviderStrategy{"alpha": {FallbackStrategy: RotationFillFirst}}}
	if !reflect.DeepEqual(stored.Normalized(), stored) {
		t.Fatal("Normalized() rewrote a row that already carried every key")
	}
}

// TestRoutingSettings_ValidateRejectsBadOverrides covers the rules the per-key
// struct tags cannot state: an entry that configures nothing, and a stored
// value outside the closed sets.
func TestRoutingSettings_ValidateRejectsBadOverrides(t *testing.T) {
	limit := func(v int) *int { return &v }
	valid := func() RoutingSettings {
		s := DefaultSettings().Routing
		s.ProviderStrategies = map[string]ProviderStrategy{"alpha": {FallbackStrategy: RotationRoundRobin, StickyLimit: limit(4)}}
		return s
	}

	if err := validateRouting(valid()); err != nil {
		t.Fatalf("a well-formed override must be accepted: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*RoutingSettings)
	}{
		{"an entry that sets neither field", func(s *RoutingSettings) {
			s.ProviderStrategies = map[string]ProviderStrategy{"alpha": {}}
		}},
		{"an unknown strategy in an override", func(s *RoutingSettings) {
			s.ProviderStrategies = map[string]ProviderStrategy{"alpha": {FallbackStrategy: "least-used"}}
		}},
		{"a limit above the bound in an override", func(s *RoutingSettings) {
			s.ProviderStrategies = map[string]ProviderStrategy{"alpha": {StickyLimit: limit(MaxStickyLimit + 1)}}
		}},
		{"a limit below one in an override", func(s *RoutingSettings) {
			s.ProviderStrategies = map[string]ProviderStrategy{"alpha": {StickyLimit: limit(0)}}
		}},
		{"an unknown global strategy", func(s *RoutingSettings) { s.FallbackStrategy = "least-used" }},
		{"a global limit below one", func(s *RoutingSettings) { s.StickyLimit = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := valid()
			tc.mutate(&settings)
			if err := validateRouting(settings); err == nil {
				t.Fatalf("Validate = nil for %q, want a rejection", tc.name)
			}
		})
	}
}

// TestSettings_UpdateReplacesTheProviderOverrideMap pins the map's write rule:
// it is a whole replacement when present, so an empty map clears every override
// while an absent key leaves the stored map alone, which is what the reference's
// PATCH does with its providerStrategies member.
func TestSettings_UpdateReplacesTheProviderOverrideMap(t *testing.T) {
	two := map[string]ProviderStrategy{
		"alpha": {FallbackStrategy: RotationRoundRobin},
		"beta":  {StickyLimit: stickyPtr(2)},
	}
	one := map[string]ProviderStrategy{"beta": {StickyLimit: stickyPtr(2)}}
	none := map[string]ProviderStrategy{}

	settings := DefaultSettings()
	if err := settings.Update(SettingsPatch{Routing: &RoutingSettingsPatch{ProviderStrategies: &two}}); err != nil {
		t.Fatalf("Update(two) error = %v", err)
	}
	if len(settings.Routing.ProviderStrategies) != 2 {
		t.Fatalf("overrides = %v, want both entries", settings.Routing.ProviderStrategies)
	}

	if err := settings.Update(SettingsPatch{Routing: &RoutingSettingsPatch{StickyLimit: stickyPtr(9)}}); err != nil {
		t.Fatalf("Update(limit only) error = %v", err)
	}
	if len(settings.Routing.ProviderStrategies) != 2 {
		t.Fatal("a patch without the map must leave every override in place")
	}

	if err := settings.Update(SettingsPatch{Routing: &RoutingSettingsPatch{ProviderStrategies: &one}}); err != nil {
		t.Fatalf("Update(one) error = %v", err)
	}
	if len(settings.Routing.ProviderStrategies) != 1 {
		t.Fatalf("overrides = %v, want the whole map replaced by one entry", settings.Routing.ProviderStrategies)
	}

	if err := settings.Update(SettingsPatch{Routing: &RoutingSettingsPatch{ProviderStrategies: &none}}); err != nil {
		t.Fatalf("Update(empty) error = %v", err)
	}
	if len(settings.Routing.ProviderStrategies) != 0 || settings.Routing.ProviderStrategies == nil {
		t.Fatalf("overrides = %v, want an empty map rather than nil", settings.Routing.ProviderStrategies)
	}
}

// stickyPtr is the pointer helper for this file's override fixtures.
func stickyPtr(v int) *int { return &v }
