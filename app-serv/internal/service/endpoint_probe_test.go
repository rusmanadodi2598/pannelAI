// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_probe_test.go
// @for       Tests for what a connectivity test records: the endpoint's
//
//	test_status and the key's health (SPEC-API-001 §7.5).
//
// @uses      context, errors, testing, time, internal/domain.
// @reason    §7.5 makes a test answer "does this credential work", and the answer has
//
//	to reach both the endpoint's test_status and the key's circuit state
//	without a second health field. A plausible bug records one and not the
//	other, or reports a refusal as a 500 instead of a fail state, so both are
//	pinned here. The key-targeting rules live in
//	endpoint_probe_selection_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestEndpointService_TestRecordsBothStatuses pins the recording rule: a probe outcome
// reaches the endpoint's test_status, the key's health, and both persist.
func TestEndpointService_TestRecordsBothStatuses(t *testing.T) {
	cases := []struct {
		name           string
		outcome        ProbeOutcome
		proberErr      error
		wantState      string
		wantKeyStatus  domain.UpstreamKeyStatus
		wantErrors     int
		wantUsedAtSet  bool
		wantRateWindow bool
	}{
		{
			name:          "a success records ok and resets the key",
			outcome:       ProbeOutcome{State: domain.EndpointTestOK, LatencyMS: 42, Status: 200},
			wantState:     domain.EndpointTestOK,
			wantKeyStatus: domain.UpstreamKeyActive,
			wantUsedAtSet: true,
		},
		{
			name:           "a refusal records fail and parks the key",
			outcome:        ProbeOutcome{State: domain.EndpointTestFail, LatencyMS: 11, Status: 401, Message: "credential rejected"},
			wantState:      domain.EndpointTestFail,
			wantKeyStatus:  domain.UpstreamKeyError,
			wantErrors:     1,
			wantRateWindow: true,
		},
		{
			name:           "a prober fault is recorded as a failure, not returned",
			proberErr:      errors.New("no connector for this provider"),
			wantState:      domain.EndpointTestFail,
			wantKeyStatus:  domain.UpstreamKeyError,
			wantErrors:     1,
			wantRateWindow: true,
		},
		{
			name:           "an unknown state reported by a connector becomes fail",
			outcome:        ProbeOutcome{State: "maybe", LatencyMS: 5},
			wantState:      domain.EndpointTestFail,
			wantKeyStatus:  domain.UpstreamKeyError,
			wantErrors:     1,
			wantRateWindow: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newEndpointSvc(t)
			prober := &fakeProber{outcome: tc.outcome, err: tc.proberErr}
			svc.prober = prober
			endpoint := keyedEndpointWith(t, svc, "deepseek", "acct", "primary", "secondary")

			_, outcome, err := svc.Test(context.Background(), endpoint.ID(), "")
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			if outcome.State != tc.wantState {
				t.Fatalf("outcome state = %q, want %q (%s)", outcome.State, tc.wantState, describe(outcome))
			}
			if prober.calls != 1 {
				t.Fatalf("prober calls = %d, want 1", prober.calls)
			}

			reloaded, err := svc.Get(context.Background(), endpoint.ID())
			if err != nil {
				t.Fatal(err)
			}
			if got := reloaded.TestStatus().State; got != tc.wantState {
				t.Fatalf("test_status = %q, want %q", got, tc.wantState)
			}
			if reloaded.TestStatus().CheckedAt == nil {
				t.Fatal("test_status must carry the instant it was recorded at")
			}

			key := reloaded.Keys()[0]
			if key.Status() != tc.wantKeyStatus {
				t.Fatalf("key status = %q, want %q", key.Status(), tc.wantKeyStatus)
			}
			if key.ConsecutiveErrors() != tc.wantErrors {
				t.Fatalf("consecutive errors = %d, want %d", key.ConsecutiveErrors(), tc.wantErrors)
			}
			if tc.wantUsedAtSet && key.LastUsedAt() == nil {
				t.Fatal("a successful probe must stamp last_used_at")
			}
			if tc.wantRateWindow && key.RateLimitedUntil() == nil {
				t.Fatal("a failed probe must park the key in its backoff window")
			}
			// The persisted store must agree with the reloaded aggregate: a probe
			// whose health only reached memory would leave the router's view stale.
			if len(store.keysByEndpoint[endpoint.ID()]) == 0 {
				t.Fatal("the key health was not persisted")
			}
		})
	}
}

// TestEndpointService_TestParksOnFirstFailure pins the first-failure parking rule:
// one refused probe parks the key in a window sized by the upstream's own status
// (a 401 is the credential's failure, so two minutes), exactly as the same answer
// would during routing, because the breaker owns that state rather than the probe.
func TestEndpointService_TestParksOnFirstFailure(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	svc.prober = &fakeProber{outcome: ProbeOutcome{State: domain.EndpointTestFail, Status: 401, Message: "rejected"}}
	endpoint := keyedEndpointWith(t, svc, "deepseek", "acct", "primary", "secondary")
	ctx := context.Background()

	if _, _, err := svc.Test(ctx, endpoint.ID(), endpoint.Keys()[0].ID()); err != nil {
		t.Fatalf("Test() error = %v", err)
	}

	reloaded, err := svc.Get(ctx, endpoint.ID())
	if err != nil {
		t.Fatal(err)
	}
	parked := reloaded.Key(endpoint.Keys()[0].ID())
	if parked.Status() != domain.UpstreamKeyError {
		t.Fatalf("status = %q, want error after one failure", parked.Status())
	}
	if parked.RateLimitedUntil() == nil {
		t.Fatal("a parked key must carry its backoff window")
	}
	if want := testNow.Add(2 * time.Minute); !parked.RateLimitedUntil().Equal(want) {
		t.Fatalf("backoff until %v, want %v for an auth-shaped refusal", parked.RateLimitedUntil(), want)
	}
	if parked.Available(testNow) {
		t.Fatal("a parked key must not be available inside its backoff window")
	}
	// A success clears it again, which is what makes a fixed credential recoverable.
	svc.prober = &fakeProber{outcome: ProbeOutcome{State: domain.EndpointTestOK, Status: 200}}
	if _, _, err := svc.Test(ctx, endpoint.ID(), endpoint.Keys()[0].ID()); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.Get(ctx, endpoint.ID())
	if err != nil {
		t.Fatal(err)
	}
	if got := recovered.Key(endpoint.Keys()[0].ID()); got.Status() != domain.UpstreamKeyActive {
		t.Fatalf("status = %q, want active after a success", got.Status())
	}
}
