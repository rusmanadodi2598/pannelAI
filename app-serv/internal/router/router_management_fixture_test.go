// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_management_fixture_test.go
// @for       The wired management handler set and its registry fixture.
// @uses      internal/domain, internal/handler, internal/registry,
//
//	internal/service, context, testing, time.
//
// @reason    The route tables and the fixture that feeds them are separate
//
//	declarations, and AGENTS.md §1.1 caps a file at 250 lines. Keeping
//	the fixture here means a new management vertical adds one field and
//	one constructor line, while each route table stays the audit it is.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// catalogServices is the wired management handler set a route test drives.
type catalogServices struct {
	model     *handler.ModelHandler
	combo     *handler.ComboHandler
	comboTest *handler.ComboTestHandler
	vision    *handler.VisionAdapterHandler
	proxy     *handler.ProxyHandler
	media     *handler.MediaProviderHandler
}

// newCatalogFixture wires the catalog, combo, adapter, proxy, and media
// services over in-memory repositories.
func newCatalogFixture(t *testing.T) catalogServices {
	t.Helper()
	index, err := registry.NewIndex(registry.Document{Revision: "test", Providers: []registry.Provider{
		{
			ID: "openai", Priority: 1, Category: "api",
			Models: []registry.Model{
				{ID: "gpt-4o", Name: "GPT-4o", Kind: "llm", Capabilities: []string{"vision", "tools"}},
				{ID: "gpt-4o-mini", Name: "GPT-4o mini", Kind: "llm", Capabilities: []string{"tools"}},
			},
		},
		{
			ID: "anthropic", Priority: 2, Category: "api",
			Models: []registry.Model{{ID: "claude-3", Name: "Claude 3", Kind: "llm", Capabilities: []string{"vision"}}},
		},
	}})
	if err != nil {
		t.Fatalf("registry.NewIndex() error = %v", err)
	}
	catalogRepo := newMemCatalogRepo()
	comboRepo := newMemComboRepo()
	catalog, err := service.NewModelCatalogService(service.ModelCatalogServiceDeps{
		Index: index, Repo: catalogRepo, Combos: comboRepo,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	combos, err := service.NewComboService(service.ComboServiceDeps{
		Repo: comboRepo, Catalog: catalog, Rotation: nil,
	})
	if err != nil {
		t.Fatalf("NewComboService() error = %v", err)
	}
	adapter, err := service.NewVisionAdapterService(service.VisionAdapterServiceDeps{
		Repo: newMemAdapterRepo(), Catalog: catalog,
	})
	if err != nil {
		t.Fatalf("NewVisionAdapterService() error = %v", err)
	}
	comboTests, err := service.NewComboTestService(combos, routeProber{})
	if err != nil {
		t.Fatalf("NewComboTestService() error = %v", err)
	}
	seedRouteFixture(t, comboRepo)
	return catalogServices{
		model:     handler.NewModelHandler(catalog),
		combo:     handler.NewComboHandler(combos),
		comboTest: handler.NewComboTestHandler(comboTests),
		vision:    handler.NewVisionAdapterHandler(adapter),
		proxy:     newProxyRouteHandler(t),
		media:     newMediaRouteHandler(t),
	}
}

// seedRouteFixture stores the combo the route tests address by id.
func seedRouteFixture(t *testing.T, combos *memComboRepo) {
	t.Helper()
	model, err := domain.NewComboModel("openai/gpt-4o", 1)
	if err != nil {
		t.Fatalf("NewComboModel() error = %v", err)
	}
	combo, err := domain.NewCombo("cmb_seeded", "seeded-combo", domain.ComboFallback, 0, "",
		[]domain.ComboModel{model}, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	if err := combos.Create(context.Background(), combo); err != nil {
		t.Fatalf("seeding a combo: %v", err)
	}
}
