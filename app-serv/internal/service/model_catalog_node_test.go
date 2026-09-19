// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_node_test.go
// @for       The catalog and combo rules that apply to a custom provider node.
// @uses      internal/domain, internal/registry, context, testing.
// @reason    A node is routable everywhere the runtime overlay is handed over,
//
//	so the catalog and the combo reference check must read that same
//	view: a model registered under a node has to appear in the
//	catalog and be usable as a combo ref. These tests pin both, the
//	pair that answered "unknown provider_id" and "does not resolve"
//	when the catalog held the boot-time registry instead.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// nodeIndex builds a small registry with one custom node synthesized into it,
// which is what the composition root's runtime overlay produces at boot.
func nodeIndex(t *testing.T, node registry.CustomNode) CatalogIndex {
	t.Helper()
	index, err := testIndex(t, testProvider("openai", "api", testModel("gpt-4o", "GPT-4o", "llm"))).WithCustom(node)
	if err != nil {
		t.Fatalf("WithCustom() error = %v", err)
	}
	return index
}

// testNode is a minimal OpenAI-compatible node.
func testNode() registry.CustomNode {
	return registry.CustomNode{
		ID:      registry.OpenAICompatiblePrefix + "node1",
		Name:    "Node One",
		Prefix:  "stubnode",
		APIType: "chat",
		BaseURL: "http://127.0.0.1:9/v1",
	}
}

// TestModelCatalogService_RegistersAModelUnderACustomNode pins the write the
// node story needs: a model may be registered under a provider that exists only
// as a synthesized node, and the catalog then offers it.
func TestModelCatalogService_RegistersAModelUnderACustomNode(t *testing.T) {
	ctx := context.Background()
	node := testNode()
	catalog := newCatalogService(t, nodeIndex(t, node), newStubCatalogRepo(), newStubComboRepo())

	model, err := catalog.AddCustom(ctx, node.ID, "stub-model", "Stub Model", domain.NewModelCapabilities("chat"))
	if err != nil {
		t.Fatalf("AddCustom() under a custom node error = %v", err)
	}
	if model.ProviderID() != node.ID {
		t.Fatalf("AddCustom() provider = %q, want %q", model.ProviderID(), node.ID)
	}

	models, err := catalog.Catalog(ctx, CatalogFilter{ProviderID: node.ID})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if got := catalogKeys(models); len(got) != 1 || got[0] != node.ID+"/stub-model" {
		t.Fatalf("Catalog() = %v, want the node's custom model", got)
	}
	if models[0].Source() != domain.CatalogSourceCustom {
		t.Fatalf("Catalog() source = %q, want %q", models[0].Source(), domain.CatalogSourceCustom)
	}
}

// TestComboService_AcceptsARefUnderACustomNode pins the combo half: the same
// merged view backs the reference check, so a node's model is a legal ref.
func TestComboService_AcceptsARefUnderACustomNode(t *testing.T) {
	ctx := context.Background()
	node := testNode()
	catalog := newCatalogService(t, nodeIndex(t, node), newStubCatalogRepo(), newStubComboRepo())
	if _, err := catalog.AddCustom(ctx, node.ID, "stub-model", "Stub Model", domain.NewModelCapabilities("chat")); err != nil {
		t.Fatalf("AddCustom() error = %v", err)
	}

	combos, err := NewComboService(ComboServiceDeps{Repo: newStubComboRepo(), Catalog: catalog})
	if err != nil {
		t.Fatalf("NewComboService() error = %v", err)
	}
	combo, err := combos.Create(ctx, comboDraft(t, "node-combo", domain.ComboFallback, 0, "",
		comboRef(t, node.ID+"/stub-model", 1)))
	if err != nil {
		t.Fatalf("Create() with a node ref error = %v", err)
	}
	if combo.ModelCount() != 1 {
		t.Fatalf("Create() model count = %d, want 1", combo.ModelCount())
	}
}
