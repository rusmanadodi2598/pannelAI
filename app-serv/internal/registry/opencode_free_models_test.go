// Package registry loads the provider registry and answers lookups over it.
//
// @file      internal/registry/opencode_free_models_test.go
// @for       The OpenCode Free entry's declared models, pinned to the reference.
// @uses      testing.
// @reason    The port dropped the reference's static model list for opencode, so its free models were answerable through passthrough yet absent from every models list, and resolved onto the provider's OpenAI format instead of the per-model formats the reference declares (9router PR #4073). Pinning the entry here keeps the catalog and the reference from drifting apart again.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-20
package registry

import "testing"

// TestEmbeddedOpenCode_FreeModels pins the opencode entry's declared models to
// the reference registry (open-sse/providers/registry/opencode.js): id, display
// name, upstream wire format, chat eligibility. Four models are declared and
// only three are chat: the fourth is a System One decision model
// (`kind: "systemone"`) whose payload is the provider's own vocabulary.
// Carrying it keeps the catalog truthful about the id and the kind keeps the
// chat plane off it; both are asserted together so a future edit cannot quietly
// promote the decision model back to chat.
func TestEmbeddedOpenCode_FreeModels(t *testing.T) {
	index, err := Load()
	if err != nil {
		t.Fatalf("loading embedded registry: %v", err)
	}
	provider, ok := index.Provider("opencode")
	if !ok {
		t.Fatal("opencode is missing from the embedded registry")
	}

	cases := []struct {
		id           string
		name         string
		targetFormat string
		chat         bool
		// minOutput is the smallest output ceiling the model still answers
		// within: muse-spark 1.3 spends a small one entirely on thinking and
		// answers an empty body.
		minOutput int
	}{
		{id: "muse-spark-1.2-contributor-free", name: "Muse Spark 1.2 Contributor Free", targetFormat: "openai-responses", chat: true},
		{id: "muse-spark-1.3-contributor-free", name: "Muse Spark 1.3 Contributor Free", targetFormat: "openai-responses", chat: true, minOutput: 512},
		{id: "union-alpha", name: "Union Alpha Free", targetFormat: "claude", chat: true},
		{id: "jev-1.13-free", name: "Jev 1.13 Free", targetFormat: "", chat: false},
	}
	if len(provider.Models) != len(cases) {
		t.Fatalf("opencode declares %d models, want %d (%v)", len(provider.Models), len(cases), provider.Models)
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			model, found := index.Model("opencode", tc.id)
			if !found {
				t.Fatalf("model %q is not declared under opencode", tc.id)
			}
			if model.Name != tc.name {
				t.Fatalf("name = %q, want %q", model.Name, tc.name)
			}
			if model.TargetFormat != tc.targetFormat {
				t.Fatalf("target_format = %q, want %q", model.TargetFormat, tc.targetFormat)
			}
			if model.IsChat() != tc.chat {
				t.Fatalf("IsChat() = %v, want %v (kind %q)", model.IsChat(), tc.chat, model.Kind)
			}
			if model.MinOutputTokens != tc.minOutput {
				t.Fatalf("min_output_tokens = %d, want %d", model.MinOutputTokens, tc.minOutput)
			}
		})
	}

	if !provider.PassthroughModels {
		t.Fatal("opencode must stay passthrough: model ids beyond the declared list are still answerable")
	}
}

// TestEmbeddedMuseSparkFloorPinsEveryEntry asserts that a muse-spark id declared
// under more than one OpenCode entry carries the same output floor under both.
// The free tier and the zen entry route the same upstream model, so a floor
// declared on one and forgotten on the other would answer the same small ceiling
// with an empty body on only one of the two lanes.
func TestEmbeddedMuseSparkFloorPinsEveryEntry(t *testing.T) {
	index, err := Load()
	if err != nil {
		t.Fatalf("loading embedded registry: %v", err)
	}
	const modelID = "muse-spark-1.3-contributor-free"
	declared := 0
	for _, providerID := range []string{"opencode", "opencode-zen"} {
		model, found := index.Model(providerID, modelID)
		if !found {
			continue
		}
		declared++
		if model.MinOutputTokens != 512 {
			t.Fatalf("%s/%s min_output_tokens = %d, want 512", providerID, modelID, model.MinOutputTokens)
		}
	}
	if declared == 0 {
		t.Fatalf("%s is declared under neither OpenCode entry", modelID)
	}
}

// TestEmbeddedOpenCode_SystemOneEndpoint pins the endpoint the decision model is
// served from. The reference declares it as systemoneConfig on the entry
// (registry/opencode.js:34-40) rather than deriving it from the chat base, so a
// port that guessed the URL would be one path segment away from the wrong
// endpoint.
func TestEmbeddedOpenCode_SystemOneEndpoint(t *testing.T) {
	index, err := Load()
	if err != nil {
		t.Fatalf("loading embedded registry: %v", err)
	}
	provider, ok := index.Provider("opencode")
	if !ok {
		t.Fatal("opencode is missing from the embedded registry")
	}
	if provider.SystemOne == nil {
		t.Fatal("opencode declares no systemone endpoint")
	}
	if got, want := provider.SystemOne.BaseURL, "https://opencode.ai/zen/v1/systemone"; got != want {
		t.Fatalf("systemone base_url = %q, want %q", got, want)
	}
	// The client identity travels with the block in the reference, because the
	// decision endpoint reads the same headers the chat lane does.
	if got := provider.SystemOne.Headers["x-opencode-client"]; got != "desktop" {
		t.Fatalf("systemone x-opencode-client = %q, want desktop", got)
	}
	if got := provider.SystemOne.Headers["User-Agent"]; got == "" {
		t.Fatal("systemone declares no User-Agent")
	}
}
