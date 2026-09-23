// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/custom_node_models_test.go
// @for       The node's model list: a synthesized entry carries the list the
//
//	node was handed, so every index consumer sees it.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.4 serves a custom node's models, and draft 017 §4.2
//
//	measured what a synthesized entry carried without this: `len(entry.Models)
//	= 0`, so the node appeared in four surfaces with no model at all. The
//	list is injected rather than declared because a compatible node's models
//	come from its own upstream; the registry is where it lands because every
//	consumer (the detail route, the catalog, the data plane) already reads
//	models from there.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package registry

import "testing"

// TestSynthesize_CarriesTheNodesModelList covers the count shapes a node can
// have: none (an operator who declared nothing), one, and many.
func TestSynthesize_CarriesTheNodesModelList(t *testing.T) {
	cases := []struct {
		name   string
		models []Model
	}{
		{name: "a node with no model carries none", models: nil},
		{name: "one model is carried", models: []Model{{ID: "gpt-4o-mini"}}},
		{name: "many models are carried in order", models: []Model{{ID: "a"}, {ID: "b"}, {ID: "c"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			node := CustomNode{
				ID:      "openai-compatible-01TEST",
				Name:    "Corp gateway",
				Prefix:  "corp",
				APIType: "chat",
				BaseURL: "https://llm.corp.test/v1",
				Models:  tc.models,
			}
			overlaid, err := index.WithCustom(node)
			if err != nil {
				t.Fatalf("WithCustom() error = %v", err)
			}
			entry, ok := overlaid.Provider(node.ID)
			if !ok {
				t.Fatalf("the synthesized node %q does not resolve", node.ID)
			}
			if len(entry.Models) != len(tc.models) {
				t.Fatalf("len(entry.Models) = %d, want %d", len(entry.Models), len(tc.models))
			}
			for i, want := range tc.models {
				if entry.Models[i].ID != want.ID {
					t.Fatalf("entry.Models[%d].ID = %q, want %q", i, entry.Models[i].ID, want.ID)
				}
			}
		})
	}
}

// TestSynthesize_CopiesTheModelList pins the ownership rule the index follows
// everywhere else: a caller cannot mutate the index by writing through the
// slice it handed over, and reading the entry cannot mutate the caller's.
func TestSynthesize_CopiesTheModelList(t *testing.T) {
	index, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	declared := []Model{{ID: "first"}, {ID: "second"}}
	node := CustomNode{
		ID:      "openai-compatible-01COPY",
		Name:    "Corp gateway",
		Prefix:  "corp",
		APIType: "chat",
		BaseURL: "https://llm.corp.test/v1",
		Models:  declared,
	}
	overlaid, err := index.WithCustom(node)
	if err != nil {
		t.Fatalf("WithCustom() error = %v", err)
	}
	declared[0].ID = "mutated-after-synthesis"
	entry, ok := overlaid.Provider(node.ID)
	if !ok {
		t.Fatalf("the synthesized node %q does not resolve", node.ID)
	}
	if entry.Models[0].ID != "first" {
		t.Fatalf("entry.Models[0].ID = %q after the caller mutated its slice; the index kept a reference instead of a copy", entry.Models[0].ID)
	}
}
