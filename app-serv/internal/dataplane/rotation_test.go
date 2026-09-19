// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/rotation_test.go
// @for       The round_robin strategy end to end: the rotated leader serves, and
//
//	an order the seam cannot answer falls back to priority order.
//
// @uses      testing, context, errors, sync, internal/domain.
// @reason    SPEC-API-001 §7.7 makes round_robin a distribution rule and §9.6
//
//	puts its state in Redis, so the two halves have to be proven
//	together: the engine must ask for the order with the combo's own
//	sticky limit, serve the leader the store returned, and still serve
//	the request when the store is down.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// fakeOrderer is a ComboOrderer double: one configured order, the last combo it
// was asked about, and an optional failure so a test can drive the fall-back.
type fakeOrderer struct {
	mu    sync.Mutex
	order []string
	err   error
	calls int
	combo domain.Combo
}

func (f *fakeOrderer) Order(_ context.Context, combo domain.Combo) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.combo = combo
	if f.err != nil {
		return nil, f.err
	}
	return f.order, nil
}

// rotationEngine wires an engine over the two-provider relay upstream, with the
// given combo and order seam. A nil double becomes a nil seam rather than a
// typed nil, which is what a deployment without Redis wires.
func rotationEngine(t *testing.T, upstreamURL string, combo domain.Combo, orders *fakeOrderer) (*Engine, *memEndpointRepo) {
	t.Helper()
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	var seam ComboOrderer
	if orders != nil {
		seam = orders
	}
	engine := newEngineWith(t, fusionProviders(upstreamURL, "alpha", "beta"), repo,
		map[string]domain.Combo{combo.Name(): combo}, seam)
	return engine, repo
}

// TestRelay_RoundRobinServesTheRotatedLeader pins the distribution: the order
// the seam returns is the order the request walks, and the seam is asked about
// the combo aggregate itself, so the strategy layer reads the combo's own name
// and sticky limit rather than a copy that could drift.
func TestRelay_RoundRobinServesTheRotatedLeader(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	orders := &fakeOrderer{order: []string{"beta/works", "alpha/broken"}}
	combo := roundRobinRow("daily", 3, "alpha/broken", "beta/works")
	engine, _ := rotationEngine(t, server.URL, combo, orders)

	outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if outcome.ProviderID != "beta" || outcome.Model != "works" {
		t.Fatalf("Outcome = %s/%s, want the rotated leader beta/works", outcome.ProviderID, outcome.Model)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1: the rotated leader served without failover", calls)
	}
	if orders.calls != 1 {
		t.Fatalf("order asks = %d, want 1", orders.calls)
	}
	if orders.combo.Name() != "daily" {
		t.Fatalf("order combo = %q, want the combo name daily", orders.combo.Name())
	}
	if orders.combo.StickyLimit() != 3 {
		t.Fatalf("order combo sticky limit = %d, want the combo's own 3", orders.combo.StickyLimit())
	}
	refs := orders.combo.Refs()
	if len(refs) != 2 || refs[0] != "alpha/broken" {
		t.Fatalf("order combo refs = %v, want the combo's stored priority order", refs)
	}
}

// TestRelay_RotationIsAnOptimisation pins the fall-back matrix: whenever the
// store cannot answer, or the strategy does not rotate, the request is served in
// the stored priority order instead of failing.
func TestRelay_RotationIsAnOptimisation(t *testing.T) {
	cases := []struct {
		name         string
		combo        domain.Combo
		orders       *fakeOrderer
		wantRotation int
		wantCalls    int
	}{
		{
			name:         "a rotation store that fails",
			combo:        roundRobinRow("daily", 1, "alpha/broken", "beta/works"),
			orders:       &fakeOrderer{err: errors.New("redis is down")},
			wantRotation: 1,
			wantCalls:    2,
		},
		{
			name:         "a store answering with an unusable order",
			combo:        roundRobinRow("daily", 1, "alpha/broken", "beta/works"),
			orders:       &fakeOrderer{order: []string{"beta/works"}},
			wantRotation: 1,
			wantCalls:    2,
		},
		{
			name:         "a deployment with no rotation store",
			combo:        roundRobinRow("daily", 1, "alpha/broken", "beta/works"),
			orders:       nil,
			wantRotation: 0,
			wantCalls:    2,
		},
		{
			name:         "a fallback combo, which does not rotate",
			combo:        comboRow("daily", "alpha/broken", "beta/works"),
			orders:       &fakeOrderer{order: []string{"beta/works", "alpha/broken"}},
			wantRotation: 0,
			wantCalls:    2,
		},
		{
			name:         "a combo with nothing to rotate",
			combo:        roundRobinRow("solo", 1, "beta/works"),
			orders:       &fakeOrderer{order: []string{"beta/works"}},
			wantRotation: 0,
			wantCalls:    1,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls int
			server := newRelayUpstream(t, &calls)
			engine, _ := rotationEngine(t, server.URL, testCase.combo, testCase.orders)

			outcome, err := engine.Relay(context.Background(), relayRequest(testCase.combo.Name()), nil)
			if err != nil {
				t.Fatalf("Relay() error = %v, want the request served in priority order", err)
			}
			if outcome.ProviderID != "beta" {
				t.Fatalf("Outcome.ProviderID = %q, want beta: priority order fails over from alpha", outcome.ProviderID)
			}
			if calls != testCase.wantCalls {
				t.Fatalf("upstream calls = %d, want %d: the stored order was walked from the top",
					calls, testCase.wantCalls)
			}
			if testCase.orders != nil && testCase.orders.calls != testCase.wantRotation {
				t.Fatalf("order asks = %d, want %d", testCase.orders.calls, testCase.wantRotation)
			}
		})
	}
}
