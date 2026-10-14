// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_plan_test.go
// @for       Tests for one leg's snapshot: a burst spread by the key cursor, one cursor step per leg, and one provider read per leg however long the walk is.
// @uses      context, sync, testing, time, internal/domain.
// @reason    Credential rotation used to be derived from the last time a key succeeded, so N concurrent selections saw one identical set of timestamps and all chose the same key, while each failover attempt re-read the provider from PostgreSQL and re-rolled the account cursor. Both are invisible to a single-threaded test, so the proof has to be a burst.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// atomicKeyCursor stands in for the Redis counter: every call hands out the next
// number under a mutex, which is the one property the arbiter has. A fixed clock
// removes timestamp variance, so the only thing that can spread the picks is the
// cursor, which is exactly what is under test.
type atomicKeyCursor struct {
	mu    sync.Mutex
	next  int
	err   error
	calls int
	asked []string
}

func (c *atomicKeyCursor) Next(_ context.Context, endpointID string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.asked = append(c.asked, endpointID)
	if c.err != nil {
		return 0, c.err
	}
	c.next++
	return c.next, nil
}

// fiveKeyRepo builds one account holding five keys that agree on everything routing
// can see: same priority, no history. The tiebreak alone decides the first pick, so
// nothing but the cursor can separate the second from the first.
func fiveKeyRepo(t *testing.T) (*memEndpointRepo, int) {
	t.Helper()
	const keyCount = 5
	repo := newMemEndpointRepo()
	keys := make([]keyFixture, 0, keyCount)
	for i := range keyCount {
		keys = append(keys, keyFixture{id: string(rune('a' + i)), priority: 1})
	}
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep-1", 1, domain.UpstreamEndpointActive, keys),
	}
	return repo, keyCount
}

// TestSelectionPlan_BurstChoosesDistinctKeys is the defect the cursor exists for.
// Before it, every goroutine read one identical set of idle stamps and every pick
// landed on the same credential.
func TestSelectionPlan_BurstChoosesDistinctKeys(t *testing.T) {
	repo, keyCount := fiveKeyRepo(t)
	cursor := &atomicKeyCursor{}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, KeyCursor: cursor,
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 1}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	// Each goroutine runs its own leg, which is what N concurrent clients do.
	picks := make([]string, keyCount)
	var group sync.WaitGroup
	for i := range keyCount {
		group.Add(1)
		go func(slot int) {
			defer group.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("leg panicked: %v", r)
				}
			}()
			plan, err := selector.BeginPlan(context.Background(), "alpha")
			if err != nil {
				t.Errorf("BeginPlan() error = %v", err)
				return
			}
			selection, ok, err := plan.Next(context.Background(), nil)
			if err != nil {
				t.Errorf("BeginPlan() leg credential error = %v", err)
				return
			}
			if !ok {
				t.Errorf("leg %d found no credential", slot)
				return
			}
			picks[slot] = selection.Key.ID()
		}(i)
	}
	group.Wait()

	distinct := map[string]bool{}
	for _, id := range picks {
		if id == "" {
			t.Fatal("a leg returned no key")
		}
		distinct[id] = true
	}
	if len(distinct) != keyCount {
		t.Fatalf("distinct keys chosen by %d concurrent legs = %d (%v), want every key once",
			keyCount, len(distinct), picks)
	}
	if cursor.calls != keyCount {
		t.Fatalf("cursor reads = %d, want one per leg", cursor.calls)
	}
}

// TestSelectionPlan_WalkReadsNoStorePerAttempt pins the reload defect: a leg that
// fails over five times must still ask PostgreSQL for the provider once and roll
// the account cursor once.
func TestSelectionPlan_WalkReadsNoStorePerAttempt(t *testing.T) {
	repo, keyCount := fiveKeyRepo(t)
	cursor := &stubCursor{offset: 0}
	keyCursor := &atomicKeyCursor{}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, Cursor: cursor, KeyCursor: keyCursor,
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 1}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	plan, err := selector.BeginPlan(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("BeginPlan() error = %v", err)
	}
	spent := map[string]struct{}{}
	for range keyCount {
		selection, ok, err := plan.Next(context.Background(), spent)
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if !ok {
			t.Fatalf("the walk stopped with %d keys of %d spent", len(spent), keyCount)
		}
		spent[CandidateID(selection)] = struct{}{}
	}
	if len(spent) != keyCount {
		t.Fatalf("distinct credentials walked = %d, want the whole set", len(spent))
	}
	if cursor.calls != 0 {
		t.Fatalf("account cursor reads = %d, want 0: one account has nothing to rotate between, and a five-key walk must not ask anyway", cursor.calls)
	}
	if keyCursor.calls != 1 {
		t.Fatalf("key cursor reads = %d, want 1: one account, one position, however many keys it spends", keyCursor.calls)
	}
	if repo.lists != 1 {
		t.Fatalf("provider reads = %d, want 1 per leg, not one per attempt", repo.lists)
	}
}

// TestSelectionPlan_AccountCursorStepsOncePerLeg pins SPEC-API-001 §7.5's sentence
// that the walk used to contradict: the cursor advances once per request, not once
// per attempt. A leg that fails over between accounts may only move the provider on
// once, or the sticky limit means nothing and an account the request already spent
// comes back around on the wrap.
func TestSelectionPlan_AccountCursorStepsOncePerLeg(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep-1", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "a", priority: 1}}),
		buildEndpoint(t, "ep-2", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "b", priority: 1}}),
	}
	cursor := &stubCursor{offset: 1}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, Cursor: cursor,
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 1}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	plan, err := selector.BeginPlan(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("BeginPlan() error = %v", err)
	}
	spent := map[string]struct{}{}
	for range 2 {
		selection, ok, err := plan.Next(context.Background(), spent)
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if !ok {
			t.Fatal("the walk stopped before spending both accounts")
		}
		spent[CandidateID(selection)] = struct{}{}
	}
	if cursor.calls != 1 {
		t.Fatalf("account cursor reads = %d, want 1 for the whole leg", cursor.calls)
	}
	if repo.lists != 1 {
		t.Fatalf("provider reads = %d, want 1 for the whole leg", repo.lists)
	}
}

// TestSelectionPlan_EmptyProviderNamesTheMissingAccount pins that a provider with
// no rows is the same answer the walk gave before the plan existed: an unavailable
// provider, not an empty selection.
func TestSelectionPlan_EmptyProviderNamesTheMissingAccount(t *testing.T) {
	repo := newMemEndpointRepo()
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	if _, err := selector.SelectNext(context.Background(), "ghost", nil); err == nil {
		t.Fatal("SelectNext(ghost) = nil, want NO_PROVIDER_AVAILABLE")
	}
	plan, err := selector.BeginPlan(context.Background(), "ghost")
	if err != nil {
		t.Fatalf("BeginPlan(ghost) error = %v", err)
	}
	if _, ok, err := plan.Next(context.Background(), nil); ok || err != nil {
		t.Fatalf("Next() = %v/%v, want an empty provider to answer nothing and no error", ok, err)
	}
}
