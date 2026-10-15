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

func (s *stubHealth) KeyHealthPolicy(ctx context.Context, providerID string) (domain.KeyHealthPolicy, error) {
	if err := ctx.Err(); err != nil {
		return domain.KeyHealthPolicy{}, err
	}
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
	if _, err := selector.RecordFailure(context.Background(), selection, "upstream rejected", domain.KeyFailureAuth, domain.RestDirective{}); err != nil {
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
			if _, err := selector.RecordFailure(context.Background(), selection, "upstream rejected", domain.KeyFailureAuth, domain.RestDirective{}); err != nil {
				t.Fatalf("recording: %v", err)
			}
			if got := parkWindow(t, repo.health["uky_a"]); got != 2*time.Minute {
				t.Fatalf("park window = %v, want the documented 2m", got)
			}
		})
	}
}

// TestSelector_KeylessSelectionParksTheAccount states the boundary of the account circuit:
// an account with no credential row parks as one when the failure says its credential is
// the cause, and asks for no parking budget when it does not. Before this rule a dead
// OAuth token was never parked, so every request paid a round trip to be refused.
func TestSelector_KeylessSelectionParksTheAccount(t *testing.T) {
	cases := []struct {
		name       string
		class      domain.KeyFailureClass
		wantAsked  int
		wantWindow time.Duration
	}{
		{name: "a transient outage parks the account for the flat window",
			class: domain.KeyFailureTransient, wantAsked: 1, wantWindow: 30 * time.Second},
		{name: "a refused credential parks it longer",
			class: domain.KeyFailureAuth, wantAsked: 1, wantWindow: 2 * time.Minute},
		{name: "a network failure parks nothing and reads no budget",
			class: domain.KeyFailureTarget, wantAsked: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			if _, err := selector.RecordFailure(context.Background(), selection, "the upstream is overloaded", tc.class, domain.RestDirective{}); err != nil {
				t.Fatalf("recording: %v", err)
			}
			if len(health.asked) != tc.wantAsked {
				t.Fatalf("policy reads = %v, want %d", health.asked, tc.wantAsked)
			}
			parked := repo.outcome["ep_a"].RateLimitedUntil()
			if tc.wantWindow == 0 {
				if parked != nil {
					t.Fatalf("account park = %v, want nothing set for a failure the credential caused", parked)
				}
				return
			}
			if parked == nil || !parked.Equal(now.Add(tc.wantWindow)) {
				t.Fatalf("account park = %v, want %v", parked, now.Add(tc.wantWindow))
			}
			if got := repo.outcome["ep_a"].AccountConsecutiveErrors(); got != 1 {
				t.Fatalf("account run = %d, want 1", got)
			}
		})
	}
}

// TestSelector_KeyedSelectionNeverParksTheAccount pins draft 050 T1 from the other
// side: one key of a multi-key account being refused must not withhold the whole
// account from rotation, because the healthy keys beside it still answer.
func TestSelector_KeyedSelectionNeverParksTheAccount(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
	}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{},
		Health: &stubHealth{policies: map[string]domain.KeyHealthPolicy{"provider-a": {}}},
	})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}
	if _, err := selector.RecordFailure(context.Background(), selection, "too many requests", domain.KeyFailureRateLimit, domain.RestDirective{}); err != nil {
		t.Fatalf("recording: %v", err)
	}
	if got := parkWindow(t, repo.health["uky_a"]); got != 2*time.Second {
		t.Fatalf("key park = %v, want the 2s first step", got)
	}
	if parked := repo.outcome["ep_a"].RateLimitedUntil(); parked != nil {
		t.Fatalf("account park = %v, want nil: a throttle on one key keeps its siblings in rotation", parked)
	}
}
