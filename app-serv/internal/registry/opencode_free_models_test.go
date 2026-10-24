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
// the live free-tier feed (GET /inference/v1/models, measured 2026-10-15): id,
// display name, upstream wire format, chat eligibility. Eleven are declared and
// only ten are chat: `jev-1.13-free` is a System One decision model whose payload
// is the provider's own vocabulary, so the id stays truthful and the kind keeps
// it off the chat plane. Both are asserted together so an edit cannot quietly
// promote it back.
//
// `union-alpha` left on that measurement, answering 400 upstream; the Claude wire
// it pinned is held by `provider/opencode_test.go` instead.
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
		// within. Measured 2026-10-16 against the live upstream: under these
		// numbers the model spends the ceiling on thinking and answers an empty
		// body, or a tool call carrying no arguments.
		minOutput int
	}{
		{id: "muse-spark-1.2-contributor-free", name: "Muse Spark 1.2 Contributor Free", targetFormat: "openai-responses", chat: true},
		{id: "muse-spark-1.3-contributor-free", name: "Muse Spark 1.3 Contributor Free", targetFormat: "openai-responses", chat: true, minOutput: 2048},
		{id: "big-pickle", name: "Big Pickle Free", chat: true},
		{id: "cat-jam-free", name: "Cat Jam Free", chat: true},
		{id: "deepseek-v4.1-flash-free", name: "DeepSeek V4.1 Flash Free", chat: true, minOutput: 2048},
		{id: "longcat-2.5-preview-free", name: "LongCat 2.5 Preview Free", chat: true},
		{id: "mimo-v2.6-flash-free", name: "MiMo V2.6 Flash Free", chat: true},
		{id: "nemotron-3.5-lightning-free", name: "Nemotron 3.5 Lightning Free", chat: true},
		{id: "space-bunny-free", name: "Space Bunny Free", chat: true, minOutput: 1024},
		{id: "step-5-preview-free", name: "Step 5 Preview Free", chat: true},
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
		if model.MinOutputTokens != 2048 {
			t.Fatalf("%s/%s min_output_tokens = %d, want 2048", providerID, modelID, model.MinOutputTokens)
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
	if got, want := provider.SystemOne.BaseURL, "https://opencode.ai/inference/v1/systemone"; got != want {
		t.Fatalf("systemone base_url = %q, want %q", got, want)
	}
	// The client identity travels with the block in the reference, because the
	// decision endpoint reads the same headers the chat lane does. The reference
	// keeps `desktop` here even after the v2 move (PR #4840's registry hunk).
	if got := provider.SystemOne.Headers["x-opencode-client"]; got != "desktop" {
		t.Fatalf("systemone x-opencode-client = %q, want desktop", got)
	}
	if got := provider.SystemOne.Headers["User-Agent"]; got == "" {
		t.Fatal("systemone declares no User-Agent")
	}
}
