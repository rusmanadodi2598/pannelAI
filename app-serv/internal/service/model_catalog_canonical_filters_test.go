// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_canonical_filters_test.go
// @for       The canonical-name rule applied to the three set surfaces: the
//
//	vision adapter's model list, the catalog's provider filter, the
//	alias target check, and the custom-list filter.
//
// @uses      internal/domain, context, testing.
// @reason    Draft 024 F2/F3 measured each of these refusing the alias and
//
//	node-prefix forms the router resolves, so each gets its own pin.
//	Separated from the combo tests at the AGENTS.md §1.1 line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestVisionAdapterService_ReplaceAcceptsEveryRouterForm pins F3: the adapter's
// model list is written through the same predicate, so the alias and prefix
// forms save there too. The capability predicate is stubbed permissive because
// the question under test is the name form, not the capability decision.
func TestVisionAdapterService_ReplaceAcceptsEveryRouterForm(t *testing.T) {
	ctx := context.Background()
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	catalog, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: canonicalNodeIndex(t), Repo: repo, Combos: combos,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	adapter, err := NewVisionAdapterService(VisionAdapterServiceDeps{
		Repo: newStubAdapterRepo(), Catalog: catalog, Capable: func(domain.ModelRef) bool { return true },
	})
	if err != nil {
		t.Fatalf("NewVisionAdapterService() error = %v", err)
	}

	for _, raw := range []string{"ks/glm-4.7", "corp/corp-chat", "openai-compatible-1/corp-chat"} {
		t.Run(raw, func(t *testing.T) {
			ref, err := domain.ParseModelRef(raw)
			if err != nil {
				t.Fatalf("ParseModelRef(%q) error = %v", raw, err)
			}
			if _, err := adapter.Replace(ctx, true, false, []domain.ModelRef{ref}); err != nil {
				t.Fatalf("Replace() with %q error = %v", raw, err)
			}
		})
	}
}

// TestModelCatalogService_CatalogFiltersByEveryRouterForm pins the filter half
// of F2: `provider_id` accepts the same three forms, so an operator filtering
// by the alias sees the rows the id form shows.
func TestModelCatalogService_CatalogFiltersByEveryRouterForm(t *testing.T) {
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
		name        string
		providerID  string
		wantContain string
		wantRows    int
	}{
		{name: "the id form filters", providerID: "kserve", wantContain: "kserve/glm-4.7", wantRows: 1},
		{name: "the alias form filters the same rows", providerID: "ks", wantContain: "kserve/glm-4.7", wantRows: 1},
		{name: "the node id form filters", providerID: "openai-compatible-1", wantContain: "openai-compatible-1/corp-chat", wantRows: 1},
		{name: "the node prefix form filters the same rows", providerID: "corp", wantContain: "openai-compatible-1/corp-chat", wantRows: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := service.Catalog(ctx, CatalogFilter{ProviderID: tc.providerID})
			if err != nil {
				t.Fatalf("Catalog() error = %v", err)
			}
			if len(rows) != tc.wantRows {
				t.Fatalf("Catalog(provider_id=%q) = %d rows, want %d: %v", tc.providerID, len(rows), tc.wantRows, catalogKeys(rows))
			}
			if len(rows) > 0 && rows[0].Ref().String() != tc.wantContain {
				t.Fatalf("Catalog(provider_id=%q) row = %q, want %q", tc.providerID, rows[0].Ref().String(), tc.wantContain)
			}
		})
	}
}

// TestModelCatalogService_ReplaceAliasesAcceptsEveryRouterForm pins the alias
// target: a target spelled with a prefix or alias resolves the same way, so the
// reference's own examples (`oc/...` targets) round-trip.
func TestModelCatalogService_ReplaceAliasesAcceptsEveryRouterForm(t *testing.T) {
	ctx := context.Background()
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: canonicalNodeIndex(t), Repo: repo, Combos: combos,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	if err := service.ReplaceAliases(ctx, []domain.ModelAlias{
		mustAlias(t, "fast", "ks/glm-4.7"),
		mustAlias(t, "corp-fast", "corp/corp-chat"),
	}); err != nil {
		t.Fatalf("ReplaceAliases() error = %v", err)
	}
}

// TestModelCatalogService_CustomFilterMatchesBothSpellings pins the two-way
// match: a row stored under a node prefix is found by the node's id and vice
// versa, because the write path has accepted either spelling since custom nodes
// existed. A one-way match would hide half the rows from one of the two forms.
func TestModelCatalogService_CustomFilterMatchesBothSpellings(t *testing.T) {
	ctx := context.Background()
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: canonicalNodeIndex(t), Repo: newStubCatalogRepo(), Combos: newStubComboRepo(),
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	// One row under the node's id and one under its prefix, both accepted
	// writes today.
	for providerID, modelID := range map[string]string{
		"openai-compatible-1": "by-id",
		"corp":                "by-prefix",
	} {
		if _, err := service.AddCustom(ctx, providerID, modelID, modelID, nil); err != nil {
			t.Fatalf("AddCustom(%q) error = %v", providerID, err)
		}
	}

	cases := []struct {
		name       string
		providerID string
		wantIDs    []string
	}{
		{name: "the node id finds both rows", providerID: "openai-compatible-1", wantIDs: []string{"by-id", "by-prefix"}},
		{name: "the node prefix finds both rows", providerID: "corp", wantIDs: []string{"by-id", "by-prefix"}},
		{name: "an unrelated provider finds neither", providerID: "kserve", wantIDs: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			models, err := service.Custom(ctx, tc.providerID)
			if err != nil {
				t.Fatalf("Custom(%q) error = %v", tc.providerID, err)
			}
			got := make([]string, 0, len(models))
			for _, model := range models {
				got = append(got, model.ModelID())
			}
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("Custom(%q) = %v, want %v", tc.providerID, got, tc.wantIDs)
			}
			for _, want := range tc.wantIDs {
				found := false
				for _, id := range got {
					if id == want {
						found = true
					}
				}
				if !found {
					t.Fatalf("Custom(%q) = %v, want it to contain %q", tc.providerID, got, want)
				}
			}
		})
	}
}
