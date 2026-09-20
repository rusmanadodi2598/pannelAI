// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/resolve_opencode_test.go
// @for       The OpenCode Free models' listing and wire-format resolution.
// @uses      context, testing, internal/registry.
// @reason    OpenCode Free is a no-auth passthrough provider, so nothing gates
//
//	its model strings: they resolve as-is. That is exactly why the
//	declared list matters (9router PR #4073): a client cannot ask for
//	oc/muse-spark-1.2-contributor-free if no models list ever mentions
//	it, and the gateway must translate it to the Responses wire, not
//	the provider's chat-completions one. Both properties are pinned
//	against the embedded registry, where the bug lived.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package dataplane

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// opencodeResolver builds a resolver over the embedded registry, so the pin
// runs against the same document the router serves, not a synthetic index.
func opencodeResolver(t *testing.T) *Resolver {
	t.Helper()
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("loading embedded registry: %v", err)
	}
	resolver, err := NewResolver(index, fakeLookup{})
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}
	return resolver
}

// TestResolver_OpenCodeFree_DeclaredModelsResolveToTheirOwnWire pins the
// per-model target formats: a declared model resolves as declared, never as
// passthrough, so the translation layer sees the format the model answers in.
func TestResolver_OpenCodeFree_DeclaredModelsResolveToTheirOwnWire(t *testing.T) {
	resolver := opencodeResolver(t)

	cases := []struct {
		modelID string
		target  string
	}{
		{modelID: "muse-spark-1.2-contributor-free", target: TargetResponses},
		{modelID: "muse-spark-1.3-contributor-free", target: TargetResponses},
		{modelID: "union-alpha", target: TargetClaude},
	}
	for _, tc := range cases {
		t.Run(tc.modelID, func(t *testing.T) {
			resolution, err := resolver.ResolveParts(context.Background(), "opencode", tc.modelID)
			if err != nil {
				t.Fatalf("ResolveParts: %v", err)
			}
			if resolution.Model.ID != tc.modelID {
				t.Fatalf("resolved model = %q, want the declared %q", resolution.Model.ID, tc.modelID)
			}
			if resolution.Target != tc.target {
				t.Fatalf("target = %q, want %q", resolution.Target, tc.target)
			}
		})
	}
}

// TestResolver_OpenCodeFree_UnknownModelStillPassesThrough pins the other half
// of the contract: restoring the declared list must not turn opencode into a
// closed catalog, because ids beyond it remain answerable as-is.
func TestResolver_OpenCodeFree_UnknownModelStillPassesThrough(t *testing.T) {
	resolver := opencodeResolver(t)

	resolution, err := resolver.ResolveParts(context.Background(), "opencode", "not-in-the-registry")
	if err != nil {
		t.Fatalf("ResolveParts: %v", err)
	}
	if resolution.Model.ID != "not-in-the-registry" {
		t.Fatalf("model = %q, want the passthrough id as-is", resolution.Model.ID)
	}
	if resolution.Target != TargetOpenAI {
		t.Fatalf("target = %q, want %q", resolution.Target, TargetOpenAI)
	}
}

// TestResolver_OpenCodeFree_ModelsAreListed pins the user-visible symptom the
// reference fixed: the free models are discoverable in the models list a client
// reads, alongside every other routable model.
func TestResolver_OpenCodeFree_ModelsAreListed(t *testing.T) {
	resolver := opencodeResolver(t)

	list, err := resolver.ModelList(context.Background())
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}

	cases := []string{
		"opencode/muse-spark-1.2-contributor-free",
		"opencode/muse-spark-1.3-contributor-free",
		"opencode/union-alpha",
	}
	listed := make(map[string]schema.ModelObject, len(list.Data))
	for _, model := range list.Data {
		listed[model.ID] = model
	}
	for _, want := range cases {
		t.Run(want, func(t *testing.T) {
			model, ok := listed[want]
			if !ok {
				t.Fatalf("%q is missing from the models list", want)
			}
			if model.OwnedBy != "opencode" {
				t.Fatalf("owned_by = %q, want opencode", model.OwnedBy)
			}
		})
	}
}
