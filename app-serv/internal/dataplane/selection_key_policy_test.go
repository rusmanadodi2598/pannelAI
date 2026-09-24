// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_key_policy_test.go
// @for       The key rule inside one endpoint under each credential rotation
//
//	policy: priority order for fill-first, least-recently-used for
//	round-robin.
//
// @uses      context, testing, time, internal/domain.
// @reason    SPEC-API-001 §7.5 fixes both walks. The fixture holds two healthy
//
//	keys whose priority order and recency order disagree, so each case
//	proves its rule instead of reading whichever key happens to come
//	first.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// twoKeyEndpoint builds one provider whose single endpoint holds two healthy
// keys that disagree on order: uky-1 has the better priority and was used most
// recently, uky-2 the worse priority and never used.
func twoKeyEndpoint(t *testing.T) *memEndpointRepo {
	t.Helper()
	repo := newMemEndpointRepo()
	endpoint := buildEndpoint(t, "ep-1", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky-2", priority: 2}})
	recent := now
	endpoint.AttachKey(domain.RehydrateUpstreamKey(
		"uky-1", "ep-1", "key-uky-1", "v1:nonce:cipher", "abc...wxyz",
		1, domain.UpstreamKeyActive, &recent, "", 0, nil, now, now,
	))
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{endpoint}
	return repo
}

// TestSelector_FillFirstPicksTheFirstKeyByPriority pins the key rule inside one
// endpoint: priority order, even when the other healthy key is the idler one.
func TestSelector_FillFirstPicksTheFirstKeyByPriority(t *testing.T) {
	selector, err := NewSelector(SelectorDeps{
		Endpoints: twoKeyEndpoint(t), Opener: opener{},
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationFillFirst, StickyLimit: 3}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	selection, err := selector.Select(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selection.Key.ID() != "uky-1" {
		t.Fatalf("key = %s, want uky-1: fill-first takes the first key by priority", selection.Key.ID())
	}
}

// TestSelector_RoundRobinPicksTheIdleKey is the same fixture under the other
// mode: the least-recently-used key wins, so an idle second key is not dead
// weight.
func TestSelector_RoundRobinPicksTheIdleKey(t *testing.T) {
	selector, err := NewSelector(SelectorDeps{
		Endpoints: twoKeyEndpoint(t), Opener: opener{},
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 3}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	selection, err := selector.Select(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selection.Key.ID() != "uky-2" {
		t.Fatalf("key = %s, want uky-2: round-robin takes the least-recently-used key", selection.Key.ID())
	}
}
