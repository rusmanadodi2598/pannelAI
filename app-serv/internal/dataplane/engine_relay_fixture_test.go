// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_relay_fixture_test.go
// @for       The end-to-end relay fixture: registry and lookup doubles, a real
//
//	HTTP upstream stand-in, and the wired engine the tests drive.
//
// @uses      testing, net/http, net/http/httptest, encoding/json, context,
//
//	internal/domain, internal/provider, internal/registry, internal/schema.
//
// @reason    The relay tests pin the pipeline as one piece, so their doubles are
//
//	shared wiring rather than per-test trivia. Keeping them in one
//	file keeps the three tests about the behaviour they pin and keeps
//	both files within the AGENTS.md section 1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package dataplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// relayRegistry is a ProviderRegistry with the providers a fallback test needs,
// all speaking the OpenAI wire format and passing model ids through.
type relayRegistry struct{ providers []registry.Provider }

func (r relayRegistry) Provider(name string) (registry.Provider, bool) {
	for _, entry := range r.providers {
		if entry.ID == name {
			return entry, true
		}
	}
	return registry.Provider{}, false
}

func (relayRegistry) Model(string, string) (registry.Model, bool) { return registry.Model{}, false }

func (r relayRegistry) All() []registry.Provider { return r.providers }

// relayLookup answers the combo question only; the other lookups are empty, so
// any unexpected dereference fails the request loudly rather than silently.
type relayLookup struct{ combos map[string]domain.Combo }

func (l relayLookup) Combo(_ context.Context, name string) (domain.Combo, bool, error) {
	combo, ok := l.combos[name]
	return combo, ok, nil
}

// comboRow builds a stored combo row for the fixtures: fallback, one sticky
// request per model, no judge — the shape every pre-fusion test used.
func comboRow(name string, refs ...string) domain.Combo {
	return comboWithStrategy(name, domain.ComboFallback, "", refs...)
}

// fusionRow builds a fusion combo row: the strategy under test, and the judge
// the panel's answers are synthesized by.
func fusionRow(name, judge string, refs ...string) domain.Combo {
	return comboWithStrategy(name, domain.ComboFusion, judge, refs...)
}

// comboWithStrategy rebuilds a combo the way the repository load path would:
// through Rehydrate, because the row already exists and the shape rules were
// enforced when it was written.
func comboWithStrategy(name string, strategy domain.ComboStrategy, judge string, refs ...string) domain.Combo {
	models := make([]domain.ComboModel, 0, len(refs))
	for index, ref := range refs {
		models = append(models, domain.RehydrateComboModel(ref, index))
	}
	return domain.RehydrateCombo("cmb_"+name, name, strategy, 1, judge, models, now, now)
}

func (relayLookup) Alias(context.Context, string) (string, bool, error) { return "", false, nil }
func (relayLookup) Disabled(context.Context, string, string) (bool, error) {
	return false, nil
}
func (relayLookup) DisabledPairs(context.Context) ([]domain.ModelRef, error) { return nil, nil }
func (relayLookup) ComboNames(context.Context) ([]string, error)             { return nil, nil }

// newRelayUpstream stands in for two upstreams on one host: every model is
// answered with a server failure except "works", which returns a complete
// OpenAI completion with usage.
func newRelayUpstream(t *testing.T, calls *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("upstream received an undecodable body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if body.Model != "works" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"the first member is down"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl-upstream","object":"chat.completion","created":1,` +
			`"model":"works","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},` +
			`"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// relayProvider is one OpenAI-format registry entry pointing at the test server.
func relayProvider(id, url string) registry.Provider {
	return registry.Provider{
		ID: id, Priority: 1, Category: "api", PassthroughModels: true,
		Transport: registry.Transport{BaseURL: url, Format: registry.DefaultFormat},
	}
}

// relayEndpoint builds one active api_key endpoint with a single key, so the
// selector has an account to spend per provider.
func relayEndpoint(t *testing.T, id, providerID string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, "label-"+id, domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint(%s) error = %v", id, err)
	}
	key := domain.RehydrateUpstreamKey(
		"uky-"+id, id, "key", "sealed-"+id, "…hint",
		1, domain.UpstreamKeyActive, nil, "", 0, nil, now, now,
	)
	endpoint.AttachKey(key)
	return endpoint
}

// newRelayEngine wires the full pipeline over the doubles above: the resolver
// and selector are real, only storage and the wire are faked. An optional
// vision augmenter is the §7.8 seam, so a test can pass one and the pipeline
// behaves exactly as a wired adapter would make it.
func newRelayEngine(t *testing.T, upstreamURL string, repo *memEndpointRepo, combos map[string]domain.Combo, vision ...VisionAugmenter) *Engine {
	t.Helper()
	return newEngineWith(t, []registry.Provider{
		relayProvider("alpha", upstreamURL), relayProvider("beta", upstreamURL),
	}, repo, combos, vision...)
}

// newEngineWith is the same wiring over an explicit provider list, so a test
// that needs a third provider — the fusion judge — does not re-implement it.
func newEngineWith(t *testing.T, providers []registry.Provider, repo *memEndpointRepo, combos map[string]domain.Combo, vision ...VisionAugmenter) *Engine {
	t.Helper()
	resolver, err := NewResolver(
		relayRegistry{providers: providers},
		relayLookup{combos: combos},
	)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, StickyLimit: 1})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: http.DefaultClient})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	var augmenter VisionAugmenter
	if len(vision) > 0 {
		augmenter = vision[0]
	}
	engine, err := NewEngine(EngineDeps{
		Resolver: resolver, Selector: selector, Transport: transport, Vision: augmenter,
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return engine
}

// visionAdapter is a VisionAugmenter double: one configured answer, recorded
// asks, and an optional failure so a test can drive the fail-open path.
type visionAdapter struct {
	calls      int
	refs       []string
	applies    bool
	err        error
	providerID string
	modelID    string
}

func (a *visionAdapter) Augment(_ context.Context, providerID, modelID string) ([]string, bool, error) {
	a.calls++
	a.providerID, a.modelID = providerID, modelID
	return a.refs, a.applies, a.err
}

// relayRequest is a client request naming the combo by bare model string.
func relayRequest(model string) Request {
	return Request{
		Route:        RouteChatCompletions,
		ClientFormat: schema.FormatOpenAI,
		Model:        model,
		Chat:         &schema.ChatRequest{Model: model},
		Raw:          []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"ping"}]}`),
	}
}
