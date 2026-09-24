// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_opencode_free_engine_test.go
// @for       The engine wiring the free-tier tests drive: the real pipeline over
//
//	the measured upstream stand-in.
//
// @uses      testing, internal/domain, internal/provider, internal/registry.
// @reason    The wiring is the evidence that the lane works end to end: the
//
//	resolver, selector, transport, and connector are the production ones,
//	and only the upstream address is the stand-in. Keeping it in its own
//	file also keeps the fixture and acceptance files inside the AGENTS.md
//	section 1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// openCodeFreeIndex is a ProviderRegistry over the real OpenCode entry with its
// base URL pointed at the test server, so the real declared model list (and its
// per-model target format) is what the resolver reads.
type openCodeFreeIndex struct{ entry registry.Provider }

func (i openCodeFreeIndex) Provider(name string) (registry.Provider, bool) {
	if name == i.entry.ID {
		return i.entry, true
	}
	return registry.Provider{}, false
}

func (i openCodeFreeIndex) Model(providerID, modelID string) (registry.Model, bool) {
	if providerID != i.entry.ID {
		return registry.Model{}, false
	}
	for _, model := range i.entry.Models {
		if model.ID == modelID {
			return model, true
		}
	}
	return registry.Model{}, false
}

func (i openCodeFreeIndex) All() []registry.Provider { return []registry.Provider{i.entry} }

// newOpenCodeFreeEngine wires the real pipeline over the free-tier stand-in: the
// resolver, selector, transport, and the OpenCode connector are the production
// ones, and the endpoint carries no credential because the lane needs none.
func newOpenCodeFreeEngine(t *testing.T, upstreamURL string, seen *[]openCodeFreeCall) *Engine {
	t.Helper()
	return newOpenCodeFreeEngineWith(t, upstreamURL, seen, true)
}

// newOpenCodeFreeEngineWith is the same wiring with the endpoint row optional, so
// a test can prove the virtual-endpoint rule reaches the whole pipeline rather
// than only the selector (draft 029 F8). A run with no row is what an operator
// gets on a fresh install, before anything is configured.
func newOpenCodeFreeEngineWith(t *testing.T, upstreamURL string, seen *[]openCodeFreeCall, withEndpoint bool) *Engine {
	t.Helper()
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("loading the embedded registry: %v", err)
	}
	entry, ok := index.Provider("opencode")
	if !ok {
		t.Fatal("the opencode entry is missing from the embedded registry")
	}
	entry.Transport.BaseURL = upstreamURL

	resolver, err := NewResolver(openCodeFreeIndex{entry: entry}, fakeLookup{})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, provider.NewOpenCode(entry))
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	endpoint, err := domain.NewUpstreamEndpoint("ep_free", "opencode", "Public", domain.UpstreamAuthNone, 1, now)
	if err != nil {
		t.Fatalf("building the keyless endpoint: %v", err)
	}
	repo := newMemEndpointRepo()
	if withEndpoint {
		repo.byProvider["opencode"] = []domain.UpstreamEndpoint{endpoint}
	}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo,
		Opener:    opener{},
		// The registry is what makes the virtual endpoint reachable, and the
		// fixture passes the real entry so the rule reads the same no_auth
		// declaration the server does.
		Registry: openCodeFreeIndex{entry: entry},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: http.DefaultClient})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	engine, err := NewEngine(EngineDeps{Resolver: resolver, Selector: selector, Transport: transport})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return engine
}
