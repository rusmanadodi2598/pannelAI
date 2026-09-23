// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/catalog_filter_index_test.go
// @for       The work one filtered catalog read performs: the provider name
//
//	table is built from one index snapshot, never per row.
//
// @uses      internal/registry, context, testing.
// @reason    In production the catalog index is the composition root's adapter,
//
//	which rebuilds the node overlay — a node-list query plus a full
//	re-index — on every Provider() call. A per-row lookup would turn one
//	panel request into one query per catalog row, so the read count is
//	part of the behaviour and is pinned here. Separated from the
//	servability table at the AGENTS.md §1.1 line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestModelCatalogService_CatalogFilterReadsTheIndexOnce pins the performance
// half of the provider filter: a filtered catalog request must not rebuild the
// runtime index per row, because every Provider() call on the composition root's
// adapter is a node-list query plus a full re-index.
func TestModelCatalogService_CatalogFilterReadsTheIndexOnce(t *testing.T) {
	ctx := context.Background()
	index := servableIndex(t)
	counting := &countingIndex{inner: index}
	catalog, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: counting, Repo: newStubCatalogRepo(), Combos: newStubComboRepo(),
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	rows, err := catalog.Catalog(ctx, CatalogFilter{ProviderID: "declared-only"})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Catalog() = %d rows, want 1", len(rows))
	}
	// One All() to build the name index, plus the per-row work the filter does
	// not need. A budget of 2 leaves room for one incidental lookup while
	// failing a per-row rebuild (the fixture has five providers and two models).
	if counting.providerCalls > 2 {
		t.Fatalf("Catalog() called Provider() %d times, want at most 2", counting.providerCalls)
	}
}

// countingIndex counts Provider() calls so a test can pin that a filtered read
// does not rebuild the index per row.
type countingIndex struct {
	inner         CatalogIndex
	providerCalls int
}

func (c *countingIndex) Provider(name string) (registry.Provider, bool) {
	c.providerCalls++
	return c.inner.Provider(name)
}

func (c *countingIndex) All() []registry.Provider { return c.inner.All() }
