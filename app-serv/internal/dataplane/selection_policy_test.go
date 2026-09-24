// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_policy_test.go
// @for       The credential rotation policy at the selector: fill-first versus
//
//	round-robin, the per-provider resolution, and the degradation of
//	an unreadable policy. The key rule inside one endpoint lives in
//	selection_key_policy_test.go.
//
// @uses      context, testing, internal/domain.
// @reason    SPEC-API-001 §7.5 fixes the walk the policy chooses. These cases
//
//	pin each branch without a database, including that fill-first
//	leaves the rotation cursor untouched: a mode that reads no state
//	must not write any either.
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

// stubCursor answers a scripted offset and records what it was asked, so a test
// can prove whether a mode consulted the rotation state and with which sticky
// limit.
type stubCursor struct {
	offset int
	calls  int
	sticky []int
	err    error
}

func (c *stubCursor) NextOffset(_ context.Context, _ string, _ int, stickyLimit int) (int, error) {
	c.calls++
	c.sticky = append(c.sticky, stickyLimit)
	if c.err != nil {
		return 0, c.err
	}
	return c.offset, nil
}

// mapStrategy answers a policy per provider id, so the per-provider override
// rule can be pinned at the selector level.
type mapStrategy map[string]domain.RotationPolicy

func (m mapStrategy) RotationPolicy(_ context.Context, providerID string) (domain.RotationPolicy, error) {
	return m[providerID], nil
}

// twoEndpointProvider seeds one provider with ep-1 (priority 1) and ep-2
// (priority 2), one key each.
func twoEndpointProvider(t *testing.T, providerID string) *memEndpointRepo {
	t.Helper()
	repo := newMemEndpointRepo()
	repo.byProvider[providerID] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep-1", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky-1", priority: 1}}),
		buildEndpoint(t, "ep-2", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky-2", priority: 1}}),
	}
	return repo
}

// TestSelector_FillFirstStartsAtTheFirstEndpoint pins the reference's default:
// priority order, and no rotation state behind a mode that reads none.
func TestSelector_FillFirstStartsAtTheFirstEndpoint(t *testing.T) {
	repo := twoEndpointProvider(t, "alpha")
	cursor := &stubCursor{offset: 1}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, Cursor: cursor,
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationFillFirst, StickyLimit: 3}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	selection, err := selector.Select(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selection.Endpoint.ID() != "ep-1" {
		t.Fatalf("endpoint = %s, want ep-1: fill-first serves priority order", selection.Endpoint.ID())
	}
	if cursor.calls != 0 {
		t.Fatalf("cursor calls = %d, want 0: fill-first reads no rotation state", cursor.calls)
	}
}

// TestSelector_RoundRobinFollowsTheCursorAndItsSticky pins the other mode: the
// cursor decides the start and the policy's sticky limit is what it is asked
// with, not a boot-time default.
func TestSelector_RoundRobinFollowsTheCursorAndItsSticky(t *testing.T) {
	repo := twoEndpointProvider(t, "alpha")
	cursor := &stubCursor{offset: 1}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, Cursor: cursor,
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 7}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	selection, err := selector.Select(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selection.Endpoint.ID() != "ep-2" {
		t.Fatalf("endpoint = %s, want ep-2: the cursor's offset starts the walk", selection.Endpoint.ID())
	}
	if len(cursor.sticky) != 1 || cursor.sticky[0] != 7 {
		t.Fatalf("sticky limits seen = %v, want [7]: the policy's limit reaches the cursor", cursor.sticky)
	}
}

// TestSelector_UnreadablePolicyDegradesToFillFirst pins the failure direction:
// a settings read that fails serves priority order instead of failing the
// request, and leaves the cursor untouched.
func TestSelector_UnreadablePolicyDegradesToFillFirst(t *testing.T) {
	repo := twoEndpointProvider(t, "alpha")
	cursor := &stubCursor{offset: 1}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, Cursor: cursor,
		Strategies: stubStrategy{err: errPolicyUnavailable},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	selection, err := selector.Select(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selection.Endpoint.ID() != "ep-1" {
		t.Fatalf("endpoint = %s, want ep-1: an unreadable policy degrades to priority order", selection.Endpoint.ID())
	}
	if cursor.calls != 0 {
		t.Fatalf("cursor calls = %d, want 0: the degraded walk reads no rotation state", cursor.calls)
	}
}

// TestSelector_PolicyIsResolvedPerProvider pins the override: two providers on
// one selector walk their own modes, because the policy is asked per provider.
func TestSelector_PolicyIsResolvedPerProvider(t *testing.T) {
	repo := twoEndpointProvider(t, "alpha")
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep-1", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky-1", priority: 1}}),
		buildEndpoint(t, "ep-2", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky-2", priority: 1}}),
	}
	cursor := &stubCursor{offset: 1}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, Cursor: cursor,
		Strategies: mapStrategy{
			"alpha": {Strategy: domain.RotationFillFirst, StickyLimit: 3},
			"beta":  {Strategy: domain.RotationRoundRobin, StickyLimit: 5},
		},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	alpha, err := selector.Select(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Select(alpha) error = %v", err)
	}
	if alpha.Endpoint.ID() != "ep-1" || cursor.calls != 0 {
		t.Fatalf("alpha = %s with %d cursor calls, want ep-1 and 0", alpha.Endpoint.ID(), cursor.calls)
	}

	beta, err := selector.Select(context.Background(), "beta")
	if err != nil {
		t.Fatalf("Select(beta) error = %v", err)
	}
	if beta.Endpoint.ID() != "ep-2" {
		t.Fatalf("beta = %s, want ep-2: its own round-robin policy decides", beta.Endpoint.ID())
	}
	if len(cursor.sticky) != 1 || cursor.sticky[0] != 5 {
		t.Fatalf("sticky limits seen = %v, want [5]: beta's own limit reaches the cursor", cursor.sticky)
	}
}

// errPolicyUnavailable stands for a settings read that failed.
var errPolicyUnavailable = &stubError{"settings unavailable"}

// stubError is a minimal error, so the test file does not import errors for one
// value.
type stubError struct{ text string }

func (e *stubError) Error() string { return e.text }
