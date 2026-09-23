// Command app-serv adapts a provider node's model list to HTTP.
//
// @file      cmd/app-serv/node_models_surfaces_test.go
// @for       Draft 017 §4.2's acceptance: a node's upstream list reaches all
//
//	three surfaces that read models from the registry.
//
// @uses      internal/dataplane, internal/domain, internal/netguard,
//
//	internal/provider, internal/registry, internal/service, context,
//	net/http, net/http/httptest, testing, time.
//
// @reason    The finding was not that one route answered an empty list; it was
//
//	that four surfaces agreed on the empty answer because they all read
//	`Provider.Models`. The fix injects at that one point, so the proof has
//	to be at the surfaces: the detail route, the catalog, and the data
//	plane must each show the upstream's models. Asserting only the
//	adapter's own answer would pass with the injection removed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TestNodeModels_ReachEverySurfaceThatReadsProviderModels is the closure test.
//
// It builds the real overlay over a real stored node whose upstream answers two
// models, then asks the three surfaces draft 017 §4.2 lists as empty.
func TestNodeModels_ReachEverySurfaceThatReadsProviderModels(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"upstream-one"},{"id":"upstream-two"}]}`))
	}))
	defer upstream.Close()

	node, err := domain.NewProviderNode("", "Corp gateway", "corp", domain.NodeOpenAICompatible, domain.NodeAPIChat, upstream.URL, time.Now())
	if err != nil {
		t.Fatalf("NewProviderNode() error = %v", err)
	}

	guard, err := netguard.NewGuard([]string{"127.0.0.1/32", "::1/128"})
	if err != nil {
		t.Fatalf("netguard.NewGuard() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("provider.NewConnectors() error = %v", err)
	}
	source := newNodeModelSource(
		staticNodeLookup{id: node.ID(), baseURL: upstream.URL, format: node.Format(), apiType: node.APIType()}.lookup,
		connectors, guard, staticCredential("sk-live-abcdef"), time.Minute,
	)

	embedded, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	index := newRuntimeProviderIndex(embedded, nodeListerStub{[]domain.ProviderNode{node}}, source, nil)

	// 1. The registry entry the detail route reads.
	entry, ok := index.Provider(node.ID())
	if !ok {
		t.Fatalf("the node %q does not resolve through the overlay", node.ID())
	}
	if len(entry.Models) != 2 {
		t.Fatalf("the overlay synthesized %d models, want 2 from the upstream", len(entry.Models))
	}

	// 2. The provider read the panel's model list uses.
	providers, err := service.NewProviderService(service.ProviderServiceDeps{Index: index, Source: source})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	list, err := providers.Models(context.Background(), node.ID())
	if err != nil {
		t.Fatalf("ProviderService.Models() error = %v", err)
	}
	if list.Source != service.ModelSourceUpstream {
		t.Fatalf("Models().Source = %q, want %q", list.Source, service.ModelSourceUpstream)
	}
	if len(list.Entry.Models) != 2 {
		t.Fatalf("Models() answered %d models, want 2", len(list.Entry.Models))
	}

	// 3. The catalog, which is what `?capability=` and the panel's search read.
	catalog, err := service.NewModelCatalogService(service.ModelCatalogServiceDeps{
		Index: index, Repo: surfacesCatalogRepo{}, Combos: surfacesComboRepo{},
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	rows, err := catalog.Catalog(context.Background(), service.CatalogFilter{ProviderID: node.ID()})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("the catalog answered %d rows for the node, want 2", len(rows))
	}

	// 4. The data plane's published list, which is what a client can call.
	resolver, err := dataplane.NewResolver(index, surfacesModelLookup{})
	if err != nil {
		t.Fatalf("dataplane.NewResolver() error = %v", err)
	}
	models, err := resolver.ModelList(context.Background())
	if err != nil {
		t.Fatalf("ModelList() error = %v", err)
	}
	found := 0
	for _, model := range models.Data {
		if strings.HasPrefix(model.ID, node.ID()+"/") {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("the data plane published %d models for the node, want 2", found)
	}
}

// surfacesCatalogRepo is an empty catalog store: this test is about the registry
// half of the merge, so the operator's own rows contribute nothing.
type surfacesCatalogRepo struct{}

func (surfacesCatalogRepo) Custom(context.Context) ([]domain.CustomModel, error) { return nil, nil }
func (surfacesCatalogRepo) AddCustom(context.Context, domain.CustomModel) error  { return nil }
func (surfacesCatalogRepo) RemoveCustom(context.Context, string) error           { return nil }
func (surfacesCatalogRepo) Aliases(context.Context) ([]domain.ModelAlias, error) { return nil, nil }
func (surfacesCatalogRepo) ReplaceAliases(context.Context, []domain.ModelAlias) error {
	return nil
}
func (surfacesCatalogRepo) Disabled(context.Context) ([]domain.ModelRef, error) { return nil, nil }
func (surfacesCatalogRepo) ReplaceDisabled(context.Context, []domain.ModelRef) error {
	return nil
}

// surfacesComboRepo is an empty combo store, for the same reason: the data-plane
// list's combo half contributes nothing to this assertion.
type surfacesComboRepo struct{}

func (surfacesComboRepo) Create(context.Context, domain.Combo) error { return nil }
func (surfacesComboRepo) List(context.Context, repository.PageQuery) ([]domain.Combo, int64, error) {
	return nil, 0, nil
}
func (surfacesComboRepo) GetByID(context.Context, string) (domain.Combo, error) {
	return domain.Combo{}, domain.ErrComboNotFound
}
func (surfacesComboRepo) ExistsByName(context.Context, string) (bool, error) { return false, nil }
func (surfacesComboRepo) Names(context.Context) ([]string, error)            { return nil, nil }
func (surfacesComboRepo) GetByName(context.Context, string) (domain.Combo, error) {
	return domain.Combo{}, domain.ErrComboNotFound
}
func (surfacesComboRepo) Update(context.Context, domain.Combo) error { return nil }
func (surfacesComboRepo) Delete(context.Context, string) error       { return nil }

// surfacesModelLookup is an empty combo/alias/disabled lookup, so the data-plane
// list is exactly the registry's routable models.
type surfacesModelLookup struct{}

func (surfacesModelLookup) Combo(context.Context, string) (domain.Combo, bool, error) {
	return domain.Combo{}, false, nil
}
func (surfacesModelLookup) Alias(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (surfacesModelLookup) Disabled(context.Context, string, string) (bool, error) {
	return false, nil
}
func (surfacesModelLookup) DisabledPairs(context.Context) ([]domain.ModelRef, error) { return nil, nil }
func (surfacesModelLookup) ComboNames(context.Context) ([]string, error)             { return nil, nil }
