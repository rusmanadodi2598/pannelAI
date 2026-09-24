// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_relay_build_test.go
// @for       The engine builders every relay test shares.
//
// @uses      net/http, testing, internal/domain, internal/provider, internal/registry.
// @reason    Three builders differ only in which optional seams they pass, and
//
//	every relay test needs one of them. Keeping them together means a new
//	test picks a builder rather than re-deriving the pipeline, which is
//	what keeps an engine test about the engine.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package dataplane

import (
	"context"
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// newRelayEngine wires the full pipeline over the doubles above: the resolver
// and selector are real, only storage and the wire are faked. An optional
// vision augmenter is the §7.8 seam, so a test can pass one and the pipeline
// behaves exactly as a wired adapter would make it.
func newRelayEngine(t *testing.T, upstreamURL string, repo *memEndpointRepo, combos map[string]domain.Combo, vision ...VisionAugmenter) *Engine {
	t.Helper()
	return newEngineWith(t, []registry.Provider{
		relayProvider("alpha", upstreamURL), relayProvider("beta", upstreamURL),
	}, repo, combos, nil, vision...)
}

// newEngineWith is the same wiring over an explicit provider list and an
// optional rotation store, so a test that needs a third provider (the fusion
// judge) or a round-robin combo does not re-implement it.
func newEngineWith(t *testing.T, providers []registry.Provider, repo *memEndpointRepo, combos map[string]domain.Combo, orders ComboOrderer, vision ...VisionAugmenter) *Engine {
	t.Helper()
	return newEngineFull(t, providers, repo, combos, orders, nil, vision...)
}

// newEngineFull is the one wiring every engine test builds on: the resolver and
// selector are real, and the optional seams are passed through.
func newEngineFull(t *testing.T, providers []registry.Provider, repo *memEndpointRepo, combos map[string]domain.Combo, orders ComboOrderer, active ActiveRequests, vision ...VisionAugmenter) *Engine {
	t.Helper()
	return newEngineSeams(t, providers, repo, combos, orders, active, nil, vision...)
}

// newEngineSeams is newEngineFull plus the credential rotation seam, so a test
// that measures rotation passes a policy explicitly instead of relying on a
// default it does not control.
func newEngineSeams(t *testing.T, providers []registry.Provider, repo *memEndpointRepo, combos map[string]domain.Combo, orders ComboOrderer, active ActiveRequests, strategies CredentialStrategy, vision ...VisionAugmenter) *Engine {
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
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, Strategies: strategies})
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
		Resolver: resolver, Selector: selector, Transport: transport,
		Vision: augmenter, ComboOrder: orders, ActiveRequests: active,
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return engine
}

// stubStrategy answers one rotation policy for every provider, which is what a
// test asks for when it measures the walk rather than the settings document.
type stubStrategy struct {
	policy domain.RotationPolicy
	err    error
}

// RotationPolicy satisfies the selector's seam.
func (s stubStrategy) RotationPolicy(context.Context, string) (domain.RotationPolicy, error) {
	return s.policy, s.err
}

// rotatingEngine is newRelayEngine with a round-robin policy, so probe 5's
// rotation is asked for explicitly: the unwired default is fill-first, which
// serves one endpoint forever and would make the probe vacuous.
func rotatingEngine(t *testing.T, upstreamURL string, repo *memEndpointRepo, sticky int) *Engine {
	t.Helper()
	return newEngineSeams(t,
		[]registry.Provider{relayProvider("alpha", upstreamURL), relayProvider("beta", upstreamURL)},
		repo, nil, nil, nil,
		stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: sticky}},
	)
}
