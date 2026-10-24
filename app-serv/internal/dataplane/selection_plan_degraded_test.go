// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_plan_degraded_test.go
// @for       What rotation does when its arbiter cannot answer: the selection stamp takes over and still spreads a burst.
// @uses      context, errors, testing, time, internal/domain.
// @reason    Redis is not a correctness input, so the degraded path is the one a deployment actually runs on during an outage, and it is the path most likely to be silently wrong: ordering by the last success would pile every request onto one credential exactly when the store that could have separated them is gone.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSelectionPlan_CursorFailureStillSpreads is the degraded path: without the
// arbiter the walk orders by the selection stamp, which the stamping write advances,
// so a burst spreads within one round instead of hammering one key forever.
func TestSelectionPlan_CursorFailureStillSpreads(t *testing.T) {
	repo, keyCount := fiveKeyRepo(t)
	keyCursor := &atomicKeyCursor{err: errors.New("redis unavailable")}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, KeyCursor: keyCursor,
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 1}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	selector.clock = func() time.Time { return now }

	distinct := map[string]bool{}
	for range keyCount {
		selection, err := selector.SelectNext(context.Background(), "alpha", nil)
		if err != nil {
			t.Fatalf("SelectNext() round errored: %v", err)
		}
		distinct[selection.Key.ID()] = true
	}
	if len(distinct) != keyCount {
		t.Fatalf("degraded walk chose %d distinct keys over %d rounds (%v), want the whole set",
			keyCount, len(distinct), distinct)
	}
}

// TestSelectionPlan_NoCursorSeamDegradesToStampedOrder pins the optional seam: an
// unwired deployment serves, in stamp order, and reads no rotation state at all.
func TestSelectionPlan_NoCursorSeamDegradesToStampedOrder(t *testing.T) {
	repo, keyCount := fiveKeyRepo(t)
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{},
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 1}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	selector.clock = func() time.Time { return now }

	distinct := map[string]bool{}
	for range keyCount {
		selection, err := selector.SelectNext(context.Background(), "alpha", nil)
		if err != nil {
			t.Fatalf("SelectNext() errored with no cursor wired: %v", err)
		}
		distinct[selection.Key.ID()] = true
	}
	if len(distinct) != keyCount {
		t.Fatalf("no-cursor walk chose %d distinct keys, want the whole set", len(distinct))
	}
}
