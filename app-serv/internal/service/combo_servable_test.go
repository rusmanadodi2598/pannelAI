// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_servable_test.go
// @for       Draft 024 F4 after review: the write-path servability rule must ask
//
//	the router's own question, in both directions.
//
// @uses      internal/domain, internal/registry, context, strings, testing.
// @reason    The first cut judged servability from the catalog row, which is
//
//	neither what the router does nor complete: it refused a passthrough
//	provider's undeclared id (the router serves it) and accepted a
//	custom row on a non-passthrough provider (the router refuses it,
//	because custom rows are not in the resolver's index). It also read
//	the row's provider rather than the provider the reference resolves
//	to, so an id shadowed by another provider's alias was judged against
//	the wrong entry. These tests pin the router-shaped answer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// servableIndex builds the provider shapes the rule must distinguish, including
// a passthrough provider and a shadowed id (one provider's alias equals another
// provider's id, the shape the embedded registry carries for `mmf`).
func servableIndex(t *testing.T) *registry.Index {
	t.Helper()
	index, err := registry.NewIndex(registry.Document{Revision: "test", Providers: []registry.Provider{
		{
			ID: "declared-only", Category: "apikey", Transport: registry.Transport{Format: registry.DefaultFormat},
			Models: []registry.Model{testModel("known", "Known", "llm")},
		},
		{
			ID: "passthrough", Category: "apikey", PassthroughModels: true,
			Transport: registry.Transport{Format: registry.DefaultFormat},
		},
		{
			ID: "media-provider", Category: "apikey", Transport: registry.Transport{Format: registry.DefaultFormat},
			Models: []registry.Model{testModel("image-one", "Image", "image")},
		},
		{
			// Alias `shadowed` wins the name lookup, so `shadowed/x` routes to
			// `alias-owner`, not to this entry — the router's own rule.
			ID: "alias-owner", Category: "apikey", Alias: "shadowed", PassthroughModels: true,
			Transport: registry.Transport{Format: registry.DefaultFormat},
		},
		{
			ID: "shadowed", Category: "apikey", Transport: registry.Transport{Format: registry.DefaultFormat},
			Models: []registry.Model{testModel("hidden", "Hidden", "llm")},
		},
	}})
	if err != nil {
		t.Fatalf("registry.NewIndex() error = %v", err)
	}
	return index
}

// servableServices wires the combo and catalog services over servableIndex.
func servableServices(t *testing.T) (*ComboService, *ModelCatalogService) {
	t.Helper()
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	catalog, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: servableIndex(t), Repo: repo, Combos: combos,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	service, err := NewComboService(ComboServiceDeps{Repo: combos, Catalog: catalog, Rotation: nil})
	if err != nil {
		t.Fatalf("NewComboService() error = %v", err)
	}
	return service, catalog
}

// TestComboService_CreateJudgesServabilityLikeTheRouter covers both directions
// of the rule, the alias hop, the shadowed id, and the self-reference refusal.
func TestComboService_CreateJudgesServabilityLikeTheRouter(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		ref     string
		wantMsg string // empty means the create must succeed
	}{
		{name: "a declared chat model saves", ref: "declared-only/known"},
		{
			name: "a passthrough provider's undeclared id saves, because the router serves it",
			ref:  "passthrough/anything-at-all",
		},
		{
			name: "a declared model on a shadowed id is judged by the provider the reference resolves to",
			ref:  "shadowed/hidden",
			// The router resolves `shadowed` to `alias-owner` (an alias beats
			// another provider's id), which is a passthrough provider: the id
			// `hidden` is served by alias-owner, not by the shadowed entry. So
			// this saves — and that is the router's own answer.
		},
		{
			name:    "a non-passthrough provider's undeclared id is refused with the reason",
			ref:     "declared-only/ghost",
			wantMsg: "does not pass model ids through",
		},
		{
			name:    "a media model is refused with its kind",
			ref:     "media-provider/image-one",
			wantMsg: "is a media model",
		},
	}
	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := servableServices(t)
			draft := ComboDraft{
				Name:     "servable-" + string(rune('a'+idx)),
				Strategy: domain.ComboFallback,
				Models:   []domain.ComboModel{comboRef(t, tc.ref, 1)},
			}
			_, err := service.Create(ctx, draft)
			if tc.wantMsg == "" {
				if err != nil {
					t.Fatalf("Create(%q) error = %v, want it accepted", tc.ref, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Create(%q) accepted a reference the router cannot serve", tc.ref)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("Create(%q) error = %v, want it to name %q", tc.ref, err, tc.wantMsg)
			}
		})
	}
}
