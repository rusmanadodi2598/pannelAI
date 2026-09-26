// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_reasoning_patch_test.go
// @for       The reasoning group's PATCH contract: the mode vocabulary pinned
//
//	against the domain, the entry rule, and the lowering.
//
// @uses      encoding/json, reflect, strings, testing, internal/domain.
// @reason    SPEC-API-001 §7.14 rejects a bad mode at the boundary, and the
//
//	tag is what does it; the vocabulary lives in the domain, so a mode
//	added to one without the other is a silent hole. AGENTS.md §2.1
//	requires the contract proven beside its DTO.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-26
package schema

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestProviderThinkingTag_MatchesTheDomainVocabulary pins the oneof tag against
// domain.ThinkingModes. The tag is what rejects an unstorable mode at the
// boundary and the slice is what the injection executes, so a mode added to one
// without the other would be accepted and then ignored.
func TestProviderThinkingTag_MatchesTheDomainVocabulary(t *testing.T) {
	field, ok := reflect.TypeOf(ProviderThinkingPatch{}).FieldByName("Mode")
	if !ok {
		t.Fatal("ProviderThinkingPatch carries no Mode field")
	}
	got := oneofValues(field.Tag.Get("validate"))
	if !reflect.DeepEqual(got, domain.ThinkingModes) {
		t.Fatalf("Mode oneof list = %v, want the domain vocabulary %v", got, domain.ThinkingModes)
	}
}

// TestValidateStruct_ReasoningModeRefusesAnOutOfSetValue pins the tag layer,
// which is what refuses a mode outside the vocabulary before ValidatePatch runs.
func TestValidateStruct_ReasoningModeRefusesAnOutOfSetValue(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"a level word", "high", false},
		{"an extended state", "off", false},
		{"the auto state", "auto", true},
		{"an upper-cased mode", "AUTO", true},
		{"a mode no format accepts", "ultra", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries := map[string]ProviderThinkingPatch{"openai": {Mode: &tc.mode}}
			req := PatchSettingsRequest{Reasoning: &ReasoningSettingsPatch{ProviderThinking: &entries}}
			err := ValidateStruct(req)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateStruct() = nil, want mode %q refused", tc.mode)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateStruct() = %v, want mode %q accepted", err, tc.mode)
			}
		})
	}
}

// TestValidatePatch_ReasoningEntryRule pins the rule the tags cannot express: an
// entry must name a mode, and an empty string is refused here because
// `omitempty` lets it past the oneof tag. The advice names the reference's own
// gesture, deleting the entry.
func TestValidatePatch_ReasoningEntryRule(t *testing.T) {
	cases := []struct {
		name    string
		entry   ProviderThinkingPatch
		wantErr bool
	}{
		{"a level word", ProviderThinkingPatch{Mode: strPtr("high")}, false},
		{"an extended state", ProviderThinkingPatch{Mode: strPtr("on")}, false},
		{"a missing mode", ProviderThinkingPatch{}, true},
		{"an empty mode", ProviderThinkingPatch{Mode: strPtr("")}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries := map[string]ProviderThinkingPatch{"openai": tc.entry}
			err := ValidatePatch(PatchSettingsRequest{Reasoning: &ReasoningSettingsPatch{ProviderThinking: &entries}})
			if tc.wantErr && err == nil {
				t.Fatal("ValidatePatch() error = nil, want the entry refused")
			}
			if !tc.wantErr {
				return
			}
			if !strings.Contains(err.Error(), "provider_thinking[openai]") {
				t.Fatalf("ValidatePatch() error = %v, want it to name the entry", err)
			}
		})
	}
	if err := ValidatePatch(PatchSettingsRequest{}); err != nil {
		t.Fatalf("ValidatePatch() refused a patch with no reasoning section: %v", err)
	}
}

// TestToSettingsPatch_LowersTheReasoningGroup pins the wire-to-domain mapping,
// including the whole-map copy and the empty map that clears every entry.
func TestToSettingsPatch_LowersTheReasoningGroup(t *testing.T) {
	if got := ToSettingsPatch(PatchSettingsRequest{}); got.Reasoning != nil {
		t.Fatalf("Reasoning = %+v, want nil for an absent section", got.Reasoning)
	}

	entries := map[string]ProviderThinkingPatch{"openai": {Mode: strPtr("max")}}
	got := ToSettingsPatch(PatchSettingsRequest{Reasoning: &ReasoningSettingsPatch{ProviderThinking: &entries}})
	if got.Reasoning == nil || got.Reasoning.ProviderThinking == nil {
		t.Fatal("Reasoning = nil, want the lowered map")
	}
	if mode := (*got.Reasoning.ProviderThinking)["openai"].Mode; mode != "max" {
		t.Fatalf("provider_thinking[openai].mode = %q, want max", mode)
	}

	empty := map[string]ProviderThinkingPatch{}
	cleared := ToSettingsPatch(PatchSettingsRequest{Reasoning: &ReasoningSettingsPatch{ProviderThinking: &empty}})
	if cleared.Reasoning.ProviderThinking == nil || len(*cleared.Reasoning.ProviderThinking) != 0 {
		t.Fatalf("ProviderThinking = %v, want an empty map rather than nil", cleared.Reasoning.ProviderThinking)
	}
}

// TestSettingsResponseFrom_RendersTheReasoningGroup pins the read: the group is
// always present and the map renders as an object, never null, so the panel
// renders every provider as "auto" instead of a missing member.
func TestSettingsResponseFrom_RendersTheReasoningGroup(t *testing.T) {
	encoded, err := json.Marshal(SettingsResponseFrom(domain.DefaultSettings()))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	raw, ok := decoded["reasoning"]
	if !ok {
		t.Fatal("settings response carries no reasoning group")
	}
	if !strings.Contains(string(raw), `"provider_thinking":{}`) {
		t.Fatalf("reasoning = %s, want provider_thinking rendered as an object", raw)
	}

	settings := domain.DefaultSettings()
	settings.Reasoning.ProviderThinking = map[string]domain.ProviderThinking{"openai": {Mode: domain.ThinkingOff}}
	encoded, err = json.Marshal(SettingsResponseFrom(settings))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"provider_thinking":{"openai":{"mode":"off"}}`) {
		t.Fatalf("reasoning = %s, want the stored mode rendered", encoded)
	}
}
