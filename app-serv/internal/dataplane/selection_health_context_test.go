// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_health_context_test.go
// @for       The health write surviving a caller whose context is already cancelled: the key's park, its reset, and the parking budget read that sizes them.
// @uses      context, testing, time, internal/domain.
// @reason    A served request writes its health after the answer is on the wire, and a client that closes the connection at that moment cancels the request context before the write lands. Losing that write leaves a recovered credential parked and a failing one unparked, so the router's view disagrees with what the upstream actually did, and the operator's panel keeps showing a failure that has been fixed. Draft 021 F6 closed this for the usage rows; these pins close it for the health row.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-13
package dataplane

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// cancelledContext returns a context that is already done, the state a request
// context reaches the moment the client hangs up.
func cancelledContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cancel()
	return ctx
}

// healthFixture seeds one provider-a endpoint with one key and hands back the
// repo and a selector clocked at the fixture instant.
func healthFixture(t *testing.T, health KeyHealthPolicyReader) (*memEndpointRepo, *Selector) {
	t.Helper()
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
	}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, Health: health})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }
	return repo, selector
}

// TestSelector_RecordSuccessWritesThroughACancelledContext is the finding's own
// shape: the answer reached the client, the client left, and the key's recovery
// still has to be recorded.
func TestSelector_RecordSuccessWritesThroughACancelledContext(t *testing.T) {
	repo, selector := healthFixture(t, nil)
	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}

	if err := selector.RecordSuccess(cancelledContext(t), selection); err != nil {
		t.Fatalf("RecordSuccess() error = %v, want the cancelled caller not to erase the write", err)
	}
	stored, ok := repo.health["uky_a"]
	if !ok {
		t.Fatal("the key health write was lost with the caller's context")
	}
	if stored.Status() != domain.UpstreamKeyActive || stored.ConsecutiveErrors() != 0 {
		t.Fatalf("stored key = %q with %d errors, want the served call to clear its park",
			stored.Status(), stored.ConsecutiveErrors())
	}
	if _, seen := repo.outcome["ep_a"]; !seen {
		t.Fatal("the endpoint parity write was lost with the key write, so the panel keeps the old failure")
	}
}

// TestSelector_RecordFailureParksThroughACancelledContext is the same rule on the
// other side: a failure the client walked away from still parks the credential
// that caused it, because the next request would otherwise spend it again.
func TestSelector_RecordFailureParksThroughACancelledContext(t *testing.T) {
	repo, selector := healthFixture(t, nil)
	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}

	if err := selector.RecordFailure(cancelledContext(t), selection,
		"the credential was refused", domain.KeyFailureAuth, 0); err != nil {
		t.Fatalf("RecordFailure() error = %v", err)
	}
	stored, ok := repo.health["uky_a"]
	if !ok {
		t.Fatal("a failure was lost with the caller's context, so an unusable key stays selectable")
	}
	if stored.Status() != domain.UpstreamKeyError || stored.RateLimitedUntil() == nil {
		t.Fatalf("stored key = %q with window %v, want it parked", stored.Status(), stored.RateLimitedUntil())
	}
}

// TestSelector_ReadsTheParkingBudgetThroughACancelledContext is the sharper half:
// the budget is read from settings on the way to the same write, so a detached
// write that still read the budget on the dead context would park the key at the
// documented default while the operator configured another number.
func TestSelector_ReadsTheParkingBudgetThroughACancelledContext(t *testing.T) {
	health := &stubHealth{policies: map[string]domain.KeyHealthPolicy{
		"provider-a": {AuthWindow: 40 * time.Second},
	}}
	repo, selector := healthFixture(t, health)
	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}

	if err := selector.RecordFailure(cancelledContext(t), selection,
		"the credential was refused", domain.KeyFailureAuth, 0); err != nil {
		t.Fatalf("RecordFailure() error = %v", err)
	}
	stored := repo.health["uky_a"]
	until := stored.RateLimitedUntil()
	if until == nil {
		t.Fatal("the key was not parked")
	}
	if got := until.Sub(now); got != 40*time.Second {
		t.Fatalf("park window = %v, want the configured 40s: the budget read ran on the dead context", got)
	}
	if len(health.asked) != 1 {
		t.Fatalf("policy reads = %v, want one read through the detached context", health.asked)
	}
}
