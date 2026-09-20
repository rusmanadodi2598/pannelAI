// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_budget_test.go
// @for       Table-driven tests for the §7.12 budget gate: an endpoint whose
//
//	stored cap is spent is skipped, and a gate that cannot answer
//	does not take the data plane down.
//
// @uses      context, errors, testing, time, internal/domain.
// @reason    F1 of docs/DRAFT/005-QUOTA-TRACKER-CLOSURE.md found the cap rule
//
//	implemented and unreachable: domain.QuotaCap.Exhausted had no
//	production caller, so the spec's "router stops picking exhausted
//	endpoints" (§7.12) held in name only. These tests pin the two
//	directions that matter: a spent cap is skipped even when it is the
//	highest-priority endpoint, and a failing gate read fails open rather
//	than locking every provider out.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package dataplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubBudgetGate answers the budget question from a fixed set of endpoint ids,
// so a test states which endpoints are spent rather than staging a cap row.
type stubBudgetGate struct {
	exhausted map[string]bool
	err       error
	asked     []string
}

func (g *stubBudgetGate) Exhausted(_ context.Context, endpointID string) (bool, error) {
	g.asked = append(g.asked, endpointID)
	if g.err != nil {
		return false, g.err
	}
	return g.exhausted[endpointID], nil
}

// TestSelector_SkipsAnEndpointThatSpentItsBudget is the F1 enforcement case:
// the spent endpoint is the higher priority one, so it would win selection if
// the gate were not consulted.
func TestSelector_SkipsAnEndpointThatSpentItsBudget(t *testing.T) {
	cases := []struct {
		name         string
		exhausted    map[string]bool
		wantEndpoint string
		wantErr      bool
	}{
		{
			name:         "an uncapped gateway selects on priority alone",
			exhausted:    map[string]bool{},
			wantEndpoint: "ep_prio1",
		},
		{
			name:         "the top-priority endpoint having spent its budget hands selection to the next",
			exhausted:    map[string]bool{"ep_prio1": true},
			wantEndpoint: "ep_prio2",
		},
		{
			name:      "every endpoint having spent its budget is NO_PROVIDER_AVAILABLE",
			exhausted: map[string]bool{"ep_prio1": true, "ep_prio2": true},
			wantErr:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_prio1", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_1", priority: 1}}),
				buildEndpoint(t, "ep_prio2", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_2", priority: 1}}),
			}
			gate := &stubBudgetGate{exhausted: tc.exhausted}

			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, Gate: gate})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selector.clock = func() time.Time { return now }

			got, err := selector.Select(context.Background(), "provider-a")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want NO_PROVIDER_AVAILABLE (got %s)", got.Endpoint.ID())
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
		})
	}
}

// TestSelector_AFailingBudgetGateFailsOpen pins the stated direction: a quota
// read that errors must not lock every provider out. Failing closed here would
// turn a transient control-plane fault into a gateway-wide outage.
func TestSelector_AFailingBudgetGateFailsOpen(t *testing.T) {
	cases := []struct {
		name string
		gate func() BudgetGate
	}{
		{"a gate read that errors", func() BudgetGate {
			return &stubBudgetGate{err: errors.New("quota table is unreachable")}
		}},
		{"no gate wired at all", func() BudgetGate { return nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
			}
			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, Gate: tc.gate()})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selector.clock = func() time.Time { return now }

			got, err := selector.Select(context.Background(), "provider-a")
			if err != nil {
				t.Fatalf("err = %v, want the request served despite the gate", err)
			}
			if got.Endpoint.ID() != "ep_a" {
				t.Fatalf("endpoint = %q, want ep_a", got.Endpoint.ID())
			}
		})
	}
}

// TestSelector_AsksTheGateOncePerCandidate pins that the gate is consulted for
// the endpoints it walks, so a high-priority spent endpoint does not stop the
// walk before the next candidate is considered.
func TestSelector_AsksTheGateOncePerCandidate(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep_prio1", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_1", priority: 1}}),
		buildEndpoint(t, "ep_prio2", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_2", priority: 1}}),
	}
	gate := &stubBudgetGate{exhausted: map[string]bool{"ep_prio1": true}}

	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, Gate: gate})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	if _, err := selector.Select(context.Background(), "provider-a"); err != nil {
		t.Fatalf("err = %v, want none", err)
	}
	if len(gate.asked) != 2 {
		t.Fatalf("gate asked about %v, want both candidates in priority order", gate.asked)
	}
	if gate.asked[0] != "ep_prio1" || gate.asked[1] != "ep_prio2" {
		t.Fatalf("gate asked %v, want [ep_prio1 ep_prio2]", gate.asked)
	}
}
