// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_search_test.go
// @for       The `?q` filter of the provider list: what a query matches, and
// //
//
//	what a blank or unmatched one answers.
//
// @uses      internal/registry, context, testing.
// @reason    SPEC-UI §14 Q13 left the search to the API, and the failure this
//
//	file pins is the silent one: before `q` existed the route answered
//	the unfiltered registry to every spelling, so a panel search would
//	have looked alive while filtering nothing. The matching rule is
//	service behavior, so it is proven here rather than through the wire.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// searchProviders builds the index the query cases run against. The names
// differ from the ids on purpose: `azure-openai` carries the query in both
// fields, `claude-local` only in its name, so a rule that matched the id alone
// or the name alone would fail a case rather than pass by luck.
func searchProviders() readinessProviderIndex {
	return readinessProviderIndex{entries: []registry.Provider{
		{ID: "openai", Priority: 1, Category: "apikey", Display: registry.Display{Name: "OpenAI"}, Transport: registry.Transport{Format: "openai"}},
		{ID: "azure-openai", Priority: 2, Category: "apikey", Display: registry.Display{Name: "Azure OpenAI"}, Transport: registry.Transport{Format: "openai"}},
		{ID: "claude-local", Priority: 3, Category: "local", Display: registry.Display{Name: "Claude via Ollama"}, Transport: registry.Transport{Format: "unknown"}},
		{ID: "anthropic", Priority: 4, Category: "apikey", Display: registry.Display{Name: "Anthropic"}, Transport: registry.Transport{Format: "anthropic"}},
	}}
}

// TestProviderService_QueryMatchesIDOrName pins the matching rule as a table:
// the query is a case-insensitive substring of the id or the display name, a
// blank query narrows nothing, and a query that matches nothing answers an
// empty page rather than the whole registry.
func TestProviderService_QueryMatchesIDOrName(t *testing.T) {
	svc, err := NewProviderService(ProviderServiceDeps{Index: searchProviders()})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "absent narrows nothing", query: "", want: []string{"openai", "azure-openai", "claude-local", "anthropic"}},
		{name: "blank narrows nothing", query: "   ", want: []string{"openai", "azure-openai", "claude-local", "anthropic"}},
		{name: "a substring of the id matches both rows carrying it", query: "openai", want: []string{"openai", "azure-openai"}},
		{name: "case does not decide the match", query: "OPENAI", want: []string{"openai", "azure-openai"}},
		{name: "the surrounding whitespace is trimmed before matching", query: "  azure  ", want: []string{"azure-openai"}},
		{name: "a match in the display name alone still finds the row", query: "ollama", want: []string{"claude-local"}},
		{name: "a prefix of a name matches", query: "anthro", want: []string{"anthropic"}},
		{name: "no match answers an empty page, not the registry", query: "zzzz-nothing", want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := svc.List(context.Background(), ProviderFilter{Q: tc.query}, 1, 100)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if int(total) != len(tc.want) {
				t.Fatalf("total = %d, want %d", total, len(tc.want))
			}
			if len(rows) != len(tc.want) {
				t.Fatalf("rows = %d, want %d", len(rows), len(tc.want))
			}
			for i, want := range tc.want {
				if rows[i].Entry.ID != want {
					t.Fatalf("rows[%d].ID = %q, want %q", i, rows[i].Entry.ID, want)
				}
			}
		})
	}
}

// TestProviderService_QueryComposesWithTheOtherFilters pins the AND: a query
// narrows the set the category and routability filters already produced, and
// the total is the filtered count, so the panel's paging cannot claim rows the
// filter removed.
func TestProviderService_QueryComposesWithTheOtherFilters(t *testing.T) {
	svc, err := NewProviderService(ProviderServiceDeps{Index: searchProviders()})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}

	cases := []struct {
		name   string
		filter ProviderFilter
		want   []string
	}{
		{name: "query and category agree", filter: ProviderFilter{Q: "openai", Category: "apikey"}, want: []string{"openai", "azure-openai"}},
		{name: "category removes a query match", filter: ProviderFilter{Q: "openai", Category: "local"}, want: nil},
		{name: "query and routability agree", filter: ProviderFilter{Q: "claude", Routability: registry.RoutableNeedsConnector}, want: []string{"claude-local"}},
		{name: "routability removes a query match", filter: ProviderFilter{Q: "claude", Routability: registry.Routable}, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := svc.List(context.Background(), tc.filter, 1, 100)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if int(total) != len(tc.want) || len(rows) != len(tc.want) {
				t.Fatalf("total = %d, rows = %d, want %d", total, len(rows), len(tc.want))
			}
			for i, want := range tc.want {
				if rows[i].Entry.ID != want {
					t.Fatalf("rows[%d].ID = %q, want %q", i, rows[i].Entry.ID, want)
				}
			}
		})
	}

	// The window is sliced from the filtered set, so page two of a two-row
	// match holds the second row and never a row the query removed.
	rows, total, err := svc.List(context.Background(), ProviderFilter{Q: "openai"}, 2, 1)
	if err != nil {
		t.Fatalf("List() page 2 error = %v", err)
	}
	if total != 2 || len(rows) != 1 || rows[0].Entry.ID != "azure-openai" {
		t.Fatalf("page 2 = %+v, total = %d, want the second match only", rows, total)
	}
}
