// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_canonical_test.go
// @for       The canonical-name rule every model-ref write path shares: a ref
//
//	is valid in any form the router resolves (id, registry alias, node
//	prefix), not only the id form the catalog rows carry.
//
// @uses      internal/domain, internal/registry, context, strings, testing.
// @reason    Draft 024 F2 measured the drift: the router resolves three forms
//
//	of the first segment while ModelExists reads one, so a combo member
//	spelled with the reference's own alias (`cc/claude-...`) or a node
//	prefix was refused at write time by a list the same gateway routes.
//	These tests pin the agreement: whatever the router accepts as a
//	namespace, the write path accepts as a reference, and the catalog
//	filter answers by the same canonical id.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// canonicalNodeIndex is the fixture's index overlaid with one custom node, the
// shape the composition root hands the service in production. The node carries
// an upstream model list, as draft 017's fix injects.
func canonicalNodeIndex(t *testing.T) *registry.Index {
	t.Helper()
	base := testIndex(t, registry.Provider{
		ID: "kserve", Priority: 1, Alias: "ks", Category: "api",
		Transport: registry.Transport{Format: "openai"},
		Models:    []registry.Model{testModel("glm-4.7", "GLM 4.7", "llm")},
	})
	node := registry.CustomNode{
		ID: "openai-compatible-1", Name: "Corp gateway", Prefix: "corp",
		APIType: "chat", BaseURL: "https://corp.example.com/v1",
		Models: []registry.Model{testModel("corp-chat", "Corp chat", "llm")},
	}
	overlaid, err := base.WithCustom(node)
	if err != nil {
		t.Fatalf("WithCustom() error = %v", err)
	}
	return overlaid
}

// TestModelCatalogService_ModelExistsAcceptsEveryRouterForm is the F2 core: a
// reference names a model through the provider's id, its registry alias, or a
// node prefix, and the write-path predicate must answer yes for all of them,
// because the router resolves all three.
func TestModelCatalogService_ModelExistsAcceptsEveryRouterForm(t *testing.T) {
	ctx := context.Background()
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: canonicalNodeIndex(t), Repo: repo, Combos: combos,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}

	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "the registry id form", raw: "kserve/glm-4.7", want: true},
		{name: "the registry alias form", raw: "ks/glm-4.7", want: true},
		{name: "the node id form", raw: "openai-compatible-1/corp-chat", want: true},
		{name: "the node prefix form", raw: "corp/corp-chat", want: true},
		{name: "an alias with a model the provider does not declare", raw: "ks/ghost", want: false},
		{name: "a prefix with a model the node does not declare", raw: "corp/ghost", want: false},
		{name: "an unknown namespace", raw: "nowhere/glm-4.7", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := domain.ParseModelRef(tc.raw)
			if err != nil {
				t.Fatalf("ParseModelRef(%q) error = %v", tc.raw, err)
			}
			got, err := service.ModelExists(ctx, ref)
			if err != nil {
				t.Fatalf("ModelExists(%q) error = %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("ModelExists(%q) = %t, want %t", tc.raw, got, tc.want)
			}
		})
	}
}

// TestComboService_CreateAcceptsEveryRouterForm pins F2 at the route the owner
// named: a combo member spelled with a registry alias or a node prefix saves.
// The stored ref is what the operator typed — the router resolves it — so the
// row must round-trip unchanged.
func TestComboService_CreateAcceptsEveryRouterForm(t *testing.T) {
	ctx := context.Background()
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	catalog, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: canonicalNodeIndex(t), Repo: repo, Combos: combos,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	service, err := NewComboService(ComboServiceDeps{Repo: combos, Catalog: catalog, Rotation: nil})
	if err != nil {
		t.Fatalf("NewComboService() error = %v", err)
	}

	cases := []struct {
		name string
		ref  string
	}{
		{name: "a registry alias form member", ref: "ks/glm-4.7"},
		{name: "a node id form member", ref: "openai-compatible-1/corp-chat"},
		{name: "a node prefix form member", ref: "corp/corp-chat"},
		{name: "an alias form judge model", ref: "ks/glm-4.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			draft := ComboDraft{
				Name: "probe-" + strings.ReplaceAll(strings.ReplaceAll(tc.ref, "/", "-"), ".", "-"),
				Models: []domain.ComboModel{
					comboRef(t, "kserve/glm-4.7", 1),
				},
			}
			if strings.Contains(tc.name, "judge") {
				draft.Name = "probe-judge"
				draft.Strategy = domain.ComboFusion
				draft.JudgeModel = tc.ref
			} else {
				draft.Strategy = domain.ComboFallback
				draft.Models = []domain.ComboModel{comboRef(t, tc.ref, 1)}
			}
			combo, err := service.Create(ctx, draft)
			if err != nil {
				t.Fatalf("Create() with member %q error = %v", tc.ref, err)
			}
			if strings.Contains(tc.name, "judge") {
				if combo.JudgeModel() != tc.ref {
					t.Fatalf("judge stored as %q, want the typed %q", combo.JudgeModel(), tc.ref)
				}
				return
			}
			if len(combo.Models()) != 1 || combo.Models()[0].Ref() != tc.ref {
				t.Fatalf("member stored as %v, want the typed %q", combo.Refs(), tc.ref)
			}
		})
	}
}
