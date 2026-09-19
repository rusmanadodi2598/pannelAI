// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_media_stub_test.go
// @for       The in-memory override store and media index the §7.10 route tests
//
//	drive.
//
// @uses      internal/domain, internal/handler, internal/registry,
//
//	internal/service, context, sort, testing.
//
// @reason    AGENTS.md §1.1 caps a file at 250 lines, so the doubles live apart
//
//	from the route table. The handler takes a concrete
//	*service.MediaProviderService, so these build the real service over
//	in-memory collaborators rather than faking the service itself.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// newMediaRouteHandler wires the §7.10 management service over a small fixed
// registry and an in-memory override store.
func newMediaRouteHandler(t *testing.T) *handler.MediaProviderHandler {
	t.Helper()
	svc, err := service.NewMediaProviderService(service.MediaProviderServiceDeps{
		Index: mediaRouteIndex(),
		Repo:  newMemMediaOverrideRepo(),
	})
	if err != nil {
		t.Fatalf("NewMediaProviderService() error = %v", err)
	}
	return handler.NewMediaProviderHandler(svc)
}

// newMediaDataPlaneHandler wires the §7.10 data-plane handler over doubles: a
// caller that answers a canned image envelope and an authenticator that accepts
// any key, so a route test drives the pipeline without a network or a database.
func newMediaDataPlaneHandler(t *testing.T) *handler.MediaHandler {
	t.Helper()
	svc, err := service.NewMediaCallService(service.MediaCallServiceDeps{
		Index:  mediaRouteIndex(),
		Router: routeMediaRouter{},
		Caller: &routeMediaCaller{},
	})
	if err != nil {
		t.Fatalf("NewMediaCallService() error = %v", err)
	}
	return handler.NewMediaHandler(svc, routeMediaAuth{})
}

// routeMediaAuth accepts every request, so a route test can reach the pipeline.
// The refusal path is pinned in the handler package, where the envelope is the
// subject rather than the routing.
type routeMediaAuth struct{}

func (routeMediaAuth) Authenticate(context.Context, string) (domain.GatewayKey, error) {
	return domain.GatewayKey{}, nil
}

// routeMediaRouter answers selection with one fixed endpoint.
type routeMediaRouter struct{}

func (routeMediaRouter) Select(_ context.Context, providerID string) (dataplane.Selection, error) {
	endpoint, err := domain.NewUpstreamEndpoint("ep-"+providerID, providerID, "primary",
		domain.UpstreamAuthAPIKey, 1, time.Now().UTC())
	if err != nil {
		return dataplane.Selection{}, err
	}
	return dataplane.Selection{Endpoint: endpoint}, nil
}

func (routeMediaRouter) RecordSuccess(context.Context, dataplane.Selection) error { return nil }
func (routeMediaRouter) RecordFailure(context.Context, dataplane.Selection, string) error {
	return nil
}

// routeMediaCaller answers every media call with one body carrying both the
// generation envelope and a search result list, so one double serves every
// route: each route reads the keys its own upstream would send.
type routeMediaCaller struct{}

func (routeMediaCaller) Do(context.Context, dataplane.MediaRequest) (dataplane.MediaResponse, error) {
	return dataplane.MediaResponse{Status: 200, Body: []byte(
		`{"created":1,"data":[{"url":"https://img.example.com/a.png"}],` +
			`"results":[{"title":"Go","url":"https://go.dev"}]}`)}, nil
}

// mediaRouteIndex is the registry slice the route tests address: one provider
// that offers two media kinds and one that offers none.
func mediaRouteIndex() *mediaIndex {
	return &mediaIndex{entries: []registry.Provider{
		{
			ID: "openai", Priority: 1, Category: "api", Display: registry.Display{Name: "OpenAI"},
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.openai.com/v1/audio/speech", DefaultModel: "gpt-4o-mini-tts",
					Models: []registry.MediaModel{{ID: "gpt-4o-mini-tts", Name: "GPT-4o mini TTS"}},
					Voices: []registry.MediaVoice{{ID: "alloy", Name: "Alloy"}},
				},
				registry.MediaImage: {
					BaseURL: "https://api.openai.com/v1/images/generations", DefaultModel: "gpt-image-1",
				},
			},
		},
		{ID: "chatonly", Priority: 2, Display: registry.Display{Name: "Chat Only"}},
		{
			ID: "brave-search", Priority: 3, Category: "apikey", Display: registry.Display{Name: "Brave"},
			Media: registry.MediaConfigs{
				registry.MediaWebSearch: {
					BaseURL: "https://api.search.brave.com/res/v1", Method: "GET",
					QueryParam: "q", MaxResultsParam: "count", DefaultMaxResults: 5,
				},
			},
		},
	}}
}

// mediaIndex is a fixed ProviderIndex over the entries a test declares.
type mediaIndex struct {
	entries []registry.Provider
}

func (i *mediaIndex) Provider(name string) (registry.Provider, bool) {
	for _, entry := range i.entries {
		if entry.ID == name {
			return entry, true
		}
	}
	return registry.Provider{}, false
}

func (i *mediaIndex) All() []registry.Provider { return i.entries }

func (i *mediaIndex) Categories() []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, entry := range i.entries {
		if entry.Category != "" && !seen[entry.Category] {
			seen[entry.Category] = true
			out = append(out, entry.Category)
		}
	}
	sort.Strings(out)
	return out
}

// memMediaOverrideRepo implements repository.MediaOverrideRepository in memory.
type memMediaOverrideRepo struct {
	rows map[string]domain.MediaOverride
}

func newMemMediaOverrideRepo() *memMediaOverrideRepo {
	return &memMediaOverrideRepo{rows: map[string]domain.MediaOverride{}}
}

func (r *memMediaOverrideRepo) List(context.Context) ([]domain.MediaOverride, error) {
	out := make([]domain.MediaOverride, 0, len(r.rows))
	for _, row := range r.rows {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderID() != out[j].ProviderID() {
			return out[i].ProviderID() < out[j].ProviderID()
		}
		return out[i].Kind() < out[j].Kind()
	})
	return out, nil
}

func (r *memMediaOverrideRepo) Get(_ context.Context, providerID string, kind domain.MediaKind) (domain.MediaOverride, bool, error) {
	row, ok := r.rows[providerID+"\x00"+string(kind)]
	return row, ok, nil
}

func (r *memMediaOverrideRepo) Upsert(_ context.Context, override domain.MediaOverride) error {
	r.rows[override.ProviderID()+"\x00"+string(override.Kind())] = override
	return nil
}
