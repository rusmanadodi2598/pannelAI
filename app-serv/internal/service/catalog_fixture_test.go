// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/catalog_fixture_test.go
// @for       The shared catalog fixture: a small registry plus the wired catalog
//
//	service the §7.6 and §7.7 tests run against.
//
// @uses      internal/domain, internal/registry, context, testing, time.
// @reason    The catalog merges three sources, so a test of the merge must be
//
//	able to place rows in each one independently. Naming the fixture's
//	providers locally keeps every assertion independent of the embedded
//	registry's contents, which change with the reference.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// catalogTestNow is the instant the service tests date rows with.
func catalogTestNow() time.Time { return time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC) }

// testIndex builds a small registry so a catalog test names its own providers
// and models instead of depending on the embedded document's contents.
func testIndex(t *testing.T, providers ...registry.Provider) *registry.Index {
	t.Helper()
	index, err := registry.NewIndex(registry.Document{Revision: "test", Providers: providers})
	if err != nil {
		t.Fatalf("registry.NewIndex() error = %v", err)
	}
	return index
}

// testProvider is a minimal registry entry with one or more models.
func testProvider(id, category string, models ...registry.Model) registry.Provider {
	return registry.Provider{ID: id, Priority: 1, Category: category, Models: models}
}

// testModel is a registry model with the given capabilities.
func testModel(id, name, kind string, capabilities ...string) registry.Model {
	return registry.Model{ID: id, Name: name, Kind: kind, Capabilities: capabilities}
}

// newCatalogService wires a catalog service over in-memory repositories.
func newCatalogService(t *testing.T, index *registry.Index, repo *stubCatalogRepo, combos *stubComboRepo) *ModelCatalogService {
	t.Helper()
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{Index: index, Repo: repo, Combos: combos})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	return service
}

// catalogFixture is the shared three-source fixture: two registry providers,
// one custom model, one disabled pair, and a combo repository for the reference
// checks.
type catalogFixture struct {
	repo    *stubCatalogRepo
	combos  *stubComboRepo
	service *ModelCatalogService
}

// newCatalogFixture builds the fixture described on catalogFixture. The seeding
// calls carry the caller's context, so a test's setup answers to the same
// cancellation its assertions do.
func newCatalogFixture(t *testing.T, ctx context.Context) catalogFixture {
	t.Helper()
	index := testIndex(t,
		testProvider("openai", "api",
			testModel("gpt-4o", "GPT-4o", "llm", "vision", "tools"),
			testModel("gpt-4o-mini", "GPT-4o mini", "llm", "tools")),
		testProvider("anthropic", "api", testModel("claude-3", "Claude 3", "llm", "vision")),
	)
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	if err := repo.AddCustom(ctx, mustCustomModel(t, "openai", "local-embed", "Local Embed", "embedding")); err != nil {
		t.Fatalf("seeding a custom model: %v", err)
	}
	disabled, err := domain.NewModelRef("anthropic", "claude-3")
	if err != nil {
		t.Fatalf("NewModelRef() error = %v", err)
	}
	repo.disabled = []domain.ModelRef{disabled}
	return catalogFixture{repo: repo, combos: combos, service: newCatalogService(t, index, repo, combos)}
}

// mustCustomModel builds a custom model with one capability.
func mustCustomModel(t *testing.T, providerID, modelID, displayName, capability string) domain.CustomModel {
	t.Helper()
	model, err := domain.NewCustomModel("mdl_"+modelID, providerID, modelID, displayName,
		domain.NewModelCapabilities(capability), catalogTestNow())
	if err != nil {
		t.Fatalf("NewCustomModel(%q) error = %v", modelID, err)
	}
	return model
}
