// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_pin_test.go
// @for       The pinned selection an account-bound route needs: one named endpoint, its own key rule, and no rotation state behind it.
// @uses      context, testing, internal/domain.
// @reason    An async video job lives on the account that created it, so its poll must reach that account rather than the provider's next healthy one (docs/PORT/011-PORT-VIDEO-ASYNC-JOB.md). The pin is a rule about selection like any other, so it is pinned here against the in-memory double, including that it reads no rotation cursor and that a pin naming nothing is a request defect rather than an outage.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package dataplane

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSelector_PinnedEndpointAnswers pins the pin: the named endpoint answers
// even though another one outranks it, and its own key rule still applies.
func TestSelector_PinnedEndpointAnswers(t *testing.T) {
	cases := []struct {
		name         string
		pin          string
		endpoints    []domain.UpstreamEndpoint
		wantEndpoint string
		wantKey      string
	}{
		{
			name: "the pin outranks the priority order",
			pin:  "ep_b",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_b", priority: 1}}),
			},
			wantEndpoint: "ep_b", wantKey: "uky_b",
		},
		{
			name: "the pinned endpoint's own key rule applies",
			pin:  "ep_b",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_parked", priority: 1, failures: 1},
					{id: "uky_healthy", priority: 2},
				}),
			},
			wantEndpoint: "ep_b", wantKey: "uky_healthy",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = tc.endpoints
			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
			if err != nil {
				t.Fatalf("NewSelector() error = %v", err)
			}
			selector.clock = func() time.Time { return now }

			got, err := selector.SelectEndpoint(context.Background(), "provider-a", tc.pin)
			if err != nil {
				t.Fatalf("SelectEndpoint() error = %v", err)
			}
			if got.Endpoint.ID() != tc.wantEndpoint || got.Key.ID() != tc.wantKey {
				t.Fatalf("SelectEndpoint() = %s/%s, want %s/%s",
					got.Endpoint.ID(), got.Key.ID(), tc.wantEndpoint, tc.wantKey)
			}
		})
	}
}

// TestSelector_PinnedEndpointReadsNoCursor pins the rule that keeps an
// account-bound route from moving the unpinned ones: a caller that named an
// endpoint has already chosen, so the rotation cursor is neither read nor
// advanced, even under round-robin.
func TestSelector_PinnedEndpointReadsNoCursor(t *testing.T) {
	repo := twoEndpointProvider(t, "alpha")
	cursor := &stubCursor{offset: 1}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo, Opener: opener{}, Cursor: cursor,
		Strategies: stubStrategy{policy: domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 3}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}

	selection, err := selector.SelectEndpoint(context.Background(), "alpha", "ep-1")
	if err != nil {
		t.Fatalf("SelectEndpoint() error = %v", err)
	}
	if selection.Endpoint.ID() != "ep-1" {
		t.Fatalf("endpoint = %s, want ep-1: the pin decides, not the cursor", selection.Endpoint.ID())
	}
	if cursor.calls != 0 {
		t.Fatalf("cursor calls = %d, want 0: a pin reads no rotation state", cursor.calls)
	}
}

// TestSelector_PinnedEndpointRefusals pins the two failures apart: a pin the
// provider does not offer as an active endpoint is a request defect the client
// can fix, while a pin on an endpoint that exists but cannot serve is the same
// unavailability the unpinned walk reports.
func TestSelector_PinnedEndpointRefusals(t *testing.T) {
	cases := []struct {
		name      string
		pin       string
		endpoints []domain.UpstreamEndpoint
		wantCode  string
	}{
		{
			name: "a pin the provider does not have",
			pin:  "ep_missing",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
			},
			wantCode: CodeValidation,
		},
		{
			name: "a pin on an endpoint that is not active",
			pin:  "ep_a",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointDisabled, []keyFixture{{id: "uky_a", priority: 1}}),
			},
			wantCode: CodeValidation,
		},
		{
			name: "a pin on an endpoint whose every key is parked",
			pin:  "ep_a",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_1", priority: 1, failures: 1},
					{id: "uky_2", priority: 2, failures: 1},
				}),
			},
			wantCode: CodeNoProvider,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = tc.endpoints
			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
			if err != nil {
				t.Fatalf("NewSelector() error = %v", err)
			}
			selector.clock = func() time.Time { return now }

			_, err = selector.SelectEndpoint(context.Background(), "provider-a", tc.pin)
			if err == nil {
				t.Fatal("SelectEndpoint() error = nil, want the pin refused")
			}
			if code := AsError(err).Code; code != tc.wantCode {
				t.Fatalf("code = %q, want %q", code, tc.wantCode)
			}
		})
	}
}

// TestSelector_PinnedAccountParkedByItsOwnBreaker states the consequence of the account
// circuit: an async video poll names the account that started its job, and once that
// account is parked the pin is refused the way an exhausted key set is. The job is not
// lost, because the panel retries the poll and the window expires on its own clock.
func TestSelector_PinnedAccountParkedByItsOwnBreaker(t *testing.T) {
	parked := noAuthEndpoint(t, "ep_oauth", 1)
	if !parked.ServesItsOwnCredential() {
		t.Fatal("an account with no key row must be its own credential for this fixture to mean anything")
	}
	if !parked.RecordAccountFailure(domain.KeyFailureAuth, domain.DefaultKeyHealthPolicy(), now) {
		t.Fatal("the fixture parked nothing")
	}

	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{parked}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	selector.clock = func() time.Time { return now }

	_, err = selector.SelectEndpoint(context.Background(), "provider-a", "ep_oauth")
	if AsError(err).Code != CodeNoProvider {
		t.Fatalf("pin on a parked account = %v, want %s", AsError(err).Code, CodeNoProvider)
	}

	// The same pin succeeds once the deadline passes: the window is the whole state,
	// so nothing has to sweep it and no operator action releases a recovered account.
	selector.clock = func() time.Time { return now.Add(time.Hour) }
	if _, err := selector.SelectEndpoint(context.Background(), "provider-a", "ep_oauth"); err != nil {
		t.Fatalf("SelectEndpoint() after the window = %v, want the account usable again", err)
	}
}
