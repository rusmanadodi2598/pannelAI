// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_health_test.go
// @for       Table-driven tests for how an outcome is accounted back onto the endpoint and key.
// @uses      context, errors, strings, testing, time, internal/domain
// @reason    SPEC-API-001 §7.5 requires an upstream outcome to change the domain's own health state
//
//	rather than a copy of it, and requires a storage failure or an unreadable credential to
//	surface as an error rather than a silent skip. Both are what make the circuit breaker
//	and the retry policy trustworthy (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSelector_HealthAccounting pins that an upstream outcome changes the domain's
// own health state and persists it through RecordKeyHealth, so no second health
// model exists.
func TestSelector_HealthAccounting(t *testing.T) {
	cases := []struct {
		name          string
		preParked     bool
		parkClass     domain.KeyFailureClass
		outcome       string
		wantStatus    domain.UpstreamKeyStatus
		wantCounterAt int
		wantPersisted bool
	}{
		{name: "a success clears the counter", preParked: true, parkClass: domain.KeyFailureAuth,
			outcome: "success", wantStatus: domain.UpstreamKeyActive, wantCounterAt: 0, wantPersisted: true},
		{name: "the first auth failure parks the key", outcome: "auth-failure",
			wantStatus: domain.UpstreamKeyError, wantCounterAt: 1, wantPersisted: true},
		{name: "the first rate limit parks the key", outcome: "rate-limit-failure",
			wantStatus: domain.UpstreamKeyError, wantCounterAt: 1, wantPersisted: true},
		{name: "a request-shaped failure is not persisted", outcome: "request-failure",
			wantStatus: domain.UpstreamKeyActive, wantCounterAt: 0, wantPersisted: false},
		{name: "a success after a park restores the key", preParked: true, parkClass: domain.KeyFailureTransient,
			outcome: "success", wantStatus: domain.UpstreamKeyActive, wantCounterAt: 0, wantPersisted: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			preParked := 0
			preAgo := time.Duration(0)
			if tc.preParked {
				preParked = 1
				// Parked far enough in the past that its window has expired, so
				// the fixture key is selectable and the outcome write is what
				// the case observes.
				preAgo = 2 * time.Minute
			}
			repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_a", priority: 1, failures: preParked, failuresAgo: preAgo},
				}),
			}
			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selector.clock = func() time.Time { return now }

			selection, err := selector.Select(context.Background(), "provider-a")
			if err != nil {
				t.Fatalf("selecting: %v", err)
			}
			switch tc.outcome {
			case "success":
				err = selector.RecordSuccess(context.Background(), selection)
			case "auth-failure":
				err = selector.RecordFailure(context.Background(), selection, "upstream rejected", domain.KeyFailureAuth)
			case "rate-limit-failure":
				err = selector.RecordFailure(context.Background(), selection, "rate limited", domain.KeyFailureRateLimit)
			case "request-failure":
				err = selector.RecordFailure(context.Background(), selection, "upstream rejected the request", domain.KeyFailureRequest)
			}
			if err != nil {
				t.Fatalf("recording %s: %v", tc.outcome, err)
			}

			stored, ok := repo.health["uky_a"]
			if tc.wantPersisted {
				if !ok {
					t.Fatal("RecordKeyHealth was not called, so the health change was not persisted")
				}
				if stored.Status() != tc.wantStatus {
					t.Fatalf("stored status = %q, want %q", stored.Status(), tc.wantStatus)
				}
				if stored.ConsecutiveErrors() != tc.wantCounterAt {
					t.Fatalf("stored consecutive errors = %d, want %d", stored.ConsecutiveErrors(), tc.wantCounterAt)
				}
				return
			}
			if ok {
				t.Fatalf("a request-shaped failure wrote health %+v, want no write at all", stored)
			}
		})
	}
}

// TestSelector_RepositoryFailureIsReported pins that a storage failure surfaces
// rather than being swallowed into an empty candidate list, which would be
// reported to the client as "no provider available" and hide a database outage.
func TestSelector_RepositoryFailureIsReported(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.err = errors.New("connection reset")
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	if _, err := selector.Select(context.Background(), "provider-a"); err == nil {
		t.Fatal("err = nil, want the storage failure")
	} else if !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("err = %v, want it to carry the storage cause", err)
	}
}

// TestSelector_UnreadableCredentialIsInternal pins that an undecryptable stored
// key is an internal failure, not a selection of a different account: sending the
// sealed value upstream would authenticate with ciphertext.
func TestSelector_UnreadableCredentialIsInternal(t *testing.T) {
	cases := []struct {
		name   string
		opener SecretOpener
	}{
		{name: "an opener that fails", opener: opener{err: errors.New("bad ciphertext")}},
		{name: "no opener at all", opener: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
			}
			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: tc.opener})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selection, err := selector.Select(context.Background(), "provider-a")
			if err == nil {
				t.Fatalf("err = nil, want an internal failure (selected %+v)", selection)
			}
			if code := domain.AsAppError(err).Code; code != "INTERNAL_ERROR" {
				t.Fatalf("code = %q, want INTERNAL_ERROR", code)
			}
		})
	}
}
