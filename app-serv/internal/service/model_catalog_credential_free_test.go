// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_credential_free_test.go
// @for       The `?active=true` predicate for a credential-free provider that
//
//	holds no endpoint row, which the router serves anyway.
//
// @uses      context, testing, internal/domain, internal/registry.
// @reason    Draft 029 §4.8 F8 taught selection to synthesize a virtual
//
//	endpoint for a credential-free provider with no stored row, so
//	`opencode/space-bunny-free` answers 200 on an empty endpoint table.
//	The catalog's active filter was written against the older premise
//	("no row means not a candidate"), so it hid the whole free lane from
//	the panel while the router answered it — the disagreement that made
//	the picker offer nothing usable and drove the operator to build a
//	custom node for a provider needing no configuration. These cases pin
//	the two halves together: the filter must add a credential-free
//	provider, and must not invent one that wants a key.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// credentialFreeCatalog wires the catalog over one credential-free provider and
// one keyed provider, each carrying an id the router could serve, and counts no
// endpoint row for either (the measured state of the owner's `oczen` node and of
// the `opencode` free lane alike).
func credentialFreeCatalog(t *testing.T, counts map[string]domain.EndpointStatusCounts) *ModelCatalogService {
	t.Helper()
	free := registry.Provider{
		ID: "opencode", Priority: 1, Category: "free", NoAuth: true,
		Models: []registry.Model{testModel("space-bunny-free", "Space Bunny", "llm")},
	}
	keyed := registry.Provider{
		ID: "anthropic", Priority: 2, Category: "apikey", AuthType: registry.AuthAPIKey,
		Models: []registry.Model{testModel("claude-3", "Claude 3", "llm")},
	}
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index:  testIndex(t, free, keyed),
		Repo:   newStubCatalogRepo(),
		Combos: newStubComboRepo(),
		Active: stubActiveCounts{byProvider: counts},
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	return service
}

// TestModelCatalogService_ActiveOffersACredentialFreeProvider pins the rule F8
// made necessary: with no endpoint row at all, the credential-free provider is
// still offered because the router synthesizes its endpoint, while the keyed
// provider with no row is not.
func TestModelCatalogService_ActiveOffersACredentialFreeProvider(t *testing.T) {
	active := true
	service := credentialFreeCatalog(t, map[string]domain.EndpointStatusCounts{})

	models, err := service.Catalog(context.Background(), CatalogFilter{Active: &active})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if got := catalogKeys(models); !equalStrings(got, []string{"opencode/space-bunny-free"}) {
		t.Fatalf("Catalog(active=true) = %v, want only the credential-free provider's row", got)
	}
}

// TestModelCatalogService_ActiveStillDropsAKeyedProviderWithoutARow is the control:
// the same empty table must not make a provider that wants a key look routable,
// because selection refuses it (NO_PROVIDER_AVAILABLE) rather than inventing a
// credential.
func TestModelCatalogService_ActiveStillDropsAKeyedProviderWithoutARow(t *testing.T) {
	active := true
	service := credentialFreeCatalog(t, map[string]domain.EndpointStatusCounts{
		"opencode": {Total: 1, Active: 1},
	})

	models, err := service.Catalog(context.Background(), CatalogFilter{Active: &active})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if got := catalogKeys(models); !equalStrings(got, []string{"opencode/space-bunny-free"}) {
		t.Fatalf("Catalog(active=true) = %v, want the credential-free row only", got)
	}
}

// TestModelCatalogService_ActiveTreatsATransportNoAuthEntryAsCredentialFree pins
// the second spelling the document uses: the reference writes `noAuth` on the
// transport for some entries, so a filter reading only the provider level would
// hide those providers.
func TestModelCatalogService_ActiveTreatsATransportNoAuthEntryAsCredentialFree(t *testing.T) {
	active := true
	free := registry.Provider{
		ID: "opencode", Priority: 1, Category: "free",
		Transport: registry.Transport{NoAuth: true},
		Models:    []registry.Model{testModel("space-bunny-free", "Space Bunny", "llm")},
	}
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index:  testIndex(t, free),
		Repo:   newStubCatalogRepo(),
		Combos: newStubComboRepo(),
		Active: stubActiveCounts{byProvider: map[string]domain.EndpointStatusCounts{}},
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}

	models, err := service.Catalog(context.Background(), CatalogFilter{Active: &active})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if got := catalogKeys(models); !equalStrings(got, []string{"opencode/space-bunny-free"}) {
		t.Fatalf("Catalog(active=true) = %v, want the transport-level no_auth entry offered", got)
	}
}
