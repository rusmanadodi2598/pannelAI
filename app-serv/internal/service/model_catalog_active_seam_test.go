// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_active_seam_test.go
// @for       The seam discipline behind `?active=true`: the roll-up is asked
//
//	once, only when the parameter asks for it, and a deployment that
//	wires none refuses the question instead of answering it wrongly.
//
// @uses      internal/domain, context, sync, testing.
// @reason    Draft 025 F4/F2: the predicate is one query per read, not one per
//
//	provider, and a missing counter must not read as "nothing is
//	active" — an empty answer and an unanswerable question are
//	different facts, and the caller has to be able to tell them apart.
//	Separated from the filter table at the AGENTS.md §1.1 line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// countingActiveCounter counts how many times the roll-up was asked, and the
// widest id list one call carried. It exists so a read that asks per provider
// cannot pass as one query: the count and the width are both assertions.
type countingActiveCounter struct {
	mu    sync.Mutex
	calls int
	width int
}

func (c *countingActiveCounter) ActiveProviders(_ context.Context, ids []string) (map[string]bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if len(ids) > c.width {
		c.width = len(ids)
	}
	return map[string]bool{}, nil
}

func (c *countingActiveCounter) snapshot() (calls, width int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls, c.width
}

// TestModelCatalogService_CatalogActiveRequiresTheCounter pins the nil-seam
// refusal: a deployment that wires no counter cannot answer "which providers
// are active", and pretending otherwise would return the unfiltered catalog
// under a parameter that promised the opposite.
func TestModelCatalogService_CatalogActiveRequiresTheCounter(t *testing.T) {
	active := true
	service := activeCatalogService(t, nil)
	_, err := service.Catalog(context.Background(), CatalogFilter{Active: &active})
	if err == nil {
		t.Fatal("Catalog() error = nil, want a refusal naming the missing counter")
	}
	if code := domain.AsAppError(err).Code; code != "INTERNAL_ERROR" {
		t.Fatalf("Catalog() error code = %q, want INTERNAL_ERROR", code)
	}
	// The unfiltered read stays available without the seam: only the question
	// that needs it is refused.
	wantCatalog(t, service, CatalogFilter{}, []string{
		"anthropic/claude-3", "kserve/glm-4.7", "openai/gpt-4o", "openai/gpt-4o-mini",
	})
}

// TestModelCatalogService_CatalogActiveCountsOnce proves the F4 discipline: one
// active read asks the roll-up once, with every candidate provider id in that
// one call, so the read cannot become one query per provider.
func TestModelCatalogService_CatalogActiveCountsOnce(t *testing.T) {
	active := true
	counter := &countingActiveCounter{}
	service := activeCatalogService(t, counter)
	wantCatalog(t, service, CatalogFilter{Active: &active}, []string{})
	calls, width := counter.snapshot()
	if calls != 1 {
		t.Fatalf("the counter was asked %d times, want 1", calls)
	}
	if width != 3 {
		t.Fatalf("the one call carried %d provider ids, want 3", width)
	}
}

// TestModelCatalogService_CatalogWithoutActiveNeverCounts proves the predicate
// is asked for, not assumed: a read that did not ask for active does not touch
// the roll-up at all, which is what keeps the plain catalog read free of a
// query it does not need.
func TestModelCatalogService_CatalogWithoutActiveNeverCounts(t *testing.T) {
	counter := &countingActiveCounter{}
	service := activeCatalogService(t, counter)
	wantCatalog(t, service, CatalogFilter{ProviderID: "openai"}, []string{"openai/gpt-4o", "openai/gpt-4o-mini"})
	if calls, _ := counter.snapshot(); calls != 0 {
		t.Fatalf("the counter was asked %d times without the active filter, want 0", calls)
	}
}

// TestModelCatalogService_CatalogActiveAcceptsEveryProviderSpelling keeps the
// draft 024 F3 rule intact under the new predicate: the provider filter still
// accepts the registry alias, the rows are stored under the canonical id, and
// the active question is asked about that canonical id.
func TestModelCatalogService_CatalogActiveAcceptsEveryProviderSpelling(t *testing.T) {
	active := true
	index := testIndex(t, registry.Provider{
		ID: "kserve", Priority: 1, Alias: "ks", Category: "api",
		Transport: registry.Transport{Format: "openai"},
		Models:    []registry.Model{testModel("glm-4.7", "GLM 4.7", "llm")},
	})
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: index, Repo: newStubCatalogRepo(), Combos: newStubComboRepo(),
		Active: stubActiveCounts{byProvider: map[string]domain.EndpointStatusCounts{
			"kserve": {Total: 1, Active: 1},
		}},
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	wantCatalog(t, service, CatalogFilter{ProviderID: "ks", Active: &active}, []string{"kserve/glm-4.7"})
	// An explicit false means "do not narrow" (§6 ruling 2), so the alias form
	// answers the same rows as the unfiltered read.
	wantCatalog(t, service, CatalogFilter{ProviderID: "ks", Active: ptrFalse()}, []string{"kserve/glm-4.7"})
}

// ptrFalse is the explicit-false filter, which means "do not narrow" (draft 025
// §6 ruling 2) and is spelled once so the case above reads as intent.
func ptrFalse() *bool {
	value := false
	return &value
}
