// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_active_test.go
// @for       The active-provider half of the catalog read: `?active=true`
//
//	answers only rows whose provider holds at least one active
//	endpoint, because that is the population the router can serve.
//
// @uses      internal/domain, internal/registry, context, testing.
// @reason    Draft 025 F1/F2 measured the picker offering 586 of 587 rows
//
//	that fail NO_PROVIDER_AVAILABLE on the first request: the panel
//	asked for "only what is actually usable", and the route had no
//	parameter to answer it. "Active" is the router's own population —
//	its candidates query selects endpoints with status active under
//	the provider's canonical id — so the predicate is stated in those
//	terms rather than invented here: one status-active endpoint makes
//	the provider active, a rate-limited active endpoint keeps it
//	active (the runtime skip is a moment, not a configuration), and
//	disabled or errored endpoints do not. A deployment that wires no
//	counter is allowed everywhere else in the read graph, so asking
//	the question it cannot answer is a named refusal rather than a
//	silent unfiltered answer, which would read as "everything works".
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
)

// stubActiveCounts answers the candidate question a deployment's reader holds,
// with the same shape the PostgreSQL seam returns: one map keyed by provider id.
//
// The values are domain.EndpointStatusCounts so a table can be written the way
// an operator thinks about a provider ("two active, one rate-limited") and the
// stub translates it into the one fact the seam carries: whether a candidate
// exists at all. That translation is the point — the seam deliberately does not
// expose the roll-up, because the roll-up cannot express the candidate set.
type stubActiveCounts struct {
	byProvider map[string]domain.EndpointStatusCounts
}

func (c stubActiveCounts) ActiveProviders(_ context.Context, ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		summary := c.byProvider[id]
		out[id] = summary.Active > 0 || summary.RateLimited > 0
	}
	return out, nil
}

// activeCatalogService wires the service over a fixed three-provider registry
// and the given counter, so every case starts from the same catalog.
func activeCatalogService(t *testing.T, active ActiveProviderSet) *ModelCatalogService {
	t.Helper()
	index := testIndex(t,
		testProvider("openai", "api",
			testModel("gpt-4o", "GPT-4o", "llm"),
			testModel("gpt-4o-mini", "GPT-4o mini", "llm")),
		testProvider("anthropic", "api", testModel("claude-3", "Claude 3", "llm")),
		testProvider("kserve", "api", testModel("glm-4.7", "GLM 4.7", "llm")),
	)
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: index, Repo: newStubCatalogRepo(), Combos: newStubComboRepo(), Active: active,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	return service
}

// wantCatalog names what a filter answer must be, so the tables stay readable
// while the assertion stays exact (order included).
func wantCatalog(t *testing.T, service *ModelCatalogService, filter CatalogFilter, want []string) {
	t.Helper()
	models, err := service.Catalog(context.Background(), filter)
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if got := catalogKeys(models); !equalStrings(got, want) {
		t.Fatalf("Catalog() = %v, want %v", got, want)
	}
}

// TestModelCatalogService_CatalogActiveFilter is the F1 core: active=true keeps
// only the providers holding an active endpoint, in combination with the other
// filters, and the default stays the unfiltered catalog.
func TestModelCatalogService_CatalogActiveFilter(t *testing.T) {
	active := true
	cases := []struct {
		name   string
		counts map[string]domain.EndpointStatusCounts
		filter CatalogFilter
		want   []string
	}{
		{
			name: "one active endpoint makes the provider active",
			counts: map[string]domain.EndpointStatusCounts{
				"openai": {Total: 1, Active: 1},
			},
			filter: CatalogFilter{Active: &active},
			want:   []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
		},
		{
			name: "several active providers each keep their rows",
			counts: map[string]domain.EndpointStatusCounts{
				"openai":    {Total: 2, Active: 2},
				"anthropic": {Total: 1, Active: 1},
				"kserve":    {Total: 3, Active: 1, Error: 2},
			},
			filter: CatalogFilter{Active: &active},
			want: []string{
				"anthropic/claude-3", "kserve/glm-4.7",
				"openai/gpt-4o", "openai/gpt-4o-mini",
			},
		},
		{
			name: "a rate-limited active endpoint keeps the provider active",
			counts: map[string]domain.EndpointStatusCounts{
				"kserve": {Total: 2, Active: 1, RateLimited: 1},
			},
			filter: CatalogFilter{Active: &active},
			want:   []string{"kserve/glm-4.7"},
		},
		{
			name: "only disabled endpoints leave the provider inactive",
			counts: map[string]domain.EndpointStatusCounts{
				"anthropic": {Total: 2, Disabled: 2},
			},
			filter: CatalogFilter{Active: &active},
			want:   []string{},
		},
		{
			name: "only errored endpoints leave the provider inactive",
			counts: map[string]domain.EndpointStatusCounts{
				"openai": {Total: 1, Error: 1},
			},
			filter: CatalogFilter{Active: &active},
			want:   []string{},
		},
		{
			name:   "no measured provider is active",
			counts: map[string]domain.EndpointStatusCounts{},
			filter: CatalogFilter{Active: &active},
			want:   []string{},
		},
		{
			name: "the default still answers the whole catalog",
			counts: map[string]domain.EndpointStatusCounts{
				"openai": {Total: 1, Active: 1},
			},
			filter: CatalogFilter{},
			want: []string{
				"anthropic/claude-3", "kserve/glm-4.7",
				"openai/gpt-4o", "openai/gpt-4o-mini",
			},
		},
		{
			name: "active narrows inside a provider filter",
			counts: map[string]domain.EndpointStatusCounts{
				"openai":    {Total: 1, Active: 1},
				"anthropic": {Total: 1, Active: 1},
			},
			filter: CatalogFilter{ProviderID: "openai", Active: &active},
			want:   []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
		},
		{
			name: "a provider filter naming an inactive provider answers empty",
			counts: map[string]domain.EndpointStatusCounts{
				"openai": {Total: 1, Active: 1},
			},
			filter: CatalogFilter{ProviderID: "kserve", Active: &active},
			want:   []string{},
		},
		{
			name: "active combines with capability",
			counts: map[string]domain.EndpointStatusCounts{
				"openai": {Total: 1, Active: 1},
			},
			// The resolver answers `vision` for both gpt-4o ids (the existing
			// capability table pins that), so this case proves the two filters
			// compose rather than that either one narrows.
			filter: CatalogFilter{Capability: "vision", Active: &active},
			want:   []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
		},
		{
			name: "active combines with free text",
			counts: map[string]domain.EndpointStatusCounts{
				"openai":    {Total: 1, Active: 1},
				"anthropic": {Total: 1, Active: 1},
			},
			filter: CatalogFilter{Query: "gpt", Active: &active},
			want:   []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := activeCatalogService(t, stubActiveCounts{byProvider: tc.counts})
			wantCatalog(t, service, tc.filter, tc.want)
		})
	}
}
