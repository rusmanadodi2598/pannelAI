// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_http_doubles_test.go
// @for       The narrow doubles the chat HTTP fixture wires behind the real
//
//	registry, selector, transport, and key lookup.
//
// @uses      internal/dataplane, internal/domain, internal/provider,
// internal/registry, internal/repository, internal/service, context,
// net/http, sync, testing, time.
// @reason    The fixture builder and the collaborators it stands on are separate
//
//	concerns: the builder describes the pipeline, the doubles describe
//	the narrow storage an HTTP test can afford. Keeping them apart holds
//	both files inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

type chatProviderRegistry struct{ provider registry.Provider }

func (r chatProviderRegistry) Provider(name string) (registry.Provider, bool) {
	if name == r.provider.ID {
		return r.provider, true
	}
	return registry.Provider{}, false
}

func (r chatProviderRegistry) Model(string, string) (registry.Model, bool) {
	return registry.Model{}, false
}

func (r chatProviderRegistry) All() []registry.Provider { return []registry.Provider{r.provider} }

type chatModelLookup struct{}

func (chatModelLookup) Combo(context.Context, string) (domain.Combo, bool, error) {
	return domain.Combo{}, false, nil
}
func (chatModelLookup) Alias(context.Context, string) (string, bool, error) { return "", false, nil }
func (chatModelLookup) Disabled(context.Context, string, string) (bool, error) {
	return false, nil
}
func (chatModelLookup) DisabledPairs(context.Context) ([]domain.ModelRef, error) { return nil, nil }
func (chatModelLookup) ComboNames(context.Context) ([]string, error)             { return nil, nil }

func newChatResolver(t *testing.T, upstreamURL string) *dataplane.Resolver {
	t.Helper()
	resolver, err := dataplane.NewResolver(chatProviderRegistry{provider: registry.Provider{
		ID: "test", AuthType: registry.AuthNone, NoAuth: true, PassthroughModels: true,
		Transport: registry.Transport{BaseURL: upstreamURL, Format: registry.DefaultFormat},
	}}, chatModelLookup{})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	return resolver
}

func newChatSelector(t *testing.T) *dataplane.Selector {
	t.Helper()
	repo := &chatEndpointRepository{endpoint: chatEndpoint(t)}
	selector, err := dataplane.NewSelector(dataplane.SelectorDeps{Endpoints: repo})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	return selector
}

func chatEndpoint(t *testing.T) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint("ep-playground", "test", "playground", domain.UpstreamAuthNone, 1, testInstant())
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	return endpoint
}

func newChatTransport(t *testing.T) *dataplane.Transport {
	t.Helper()
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := dataplane.NewTransport(dataplane.TransportDeps{Connectors: connectors, Client: http.DefaultClient})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	return transport
}

type chatEndpointRepository struct {
	mu       sync.Mutex
	endpoint domain.UpstreamEndpoint
}

func (r *chatEndpointRepository) List(_ context.Context, filter repository.EndpointFilter, _ repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if filter.ProviderID != "test" || filter.Status != string(domain.UpstreamEndpointActive) {
		return nil, 0, nil
	}
	return []domain.UpstreamEndpoint{r.endpoint}, 1, nil
}
func (r *chatEndpointRepository) RecordKeyHealth(context.Context, domain.UpstreamKey) error {
	return nil
}
func (r *chatEndpointRepository) Create(context.Context, domain.UpstreamEndpoint) error { return nil }
func (r *chatEndpointRepository) GetByID(context.Context, string) (domain.UpstreamEndpoint, error) {
	return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
}
func (r *chatEndpointRepository) Update(context.Context, domain.UpstreamEndpoint) error { return nil }
func (r *chatEndpointRepository) Delete(context.Context, string) error                  { return nil }
func (r *chatEndpointRepository) AddKey(context.Context, string, domain.UpstreamKey) error {
	return nil
}
func (r *chatEndpointRepository) UpdateKey(context.Context, domain.UpstreamKey) error { return nil }
func (r *chatEndpointRepository) DeleteKey(context.Context, string, string) error     { return nil }
func (r *chatEndpointRepository) Reorder(context.Context, string, []string) error     { return nil }

type chatKeyLookup struct{ key domain.GatewayKey }

func (l chatKeyLookup) GetByValueHash(_ context.Context, hash string) (domain.GatewayKey, error) {
	if hash == l.key.ValueHash() {
		return l.key, nil
	}
	return domain.GatewayKey{}, domain.ErrGatewayKeyNotFound
}

type chatAPIKeySettings struct{}

func (chatAPIKeySettings) RequireAPIKey(context.Context) (bool, error) { return true, nil }

func testInstant() time.Time {
	return time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
}

var _ service.GatewayKeyLookup = chatKeyLookup{}
var _ service.RequireAPIKeyReader = chatAPIKeySettings{}
var _ repository.EndpointRepository = (*chatEndpointRepository)(nil)

// Chat exposes the wired service so a router test can serve the same pipeline
// through the real mux rather than a second, weaker fixture.
func (f chatHTTPFixture) Chat() *service.ChatService { return f.chat }

// Key exposes the gateway key the fixture's service accepts.
func (f chatHTTPFixture) Key() string { return f.key }
