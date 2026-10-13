// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_health_policy_test.go
// @for       The selector applying the parking windows the routing group configured for the provider a failure belongs to.
// @uses      context, errors, testing, time, internal/domain.
// @reason    Draft 050 T8 made the §7.5 windows the operator's setting rather than the build's, and the selector is the only writer of a parked key on the serving path. The read is optional and degrades, so what has to be pinned is which number lands on the key when the read answers, when it fails, and when nothing is wired.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-13
package dataplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubHealth answers a configured parking budget per provider, and can fail one
// provider on purpose so the degradation is the thing under test.
type stubHealth struct {
	policies map[string]domain.KeyHealthPolicy
	err      error
	asked    []string
}

func (s *stubHealth) KeyHealthPolicy(_ context.Context, providerID string) (domain.KeyHealthPolicy, error) {
	s.asked = append(s.asked, providerID)
	if s.err != nil {
		return domain.KeyHealthPolicy{}, s.err
	}
	return s.policies[providerID], nil
}

// parkWindow is how long the persisted key is parked from the recorded instant.
func parkWindow(t *testing.T, stored domain.UpstreamKey) time.Duration {
	t.Helper()
	until := stored.RateLimitedUntil()
	if until == nil {
		t.Fatal("the key was persisted with no park, so nothing was withheld from rotation")
	}
	return until.Sub(now)
}

// TestSelector_RecordFailureAppliesTheProvidersWindow is the finding's fix read from
// the router's side: the provider's own window, not the compiled-in one, is what
// parks the key.
func TestSelector_RecordFailureAppliesTheProvidersWindow(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
	}
	health := &stubHealth{policies: map[string]domain.KeyHealthPolicy{
		"provider-a": {AuthWindow: 15 * time.Second},
	}}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, Health: health})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}
	if err := selector.RecordFailure(context.Background(), selection, "upstream rejected", domain.KeyFailureAuth); err != nil {
		t.Fatalf("recording: %v", err)
	}
	if got := parkWindow(t, repo.health["uky_a"]); got != 15*time.Second {
		t.Fatalf("park window = %v, want the provider's configured 15s", got)
	}
	if len(health.asked) != 1 || health.asked[0] != "provider-a" {
		t.Fatalf("policy reads = %v, want one read for provider-a", health.asked)
	}
}

// TestSelector_RecordFailureDegradesToTheDocumentedWindow covers both ways the read
// is absent: a settings row that cannot be read and a deployment that wired no
// reader at all. A failing credential must still be parked, at the length the spec
// documents.
func TestSelector_RecordFailureDegradesToTheDocumentedWindow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		deps   SelectorDeps
		reader *stubHealth
	}{
		{name: "no reader wired", deps: SelectorDeps{}},
		{name: "the reader fails", deps: SelectorDeps{}, reader: &stubHealth{err: errors.New("settings row unreadable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
			}
			deps := tc.deps
			deps.Endpoints, deps.Opener = repo, opener{}
			if tc.reader != nil {
				deps.Health = tc.reader
			}
			selector, err := NewSelector(deps)
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selector.clock = func() time.Time { return now }

			selection, err := selector.Select(context.Background(), "provider-a")
			if err != nil {
				t.Fatalf("selecting: %v", err)
			}
			if err := selector.RecordFailure(context.Background(), selection, "upstream rejected", domain.KeyFailureAuth); err != nil {
				t.Fatalf("recording: %v", err)
			}
			if got := parkWindow(t, repo.health["uky_a"]); got != 2*time.Minute {
				t.Fatalf("park window = %v, want the documented 2m", got)
			}
		})
	}
}

// TestSelector_KeylessSelectionReadsNoPolicy states the boundary the optional seam
// depends on: a selection with no key circuit never asks for a parking budget, so
// the settings read is not on the path a no_auth provider serves.
func TestSelector_KeylessSelectionReadsNoPolicy(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{noAuthEndpoint(t, "ep_a", 1)}
	health := &stubHealth{policies: map[string]domain.KeyHealthPolicy{"provider-a": {}}}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, Health: health})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}
	if err := selector.RecordFailure(context.Background(), selection, "the upstream is overloaded", domain.KeyFailureTransient); err != nil {
		t.Fatalf("recording: %v", err)
	}
	if len(health.asked) != 0 {
		t.Fatalf("policy reads = %v, want none for a keyless selection", health.asked)
	}
}
