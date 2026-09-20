// Package registry loads the provider registry and answers lookups over it.
//
// @file      internal/registry/opencode_free_models_test.go
// @for       The OpenCode Free entry's declared models, pinned to the reference.
// @uses      testing.
// @reason    The port dropped the reference's static model list for opencode,
//
//	so its free models were answerable through passthrough yet absent
//	from every models list, and resolved onto the provider's OpenAI
//	format instead of the per-model formats the reference declares
//	(9router PR #4073). Pinning the entry here keeps the catalog and
//	the reference from drifting apart again.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-20
package registry

import "testing"

// TestEmbeddedOpenCode_FreeModels pins the opencode entry's declared models to
// the reference registry (open-sse/providers/registry/opencode.js): each model's
// id, display name, and the upstream wire format it speaks, because the
// provider's own format is not the one every model answers in.
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
	}{
		{id: "muse-spark-1.2-contributor-free", name: "Muse Spark 1.2 Contributor Free", targetFormat: "openai-responses"},
		{id: "muse-spark-1.3-contributor-free", name: "Muse Spark 1.3 Contributor Free", targetFormat: "openai-responses"},
		{id: "union-alpha", name: "Union Alpha Free", targetFormat: "claude"},
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
			if !model.IsChat() {
				t.Fatalf("model %q must be a chat model", tc.id)
			}
		})
	}

	if !provider.PassthroughModels {
		t.Fatal("opencode must stay passthrough: model ids beyond the declared list are still answerable")
	}
}
