// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_test.go
// @for       Table-driven tests for the endpoint and key selection order.
// @uses      context, testing, time, internal/domain
// @reason    SPEC-API-001 §7.5 fixes the selection rule: endpoints are tried in priority order, a
//
//	disabled endpoint is skipped, and a tripped key is skipped in favour of a healthy one.
//	It decides whether a request is served and which account pays for it, so every branch
//	is pinned here against the in-memory double (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSelector_Ordering pins SPEC-API-001 §7.5: endpoints are tried in priority
// order, a disabled endpoint is skipped, and a tripped key is skipped in favour of
// the next healthy one.
func TestSelector_Ordering(t *testing.T) {
	cases := []struct {
		name         string
		endpoints    []domain.UpstreamEndpoint
		wantEndpoint string
		wantKey      string
		wantErr      bool
	}{
		{
			name: "the highest-priority endpoint answers",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_b", priority: 1}}),
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_a",
		},
		{
			name: "a disabled endpoint is skipped for the next one",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointDisabled, []keyFixture{{id: "uky_a", priority: 1}}),
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_b", priority: 1}}),
			},
			wantEndpoint: "ep_b", wantKey: "uky_b",
		},
		{
			name: "the highest-priority healthy key inside the endpoint answers",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_second", priority: 2},
					{id: "uky_first", priority: 1},
				}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_first",
		},
		{
			name: "a key with a disabled status is skipped",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_off", priority: 1, status: domain.UpstreamKeyDisabled},
					{id: "uky_on", priority: 2},
				}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_on",
		},
		{
			name: "a tripped key is skipped for the next one",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_tripped", priority: 1, failures: domain.CircuitThreshold()},
					{id: "uky_healthy", priority: 2},
				}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_healthy",
		},
		{
			name: "a tripped key in the first endpoint fails over to the second",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_tripped", priority: 1, failures: domain.CircuitThreshold()},
				}),
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_b", priority: 1}}),
			},
			wantEndpoint: "ep_b", wantKey: "uky_b",
		},
		{
			name: "an endpoint below the circuit threshold is still used",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_warm", priority: 1, failures: domain.CircuitThreshold() - 1},
				}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_warm",
		},
		{
			name: "every endpoint unavailable yields NO_PROVIDER_AVAILABLE",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointDisabled, []keyFixture{{id: "uky_a", priority: 1}}),
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointDisabled, []keyFixture{{id: "uky_b", priority: 1}}),
			},
			wantErr: true,
		},
		{
			name: "an endpoint whose every key is tripped yields NO_PROVIDER_AVAILABLE",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_1", priority: 1, failures: domain.CircuitThreshold()},
					{id: "uky_2", priority: 2, failures: domain.CircuitThreshold()},
				}),
			},
			wantErr: true,
		},
		{
			name:    "a provider with no endpoint yields NO_PROVIDER_AVAILABLE",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = tc.endpoints

			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selector.clock = func() time.Time { return now }

			got, err := selector.Select(context.Background(), "provider-a")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want NO_PROVIDER_AVAILABLE (got %s/%s)", got.Endpoint.ID(), got.Key.ID())
				}
				if code := domain.AsAppError(err).Code; code != "NO_PROVIDER_AVAILABLE" {
					t.Fatalf("code = %q, want NO_PROVIDER_AVAILABLE", code)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want none", err)
			}
			if got.Endpoint.ID() != tc.wantEndpoint {
				t.Fatalf("endpoint = %q, want %q", got.Endpoint.ID(), tc.wantEndpoint)
			}
			if got.Key.ID() != tc.wantKey {
				t.Fatalf("key = %q, want %q", got.Key.ID(), tc.wantKey)
			}
			if got.Credential.APIKey == "" {
				t.Fatal("Credential.APIKey is empty, want the opened plaintext")
			}
			if got.Credential.KeyID != tc.wantKey {
				t.Fatalf("credential key id = %q, want %q", got.Credential.KeyID, tc.wantKey)
			}
		})
	}
}
