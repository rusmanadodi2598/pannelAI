// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_fake_test.go
// @for       The fake provider index, prober, and the fixtures the endpoint
//
//	service's table-driven tests share.
//
// @uses      context, testing, time, internal/domain, internal/registry.
// @reason    AGENTS.md §2.1 requires the service logic to be tested, and these
//
//	doubles are what make that possible without a registry binary, a
//	sealer key from the environment, or an HTTP upstream. They live beside
//	the store double but in their own file because AGENTS.md §1.1 caps a
//	file at 250 lines.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// fakeIndex resolves only the provider ids a test declares, so the unknown-provider
// path is reachable without the embedded registry.
type fakeIndex struct {
	known map[string]registry.Provider
}

func newFakeIndex(ids ...string) *fakeIndex {
	known := make(map[string]registry.Provider, len(ids))
	for _, id := range ids {
		known[id] = registry.Provider{ID: id, Transport: registry.Transport{Format: "openai"}}
	}
	return &fakeIndex{known: known}
}

func (f *fakeIndex) Provider(name string) (registry.Provider, bool) {
	provider, ok := f.known[name]
	return provider, ok
}

// All flattens the declared set, so the provider list tests can drive the
// service without the embedded registry.
func (f *fakeIndex) All() []registry.Provider {
	out := make([]registry.Provider, 0, len(f.known))
	for _, entry := range f.known {
		out = append(out, entry)
	}
	return out
}

// Categories reports the distinct category set of the declared entries.
func (f *fakeIndex) Categories() []string {
	seen := map[string]struct{}{}
	for _, entry := range f.known {
		seen[entry.Category] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for category := range seen {
		out = append(out, category)
	}
	return out
}

// fakeProber reports a fixed outcome, so a test can drive either branch of the probe
// recording rule without an upstream.
type fakeProber struct {
	outcome ProbeOutcome
	err     error
	calls   int
}

func (p *fakeProber) ProbeEndpoint(context.Context, domain.UpstreamEndpoint, domain.UpstreamKey, string) (ProbeOutcome, error) {
	p.calls++
	return p.outcome, p.err
}

func (p *fakeProber) ProbeNode(context.Context, domain.ProviderNode, string) (ProbeOutcome, error) {
	p.calls++
	return p.outcome, p.err
}

// newTestEndpointService builds the service with the given doubles and a fixed clock.
func newTestEndpointService(t *testing.T, store EndpointStore, index ProviderIndex, sealer CredentialSealer, prober EndpointProber) *EndpointService {
	t.Helper()
	svc, err := NewEndpointService(EndpointServiceDeps{Store: store, Index: index, Sealer: sealer, Prober: prober})
	if err != nil {
		t.Fatalf("NewEndpointService() error = %v", err)
	}
	svc.clock = func() time.Time { return testNow }
	return svc
}

// seedEndpoint creates an endpoint through the service's own rules, so a test's
// starting state cannot diverge from what create would have produced.
func seedEndpoint(t *testing.T, svc *EndpointService, providerID, label string, authType domain.UpstreamAuthType, keys ...KeyInput) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := svc.Create(context.Background(), CreateInput{
		ProviderID: providerID, Label: label, AuthType: authType, Keys: keys,
	})
	if err != nil {
		t.Fatalf("seeding endpoint %q: %v", label, err)
	}
	return endpoint
}

// mustAppError asserts an error carries the expected §8 code.
func mustAppError(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", code)
	}
	if got := domain.AsAppError(err).Code; got != code {
		t.Fatalf("code = %q, want %q (error: %v)", got, code, err)
	}
}

// describe formats a probe outcome for a test's failure message.
func describe(outcome ProbeOutcome) string {
	return fmt.Sprintf("state=%s latency=%d status=%d message=%q",
		outcome.State, outcome.LatencyMS, outcome.Status, outcome.Message)
}
