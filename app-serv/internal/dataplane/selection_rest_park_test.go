// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_rest_park_test.go
// @for       The selector's read of one classified refusal: the key is parked until the reset the vendor stated and is not offered to the next request before it.
// @uses      context, testing, time, internal/domain.
// @reason    C1 is one mechanism read from both ends, and this is the parking end: under the old five-minute ceiling a key spent for four days was back in rotation within one cycle. It is pinned on a mocked clock so the park and its expiry are what fail when either changes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSelector_QuotaExhaustedParksUntilTheStatedReset is the finding itself, read
// on a mocked clock: the key is withheld until 2026-10-19 10:48 UTC and returns to
// rotation the moment that instant passes. Under the old five-minute ceiling it
// was back in rotation within one cycle, four days early.
func TestSelector_QuotaExhaustedParksUntilTheStatedReset(t *testing.T) {
	reset := harborReset(t)
	failed := time.Date(2026, 10, 15, 10, 30, 0, 0, time.UTC)

	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
	}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return failed }

	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}
	rest := domain.RestDirective{
		Class:       domain.FailureQuotaExhausted,
		ResetAt:     reset,
		Source:      domain.ResetSourceBody,
		SignatureID: "tokenharbor.free-allowance",
	}
	if _, err := selector.RecordFailure(context.Background(), selection, "allowance spent",
		domain.FailureQuotaExhausted.KeyClass(), rest); err != nil {
		t.Fatalf("recording: %v", err)
	}

	stored := repo.health["uky_a"]
	until := stored.RateLimitedUntil()
	if until == nil || !until.Equal(reset) {
		t.Fatalf("parked until = %v, want the stated reset %v", until, reset)
	}
	if stored.LastErrorClass() != string(domain.FailureQuotaExhausted) {
		t.Fatalf("last_error_class = %q, want the recognised class", stored.LastErrorClass())
	}
	if stored.ResetSource() != string(domain.ResetSourceBody) {
		t.Fatalf("reset_source = %q, want body", stored.ResetSource())
	}

	// The walk asks the clock whether the key may serve, and the answer is no
	// until the vendor's own date: ten minutes later, a day later, a week later.
	for _, at := range []time.Time{
		failed.Add(10 * time.Minute),
		reset.Add(-24 * time.Hour),
		reset.Add(-time.Second),
	} {
		if stored.Available(at) {
			t.Fatalf("the key is available at %v, want it withheld until %v", at, reset)
		}
	}
	if !stored.Available(reset) {
		t.Fatalf("the key is still parked at %v, want it back in rotation", reset)
	}
	// A served call clears the whole classification, park included.
	stored.RecordSuccess(reset)
	if stored.RateLimitedUntil() != nil || stored.ResetSource() != "" || stored.SignatureID() != "" {
		t.Fatal("a served call left a park or a classification behind")
	}
}

// TestSelector_AStatedResetReachesTheNextSelection is the same finding one level
// up: a key parked to a far-off reset is not offered to the next request, so the
// walk spends its legs on credentials that can serve.
func TestSelector_AStatedResetReachesTheNextSelection(t *testing.T) {
	reset := harborReset(t)
	failed := time.Date(2026, 10, 15, 10, 30, 0, 0, time.UTC)

	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
			{id: "uky_spent", priority: 1}, {id: "uky_healthy", priority: 2},
		}),
	}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return failed }

	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}
	if selection.Key.ID() != "uky_spent" {
		t.Fatalf("selected = %s, want the first key by priority", selection.Key.ID())
	}
	if _, err := selector.RecordFailure(context.Background(), selection, "allowance spent",
		domain.FailureQuotaExhausted.KeyClass(), domain.RestDirective{
			Class:   domain.FailureQuotaExhausted,
			ResetAt: reset,
			Source:  domain.ResetSourceBody,
		}); err != nil {
		t.Fatalf("recording: %v", err)
	}

	next, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting again: %v", err)
	}
	if next.Key.ID() != "uky_healthy" {
		t.Fatalf("next selection = %s, want the key whose park has not expired", next.Key.ID())
	}
}
