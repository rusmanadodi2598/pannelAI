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
// id, display name, the upstream wire format it speaks, and whether the chat
// plane may serve it at all.
//
// The reference declares four models here, and only three of them are chat: the
// fourth is a System One decision model (`kind: "systemone"`), whose payload is
// the provider's own vocabulary rather than a chat body. Carrying it in the
// entry is what makes the catalog tell the truth (the id exists), and the kind
// is what keeps the chat plane from answering it with the wrong body. The two
// facts are asserted together so a future edit cannot quietly promote the
// decision model back to chat.
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
	}{
		{id: "muse-spark-1.2-contributor-free", name: "Muse Spark 1.2 Contributor Free", targetFormat: "openai-responses", chat: true},
		{id: "muse-spark-1.3-contributor-free", name: "Muse Spark 1.3 Contributor Free", targetFormat: "openai-responses", chat: true},
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
		})
	}

	if !provider.PassthroughModels {
		t.Fatal("opencode must stay passthrough: model ids beyond the declared list are still answerable")
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
