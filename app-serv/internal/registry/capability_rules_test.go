// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_rules_test.go
// @for       The capability resolver's own rules: the provider layer that
//
//	overrides the tables, the boundary inputs, and the membership test the
//	catalog filter reads.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.6 offers `?capability=vision|tools` and §7.8
//
//	refuses a vision adapter whose models cannot read images. The corpus
//	test in capability_resolve_test.go proves the port agrees with the
//	reference's own function; these cases prove the rules that corpus
//	cannot exercise, because an absent, padded, or unknown id never
//	appears in a generated row.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package registry

import "testing"

// TestCapabilities_ProviderLayerIsLoadBearing pins the fact that made the
// provider layer part of the port: the reference consults PROVIDER_CAPABILITIES
// before its exact and pattern layers (capabilities.js:586-590), and at the
// pinned revision that layer changes the vision answer for models the embedded
// registry declares. The first version of this port omitted the layer because
// the older pin measured zero such rows; the corpus now proves the opposite, so
// the test asserts the layer is reached rather than that it is harmless.
//
// The list is deliberately small and named: these are the rows that fail if the
// layer is dropped, so a reader sees exactly what the layer buys.
func TestCapabilities_ProviderLayerIsLoadBearing(t *testing.T) {
	cases := []struct {
		provider string
		model    string
		want     bool
	}{
		// codebuddy-cn reads images on every model it proxies, including the
		// ones whose family pattern would answer false.
		{provider: "codebuddy-cn", model: "deepseek-v4-pro", want: true},
		{provider: "codebuddy-cn", model: "glm-5.2", want: true},
		// nvidia's MiniMax M3 is multimodal while the same family under the
		// commandcode wire is text-only, so the answer depends on the provider.
		{provider: "nvidia", model: "minimaxai/minimax-m3", want: true},
		{provider: "commandcode", model: "MiniMaxAI/MiniMax-M2.5", want: false},
		// The name heuristic must not rescue it: M2.5 carries no modality word,
		// and the denylist is consulted before the pattern table either way.
		{provider: "commandcode", model: "Qwen/Qwen3.7-Max", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.provider+"/"+tc.model, func(t *testing.T) {
			got := Capabilities(tc.provider, tc.model)
			if got.Vision != tc.want {
				t.Fatalf("Capabilities(%q, %q).Vision = %v, want %v; the provider layer is load-bearing at this revision",
					tc.provider, tc.model, got.Vision, tc.want)
			}
		})
	}
}

// TestCapabilities_BoundaryInputs covers the inputs the corpus cannot carry: an
// absent model, whitespace, a vendor-prefixed id, an id that is itself a path,
// and an id nothing knows.
func TestCapabilities_BoundaryInputs(t *testing.T) {
	// Every floor answer carries CanDisable true, because the reference's floor
	// lets a model turn thinking off and only a table row can say otherwise.
	floor := CapabilitySet{Vision: false, Tools: true, CanDisable: true}
	cases := []struct {
		name     string
		provider string
		model    string
		want     CapabilitySet
	}{
		{"an empty model answers the floor", "openai", "", floor},
		{"whitespace is trimmed, not matched", "openai", "   ", floor},
		{
			name: "a vendor prefix is stripped before matching", provider: "openrouter", model: "anthropic/claude-opus-4-8",
			want: CapabilitySet{Vision: true, Tools: true, Reasoning: true, ThinkingFormat: "claude-adaptive", CanDisable: true},
		},
		{"a model id may itself be a path", "", "vendor/family/model", floor},
		{"an image model declares no tools", "openai", "gpt-image-1", CapabilitySet{Vision: false, Tools: false, CanDisable: true}},
		{"an unknown id answers the floor", "", "totally-unknown-9000", floor},
		{"the match ignores case", "openai", "GPT-4O", CapabilitySet{Vision: true, Tools: true, CanDisable: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Capabilities(tc.provider, tc.model); got != tc.want {
				t.Fatalf("Capabilities(%q, %q) = %+v, want %+v", tc.provider, tc.model, got, tc.want)
			}
		})
	}
}

// TestCapabilitySet_Has covers the membership rule the catalog filter uses: the
// two names it resolves, a media string it does not, and an unknown name.
func TestCapabilitySet_Has(t *testing.T) {
	cases := []struct {
		name string
		set  CapabilitySet
		want map[string]bool
	}{
		{
			name: "a vision model answers vision and tools",
			set:  CapabilitySet{Vision: true, Tools: true},
			want: map[string]bool{"vision": true, "tools": true, "edit": false, "": false, "Vision": true},
		},
		{
			name: "a padded name is trimmed before the switch",
			set:  CapabilitySet{Vision: true, Tools: true},
			want: map[string]bool{"  tools  ": true, "\tvision\n": true},
		},
		{
			name: "a floor model answers tools only",
			set:  CapabilitySet{Vision: false, Tools: true},
			want: map[string]bool{"vision": false, "tools": true, "edit": false},
		},
		{
			name: "an image model answers neither",
			set:  CapabilitySet{Vision: false, Tools: false},
			want: map[string]bool{"vision": false, "tools": false},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for name, want := range tc.want {
				if got := tc.set.Has(name); got != want {
					t.Fatalf("CapabilitySet{%+v}.Has(%q) = %v, want %v", tc.set, name, got, want)
				}
			}
		})
	}
}
