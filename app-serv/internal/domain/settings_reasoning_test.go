// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_reasoning_test.go
// @for       The reasoning group's vocabulary, its per-provider resolution, and
//
//	the whole-map patch rule.
//
// @uses      strings, testing.
// @reason    SPEC-API-001 §7.14 stores one thinking mode per provider, and the
//
//	data plane injects it only when the client carries no reasoning
//	intent of its own. AGENTS.md §2.1 requires the rule proven beside the
//	group: a mode the injection cannot execute must never be stored, and
//	"auto" must stay the absence of an entry rather than a value.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-26
package domain

import (
	"strings"
	"testing"
)

// TestThinkingModes_CarryEveryStorableMode pins the vocabulary: the two
// extended-thinking states and the union of level words, no duplicates, and
// "auto" absent because it means the entry does not exist.
func TestThinkingModes_CarryEveryStorableMode(t *testing.T) {
	if len(ThinkingModes) != 10 {
		t.Fatalf("ThinkingModes carries %d modes, want the ten documented ones", len(ThinkingModes))
	}
	seen := map[string]bool{}
	for _, mode := range ThinkingModes {
		if seen[mode] {
			t.Fatalf("ThinkingModes lists %q twice", mode)
		}
		seen[mode] = true
		if !ValidThinkingMode(ThinkingMode(mode)) {
			t.Fatalf("ThinkingModes lists %q but ValidThinkingMode refuses it", mode)
		}
	}
	for _, state := range []ThinkingMode{ThinkingOn, ThinkingOff} {
		if !ValidThinkingMode(state) {
			t.Fatalf("ValidThinkingMode(%q) = false, want the extended-thinking state accepted", state)
		}
	}
	for _, refused := range []ThinkingMode{"", "auto", "AUTO", "ultra", "none "} {
		if ValidThinkingMode(refused) {
			t.Fatalf("ValidThinkingMode(%q) = true, want it refused", refused)
		}
	}
}

// TestReasoningSettings_ThinkingForResolvesStoredEntry pins the read the data
// plane makes: an entry resolves to its mode, an absent or unstorable one
// resolves to nothing, and both answer the same way so the injection has one
// branch rather than two.
func TestReasoningSettings_ThinkingForResolvesStoredEntry(t *testing.T) {
	settings := ReasoningSettings{ProviderThinking: map[string]ProviderThinking{
		"openai":    {Mode: ThinkingOn},
		"anthropic": {Mode: "low"},
		"broken":    {Mode: "auto"},
	}}
	cases := []struct {
		provider string
		want     ThinkingMode
		wantOK   bool
	}{
		{"openai", ThinkingOn, true},
		{"anthropic", "low", true},
		{"groq", "", false},
		{"broken", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			got, ok := settings.ThinkingFor(tc.provider)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("ThinkingFor(%s) = (%q, %v), want (%q, %v)",
					tc.provider, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestReasoningSettings_NormalizedFillsTheMap pins the read path for a row
// written before the group existed: the decode replaces the group wholesale, so
// the absent map arrives nil and the panel must still render an object.
func TestReasoningSettings_NormalizedFillsTheMap(t *testing.T) {
	if got := (ReasoningSettings{}).Normalized(); got.ProviderThinking == nil {
		t.Fatal("Normalized() left the map nil, want an empty map")
	}
	stored := ReasoningSettings{ProviderThinking: map[string]ProviderThinking{"openai": {Mode: ThinkingOff}}}
	if got := stored.Normalized(); got.ProviderThinking["openai"].Mode != ThinkingOff {
		t.Fatalf("Normalized() rewrote a stored entry: %+v", got)
	}
}

// TestSettingsUpdate_ReasoningReplacesTheMapWhole pins the patch semantics: the
// map is written whole so a deleted entry stays deleted, an empty map clears
// every provider, and a mode the injection cannot execute is refused.
func TestSettingsUpdate_ReasoningReplacesTheMapWhole(t *testing.T) {
	settings := DefaultSettings()
	first := map[string]ProviderThinking{"openai": {Mode: "high"}, "groq": {Mode: ThinkingOn}}
	if err := settings.Update(SettingsPatch{Reasoning: &ReasoningSettingsPatch{ProviderThinking: &first}}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got := settings.Reasoning.ProviderThinking; len(got) != 2 || got["openai"].Mode != "high" {
		t.Fatalf("provider_thinking = %+v, want the two stored entries", got)
	}

	second := map[string]ProviderThinking{"openai": {Mode: "max"}}
	if err := settings.Update(SettingsPatch{Reasoning: &ReasoningSettingsPatch{ProviderThinking: &second}}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got := settings.Reasoning.ProviderThinking; len(got) != 1 || got["openai"].Mode != "max" {
		t.Fatalf("provider_thinking = %+v, want groq deleted by the whole-map write", got)
	}

	empty := map[string]ProviderThinking{}
	if err := settings.Update(SettingsPatch{Reasoning: &ReasoningSettingsPatch{ProviderThinking: &empty}}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got := settings.Reasoning.ProviderThinking; len(got) != 0 {
		t.Fatalf("provider_thinking = %+v, want the empty map to clear every entry", got)
	}
}

// TestSettingsUpdate_ReasoningRefusesAnUnstorableMode pins the entry rule and
// the advice the reference's panel already follows: delete the entry instead of
// storing "auto".
func TestSettingsUpdate_ReasoningRefusesAnUnstorableMode(t *testing.T) {
	for _, mode := range []ThinkingMode{"", "auto", "ultra"} {
		t.Run("mode="+string(mode), func(t *testing.T) {
			settings := DefaultSettings()
			entry := map[string]ProviderThinking{"openai": {Mode: mode}}
			err := settings.Update(SettingsPatch{Reasoning: &ReasoningSettingsPatch{ProviderThinking: &entry}})
			if err == nil {
				t.Fatalf("Update() accepted mode %q", mode)
			}
			if !strings.Contains(err.Error(), "delete the entry to follow the client") {
				t.Fatalf("Update() error = %v, want the delete-the-entry advice", err)
			}
		})
	}
}
