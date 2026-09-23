// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_models_test.go
// @for       The origin and the fallback a node's model list answers with.
//
// @uses      internal/domain, internal/registry, context, errors, testing.
// @reason    SPEC-API-001 §7.4 serves a node's models, and draft 017 §4.2
//
//	measured the two failures this covers: a node whose upstream answers
//	was reported as having no models at all, and a node whose upstream is
//	down had no list to fall back to. The origin is asserted separately
//	from the list because a client reads them together: a fallback
//	presented as current is the failure `source` exists to prevent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stubNodeModelSource is a NodeModelSource that answers a fixed value or a
// fixed error, and counts its calls so a test can assert the source is not
// consulted for a registry provider.
type stubNodeModelSource struct {
	list  NodeModelList
	err   error
	calls int
}

func (s *stubNodeModelSource) ListNodeModels(context.Context, string) (NodeModelList, error) {
	s.calls++
	return s.list, s.err
}

// customNodeEntry is a synthesized node's registry entry, with no models: it is
// what the overlay produces for a node whose list could not be read.
func customNodeEntry(id string) registry.Provider {
	return registry.Provider{ID: id, Category: "apikey", Custom: true, Transport: registry.Transport{Format: "openai"}}
}

// customNodeEntryWithModels is the same entry carrying a declared list, which is
// what a fallback answers with.
func customNodeEntryWithModels(id string, modelIDs ...string) registry.Provider {
	entry := customNodeEntry(id)
	for _, modelID := range modelIDs {
		entry.Models = append(entry.Models, registry.Model{ID: modelID})
	}
	return entry
}

// newProviderServiceWithSource wires a provider service over a one-entry index.
//
// A nil *stubNodeModelSource is normalized to an unset interface rather than
// assigned: a typed nil in an interface field is non-nil, so the service would
// take the "a source is wired" branch and panic on the call. That trap is the
// reason this helper exists instead of each test building the deps inline.
func newProviderServiceWithSource(t *testing.T, entry registry.Provider, source *stubNodeModelSource) *ProviderService {
	t.Helper()
	deps := ProviderServiceDeps{Index: readinessProviderIndex{entries: []registry.Provider{entry}}}
	if source != nil {
		deps.Source = source
	}
	svc, err := NewProviderService(deps)
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	return svc
}

// TestProviderService_Models_ReportsTheOriginOfTheList covers every outcome a
// node's list can have, plus the registry provider that never consults a source.
func TestProviderService_Models_ReportsTheOriginOfTheList(t *testing.T) {
	cases := []struct {
		name        string
		entry       registry.Provider
		source      *stubNodeModelSource
		wantSource  string
		wantModels  []string
		wantWarning string
		wantCalls   int
	}{
		{
			name:       "a registry provider answers from the registry",
			entry:      testProvider("openai", "api", testModel("gpt-4o", "GPT-4o", "llm")),
			source:     &stubNodeModelSource{list: NodeModelList{Source: ModelSourceUpstream}},
			wantSource: ModelSourceRegistry,
			wantModels: []string{"gpt-4o"},
		},
		{
			name:       "a node whose upstream answers reports upstream",
			entry:      customNodeEntry("openai-compatible-01TEST"),
			source:     &stubNodeModelSource{list: NodeModelList{Models: []registry.Model{{ID: "up-1"}, {ID: "up-2"}}, Source: ModelSourceUpstream}},
			wantSource: ModelSourceUpstream,
			wantModels: []string{"up-1", "up-2"},
			wantCalls:  1,
		},
		{
			name:  "a node whose upstream fails falls back to the declared list with a warning",
			entry: customNodeEntryWithModels("openai-compatible-01TEST", "declared-1"),
			source: &stubNodeModelSource{list: NodeModelList{
				Source:  ModelSourceRegistry,
				Warning: UpstreamUnavailableWarning,
			}},
			wantSource:  ModelSourceRegistry,
			wantModels:  []string{"declared-1"},
			wantWarning: UpstreamUnavailableWarning,
			wantCalls:   1,
		},
		{
			name:       "a node with no source configured answers the declared list",
			entry:      customNodeEntryWithModels("openai-compatible-01TEST", "declared-1"),
			source:     nil,
			wantSource: ModelSourceRegistry,
			wantModels: []string{"declared-1"},
		},
		{
			name:        "a source that errors is not a 5xx",
			entry:       customNodeEntryWithModels("openai-compatible-01TEST", "declared-1"),
			source:      &stubNodeModelSource{err: errors.New("the source is unavailable")},
			wantSource:  ModelSourceRegistry,
			wantModels:  []string{"declared-1"},
			wantWarning: UpstreamUnavailableWarning,
			wantCalls:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newProviderServiceWithSource(t, tc.entry, tc.source)
			got, err := svc.Models(context.Background(), tc.entry.ID)
			if err != nil {
				t.Fatalf("Models(%q) error = %v, want nil: a node's upstream being down is an answer, not a fault", tc.entry.ID, err)
			}
			if got.Source != tc.wantSource {
				t.Fatalf("Models(%q).Source = %q, want %q", tc.entry.ID, got.Source, tc.wantSource)
			}
			if got.Warning != tc.wantWarning {
				t.Fatalf("Models(%q).Warning = %q, want %q", tc.entry.ID, got.Warning, tc.wantWarning)
			}
			ids := make([]string, 0, len(got.Entry.Models))
			for _, model := range got.Entry.Models {
				ids = append(ids, model.ID)
			}
			if !equalStrings(ids, tc.wantModels) {
				t.Fatalf("Models(%q) models = %v, want %v", tc.entry.ID, ids, tc.wantModels)
			}
			if tc.source != nil && tc.source.calls != tc.wantCalls {
				t.Fatalf("the source was called %d times, want %d", tc.source.calls, tc.wantCalls)
			}
		})
	}
}

// TestProviderService_Models_UnknownProviderStillFails pins that the origin
// plumbing did not turn a genuine not-found into an empty list.
func TestProviderService_Models_UnknownProviderStillFails(t *testing.T) {
	svc := newProviderServiceWithSource(t, customNodeEntry("openai-compatible-01TEST"), nil)
	_, err := svc.Models(context.Background(), "no-such-provider")
	if err == nil {
		t.Fatal("Models(unknown) returned no error; an unknown provider is not an empty model list")
	}
}
