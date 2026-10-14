// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_stamp_test.go
// @for       Tests for the selection stamp the credential walk leaves behind: one per chosen key, none for a pinned account, and a lost stamp that costs the request nothing.
// @uses      context, errors, strings, testing, internal/domain.
// @reason    The stamp is the fact that makes rotation order by choice rather than by luck, so a walk that chose a key without recording it would hand the same credential to every concurrent request. A pin must not advance it, and a bookkeeping write must never become the reason a caller loses its answer.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stampSelector wires one provider whose endpoint holds two idle keys under
// round-robin, so the walk has a key to choose and a reason to remember it.
func stampSelector(t *testing.T, repo *memEndpointRepo) *Selector {
	t.Helper()
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{},
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 1}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	return selector
}

// twoIdleKeyRepo builds one endpoint whose keys are both unused, so the first pick
// is decided by the tiebreak alone and nothing else can explain the second.
func twoIdleKeyRepo(t *testing.T) *memEndpointRepo {
	t.Helper()
	repo := newMemEndpointRepo()
	endpoint := buildEndpoint(t, "ep-1", 1, domain.UpstreamEndpointActive,
		[]keyFixture{{id: "uky-1", priority: 1}, {id: "uky-2", priority: 1}})
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{endpoint}
	return repo
}

func TestSelector_SelectNextStampsTheChosenCredential(t *testing.T) {
	repo := twoIdleKeyRepo(t)
	selector := stampSelector(t, repo)

	selection, err := selector.SelectNext(context.Background(), "alpha", nil)
	if err != nil {
		t.Fatalf("SelectNext() error = %v", err)
	}
	if repo.stampsSelected != 1 {
		t.Fatalf("selection stamps = %d, want 1", repo.stampsSelected)
	}
	if _, ok := repo.selected[selection.Endpoint.ID()+"/"+selection.Key.ID()]; !ok {
		t.Fatalf("stamp = %v, want the key the walk handed out (%s)",
			repo.selected, selection.Key.ID())
	}
}

// TestSelector_SelectNextWalksDistinctKeysUnderNoHistory is the burst the stamp
// exists to break: with no prior history and no Redis cursor, a success-only stamp
// answers the same key forever, because nothing about that key ever changes.
func TestSelector_SelectNextWalksDistinctKeysUnderNoHistory(t *testing.T) {
	repo := twoIdleKeyRepo(t)
	selector := stampSelector(t, repo)

	seen := map[string]bool{}
	for range 4 {
		selection, err := selector.Select(context.Background(), "alpha")
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		seen[selection.Key.ID()] = true
	}
	if len(seen) != 2 {
		t.Fatalf("distinct keys chosen across four selections = %d, want 2: %v", len(seen), seen)
	}
}

func TestSelector_PinnedSelectionAdvancesNoRotation(t *testing.T) {
	repo := twoIdleKeyRepo(t)
	selector := stampSelector(t, repo)

	if _, err := selector.SelectEndpoint(context.Background(), "alpha", "ep-1"); err != nil {
		t.Fatalf("SelectEndpoint() error = %v", err)
	}
	// A caller that named an account has already chosen; moving the rotation state
	// on would change which account the unpinned routes reach next.
	if repo.stampsSelected != 0 {
		t.Fatalf("selection stamps = %d, want 0: a pin leaves rotation alone", repo.stampsSelected)
	}
}

func TestSelector_LostStampStillServesTheRequest(t *testing.T) {
	repo := twoIdleKeyRepo(t)
	repo.selectErr = errors.New("database unavailable")
	selector := stampSelector(t, repo)

	selection, err := selector.SelectNext(context.Background(), "alpha", nil)
	if err != nil {
		t.Fatalf("SelectNext() error = %v, want the request served regardless of the stamp", err)
	}
	if selection.Key.ID() == "" {
		t.Fatal("SelectNext() returned no credential")
	}
	if repo.stampsSelected != 1 {
		t.Fatalf("stamp attempts = %d, want the write to have been tried", repo.stampsSelected)
	}
}

// TestSelector_VirtualEndpointStampsNothing pins the free lane's exception: a
// credential-free provider with no stored row is served on a SYNTHESIZED account, so
// there is no row to stamp and every request would pay a primary-key update that
// cannot affect anything.
func TestSelector_VirtualEndpointStampsNothing(t *testing.T) {
	repo := newMemEndpointRepo()
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo,
		Registry:  virtualRegistry{"free-a": {ID: "free-a", Category: "free", NoAuth: true}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	selection, err := selector.Select(context.Background(), "free-a")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if !strings.HasPrefix(selection.Endpoint.ID(), VirtualEndpointIDPrefix) {
		t.Fatalf("endpoint = %q, want a synthesized one", selection.Endpoint.ID())
	}
	if repo.stampsSelected != 0 {
		t.Fatalf("selection stamps = %d, want 0 for an account that is not stored", repo.stampsSelected)
	}
}
