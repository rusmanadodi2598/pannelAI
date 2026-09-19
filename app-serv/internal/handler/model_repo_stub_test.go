// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/model_repo_stub_test.go
// @for       The in-memory combo and adapter repositories for the handler tests, and the wired handler fixture they serve.
// @uses      internal/domain, internal/registry, internal/repository,
//
//	internal/service, context, testing.
//
// @reason    The catalog stub and the combo/adapter stubs are separate declarations; AGENTS.md §1.1 caps a file at 250 lines, so the latter moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

func (r *stubComboRepo) Delete(_ context.Context, id string) error {
	combo, ok := r.byID[id]
	if !ok {
		return domain.ErrComboNotFound
	}
	delete(r.nameID, combo.Name())
	delete(r.byID, id)
	return nil
}

// stubAdapterRepo is an in-memory VisionAdapterRepository.
type stubAdapterRepo struct {
	adapter domain.VisionAdapter
}

func newStubAdapterRepo() *stubAdapterRepo {
	return &stubAdapterRepo{adapter: domain.DefaultVisionAdapter()}
}

func (r *stubAdapterRepo) Get(context.Context) (domain.VisionAdapter, error) { return r.adapter, nil }

func (r *stubAdapterRepo) Save(_ context.Context, adapter domain.VisionAdapter) error {
	r.adapter = adapter
	return nil
}

// stubRotation is an in-memory ComboRotationStore for the handler fixture: the
// management routes never rotate, so it returns the stored order.
type stubRotation struct{}

func (stubRotation) Next(_ context.Context, _ string, models []string, _ int) ([]string, error) {
	return models, nil
}

// managementFixture is the wired handler set the §7.6–§7.8 tests drive.
type managementFixture struct {
	model     *ModelHandler
	combo     *ComboHandler
	comboTest *ComboTestHandler
	vision    *VisionAdapterHandler
	proxy     *ProxyHandler
	// prober is the combo test route's seam double, exposed so a case can make
	// a member fail without rebuilding the fixture.
	prober *stubProber
	// proxyProber and proxyRepo are the §7.11 seams, exposed for the same
	// reason: a case can turn a probe into a failure or inspect what the
	// routes stored.
	proxyProber *stubProxyProber
	proxyRepo   *stubProxyRepo
}

// newManagementFixture wires the handlers over a small fixed registry with one
// combo and one alias, so every case starts from the same observable state.
func newManagementFixture(t *testing.T) managementFixture {
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
	catalogRepo := newStubCatalogRepo()
	comboRepo := newStubComboRepo()
	catalog, err := service.NewModelCatalogService(service.ModelCatalogServiceDeps{
		Index: index, Repo: catalogRepo, Combos: comboRepo,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	comboService, err := service.NewComboService(service.ComboServiceDeps{
		Repo: comboRepo, Catalog: catalog, Rotation: stubRotation{},
	})
	if err != nil {
		t.Fatalf("NewComboService() error = %v", err)
	}
	adapterService, err := service.NewVisionAdapterService(service.VisionAdapterServiceDeps{
		Repo: newStubAdapterRepo(), Catalog: catalog,
	})
	if err != nil {
		t.Fatalf("NewVisionAdapterService() error = %v", err)
	}
	prober := newStubProber()
	comboTestService, err := service.NewComboTestService(comboService, prober)
	if err != nil {
		t.Fatalf("NewComboTestService() error = %v", err)
	}
	proxyProber := &stubProxyProber{result: service.ProxyProbeResult{State: domain.EndpointTestOK, LatencyMS: 7}}
	proxyService, proxyRepo := newStubProxyService(t, proxyProber)
	seedHandlerFixture(t, catalog, comboRepo)
	return managementFixture{
		model:       NewModelHandler(catalog),
		combo:       NewComboHandler(comboService),
		comboTest:   NewComboTestHandler(comboTestService),
		vision:      NewVisionAdapterHandler(adapterService),
		proxy:       NewProxyHandler(proxyService),
		prober:      prober,
		proxyProber: proxyProber,
		proxyRepo:   proxyRepo,
	}
}

// seedHandlerFixture stores the combo the tests address by id and the disabled
// pair they expect the catalog to hide.
func seedHandlerFixture(t *testing.T, catalog *service.ModelCatalogService, repo *stubComboRepo) {
	t.Helper()
	ctx := context.Background()
	model, err := domain.NewComboModel("openai/gpt-4o", 1)
	if err != nil {
		t.Fatalf("NewComboModel() error = %v", err)
	}
	combo, err := domain.NewCombo("cmb_seeded", "seeded-combo", domain.ComboFallback, 0, "",
		[]domain.ComboModel{model}, handlerNow())
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	if err := repo.Create(ctx, combo); err != nil {
		t.Fatalf("seeding a combo: %v", err)
	}
	// The fixture's comment promises one alias, and the §7.7 reference forms
	// include an alias, so the alias the happy-path case names has to exist for
	// that case to test resolution rather than rejection.
	alias, err := domain.NewModelAlias("fast", "openai/gpt-4o")
	if err != nil {
		t.Fatalf("NewModelAlias() error = %v", err)
	}
	if err := catalog.ReplaceAliases(ctx, []domain.ModelAlias{alias}); err != nil {
		t.Fatalf("seeding the alias set: %v", err)
	}
	disabled, err := domain.NewModelRef("anthropic", "claude-3")
	if err != nil {
		t.Fatalf("NewModelRef() error = %v", err)
	}
	if err := catalog.ReplaceDisabled(ctx, []domain.ModelRef{disabled}); err != nil {
		t.Fatalf("seeding the disabled set: %v", err)
	}
}
