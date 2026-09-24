//go:build integration

// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/systemone_live_test.go
// @for       The live decision-route run against the real OpenCode upstream.
// @uses      context, encoding/json, testing, time, internal/dataplane,
//
//	internal/domain, internal/provider, internal/registry, internal/schema.
//
// @reason    The hermetic tests pin the route's shape against doubles. This file
//
//	is the other half of the evidence R-35 asks for: the same route against
//	the real upstream, so "the decision model answers" is a measurement
//	rather than an inference. It carries the `integration` build tag because
//	it spends the upstream's anonymous quota and needs the network.
//
//	  go test -tags=integration -run TestSystemOneLive ./internal/service/
//
//	It drives the real resolver over the embedded registry, the real HTTP
//	caller, and the real entry block, and substitutes only the account
//	selection (one keyless endpoint), because the free lane has no stored
//	account to select.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// liveResolver resolves against the real embedded registry, so the model's own
// declared kind and the entry's own decision block are what the route reads.
type liveResolver struct{ index *registry.Index }

func (r liveResolver) Resolve(ctx context.Context, model string) (dataplane.Resolution, error) {
	resolver, err := dataplane.NewResolver(liveIndex(r), liveLookup{})
	if err != nil {
		return dataplane.Resolution{}, err
	}
	return resolver.Resolve(ctx, model)
}

// ResolveForSystemOne asks the same resolver the decision route's question, so
// the live run exercises the real kind guard.
func (r liveResolver) ResolveForSystemOne(ctx context.Context, model string) (dataplane.Resolution, error) {
	resolver, err := dataplane.NewResolver(liveIndex(r), liveLookup{})
	if err != nil {
		return dataplane.Resolution{}, err
	}
	return resolver.ResolveForSystemOne(ctx, model)
}

// liveIndex is a ProviderRegistry over the real embedded registry.
type liveIndex struct{ index *registry.Index }

func (i liveIndex) Provider(name string) (registry.Provider, bool) { return i.index.Provider(name) }
func (i liveIndex) Model(providerID, modelID string) (registry.Model, bool) {
	return i.index.Model(providerID, modelID)
}
func (i liveIndex) All() []registry.Provider { return i.index.All() }

// liveLookup answers the catalog questions with nothing, so resolution takes the
// declared-model path rather than the combo or alias one.
type liveLookup struct{}

func (liveLookup) Combo(context.Context, string) (domain.Combo, bool, error) {
	return domain.Combo{}, false, nil
}
func (liveLookup) Alias(context.Context, string) (string, bool, error) { return "", false, nil }
func (liveLookup) Disabled(context.Context, string, string) (bool, error) {
	return false, nil
}
func (liveLookup) DisabledPairs(context.Context) ([]domain.ModelRef, error) { return nil, nil }
func (liveLookup) ComboNames(context.Context) ([]string, error)             { return nil, nil }

// liveKeylessRouter hands out one keyless endpoint, which is what the free lane
// has: no stored account, no key, and health writes that do nothing.
type liveKeylessRouter struct {
	endpoint   domain.UpstreamEndpoint
	credential provider.Credential
}

func (r *liveKeylessRouter) Select(context.Context, string) (dataplane.Selection, error) {
	return dataplane.Selection{Endpoint: r.endpoint, Credential: r.credential}, nil
}
func (r *liveKeylessRouter) RecordSuccess(context.Context, dataplane.Selection) error { return nil }
func (r *liveKeylessRouter) RecordFailure(context.Context, dataplane.Selection, string, domain.KeyFailureClass) error {
	return nil
}

// TestSystemOneLive_DecisionModelAnswersFromTheRealUpstream is the live half of
// the F6 acceptance: the decision route reaches the real upstream with no
// credential configured and the answer carries the decision that was asked for.
func TestSystemOneLive_DecisionModelAnswersFromTheRealUpstream(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("loading the embedded registry: %v", err)
	}
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_free", "opencode", "Public", domain.UpstreamAuthNone, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("building the keyless endpoint: %v", err)
	}
	svc, err := NewSystemOneService(SystemOneServiceDeps{
		Resolver: liveResolver{index: index},
		Router:   &liveKeylessRouter{endpoint: endpoint, credential: provider.NoCredential("ep_free")},
		Caller:   dataplane.NewMediaTransport(nil),
	})
	if err != nil {
		t.Fatalf("NewSystemOneService() error = %v", err)
	}

	raw := []byte(`{"model":"opencode/jev-1.13-free","state":"Customer: I was charged twice for my order this morning.",` +
		`"questions":{"probe":{"type":"noul","instructions":"Is the customer reporting a billing problem?"}}}`)
	req, err := schema.DecodeSystemOneRequest(raw)
	if err != nil {
		t.Fatalf("decoding the decision body: %v", err)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("validating the decision body: %v", err)
	}

	answer, err := svc.Decide(context.Background(), req, "key-live")
	if err != nil {
		t.Fatalf("Decide() error = %v, want the live decision model to answer", err)
	}
	var parsed struct {
		Model   string                     `json:"model"`
		Answers map[string]json.RawMessage `json:"answers"`
	}
	if err := json.Unmarshal(answer, &parsed); err != nil {
		t.Fatalf("decoding the live answer: %v", err)
	}
	if len(parsed.Answers) == 0 {
		t.Fatalf("the live answer carries no decisions: %s", answer)
	}
	if _, ok := parsed.Answers["probe"]; !ok {
		t.Fatalf("the live answer is missing the question asked: %s", answer)
	}
}
