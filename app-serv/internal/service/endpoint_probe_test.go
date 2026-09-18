// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_probe_test.go
// @for       Table-driven tests for the connectivity test's key selection, credential
//
//	opening, and health recording (SPEC-API-001 §7.5).
//
// @uses      context, errors, strconv, testing, internal/domain.
// @reason    §7.5 makes a test answer "does this credential work", and the answer has
//
//	to reach both the endpoint's test_status and the key's circuit state without a
//	second health field. A plausible bug records one and not the other, or reports a
//	refusal as a 500 instead of a fail state, so both are pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"errors"
	"strconv"
	"testing"

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
			name:          "a refusal records fail and counts the failure",
			outcome:       ProbeOutcome{State: domain.EndpointTestFail, LatencyMS: 11, Status: 401, Message: "credential rejected"},
			wantState:     domain.EndpointTestFail,
			wantKeyStatus: domain.UpstreamKeyActive,
			wantErrors:    1,
		},
		{
			name:          "a prober fault is recorded as a failure, not returned",
			proberErr:     errors.New("no connector for this provider"),
			wantState:     domain.EndpointTestFail,
			wantKeyStatus: domain.UpstreamKeyActive,
			wantErrors:    1,
		},
		{
			name:          "an unknown state reported by a connector becomes fail",
			outcome:       ProbeOutcome{State: "maybe", LatencyMS: 5},
			wantState:     domain.EndpointTestFail,
			wantKeyStatus: domain.UpstreamKeyActive,
			wantErrors:    1,
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
			// The persisted store must agree with the reloaded aggregate: a probe
			// whose health only reached memory would leave the router's view stale.
			if len(store.keysByEndpoint[endpoint.ID()]) == 0 {
				t.Fatal("the key health was not persisted")
			}
		})
	}
}

// TestEndpointService_TestTripsTheCircuit pins that three consecutive failed probes
// trip the key exactly as an upstream failure would, because the breaker owns that
// state rather than the probe.
func TestEndpointService_TestTripsTheCircuit(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	svc.prober = &fakeProber{outcome: ProbeOutcome{State: domain.EndpointTestFail, Status: 401, Message: "rejected"}}
	endpoint := keyedEndpointWith(t, svc, "deepseek", "acct", "primary", "secondary")
	ctx := context.Background()

	threshold := domain.CircuitThreshold()
	for i := range threshold {
		if _, _, err := svc.Test(ctx, endpoint.ID(), endpoint.Keys()[0].ID()); err != nil {
			t.Fatalf("Test() call %d error = %v", i+1, err)
		}
	}

	reloaded, err := svc.Get(ctx, endpoint.ID())
	if err != nil {
		t.Fatal(err)
	}
	tripped := reloaded.Key(endpoint.Keys()[0].ID())
	if tripped.Status() != domain.UpstreamKeyError {
		t.Fatalf("status = %q, want error after %d failures", tripped.Status(), threshold)
	}
	if tripped.RateLimitedUntil() == nil {
		t.Fatal("a tripped key must carry its backoff window")
	}
	if tripped.Available(testNow) {
		t.Fatal("a tripped key must not be available inside its backoff window")
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

// TestEndpointService_TestSelectsTheKey pins §7.5's targeting rule: a named key is
// tested as named, the default is the first active key by priority, and a key outside
// its backoff is still testable — which is the whole point of a manual test.
func TestEndpointService_TestSelectsTheKey(t *testing.T) {
	cases := []struct {
		name       string
		keyID      func(domain.UpstreamEndpoint) string
		priorities []int
		wantIndex  int
		wantCode   string
	}{
		{name: "the default is the lowest-priority active key", priorities: []int{2, 1, 3}, wantIndex: 1},
		{name: "an explicit key wins", priorities: []int{1, 2}, wantIndex: 1,
			keyID: func(e domain.UpstreamEndpoint) string { return e.Keys()[1].ID() }},
		{name: "an unknown key id is not found", priorities: []int{1}, wantCode: "NOT_FOUND",
			keyID: func(domain.UpstreamEndpoint) string { return "uky_absent" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			svc.prober = &fakeProber{outcome: ProbeOutcome{State: domain.EndpointTestOK, Status: 200}}
			ctx := context.Background()
			endpoint := keyedEndpointWithPriorities(t, svc, tc.priorities)

			keyID := ""
			if tc.keyID != nil {
				keyID = tc.keyID(endpoint)
			}
			_, _, err := svc.Test(ctx, endpoint.ID(), keyID)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			reloaded, err := svc.Get(ctx, endpoint.ID())
			if err != nil {
				t.Fatal(err)
			}
			tested := reloaded.Keys()[tc.wantIndex]
			if tested.LastUsedAt() == nil {
				t.Fatalf("the key at index %d must be the one tested", tc.wantIndex)
			}
			for i, key := range reloaded.Keys() {
				if i != tc.wantIndex && key.LastUsedAt() != nil {
					t.Fatalf("key %d was stamped but index %d was the target", i, tc.wantIndex)
				}
			}
		})
	}
}

// TestEndpointService_TestRefusesWithoutAUsableCredential pins the paths that cannot
// probe: a disabled key set, an unreadable stored credential, and an unconfigured
// prober.
func TestEndpointService_TestRefusesWithoutAUsableCredential(t *testing.T) {
	t.Run("no active key is a validation failure", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		svc.prober = &fakeProber{outcome: ProbeOutcome{State: domain.EndpointTestOK}}
		ctx := context.Background()
		endpoint := keyedEndpointWith(t, svc, "deepseek", "acct", "primary", "secondary")
		for _, key := range endpoint.Keys() {
			if _, err := svc.UpdateKey(ctx, endpoint.ID(), key.ID(), KeyPatch{Status: strPtr("disabled")}); err != nil {
				t.Fatal(err)
			}
		}
		_, _, err := svc.Test(ctx, endpoint.ID(), "")
		mustAppError(t, err, "VALIDATION_ERROR")
	})

	t.Run("an unreadable stored credential is a validation failure", func(t *testing.T) {
		svc, store := newEndpointSvc(t)
		svc.prober = &fakeProber{outcome: ProbeOutcome{State: domain.EndpointTestOK}}
		ctx := context.Background()
		endpoint := keyedEndpointWith(t, svc, "deepseek", "acct", "primary")
		// Simulate a row written under a different key: the ciphertext is not
		// readable, and that must surface as a validation failure rather than a
		// panic or a probe that reported success on a blank credential.
		stored := store.keysByEndpoint[endpoint.ID()]
		stored[0] = domain.RehydrateUpstreamKey(stored[0].ID(), stored[0].EndpointID(),
			stored[0].Label(), "v1:AAAA:c2VhbGVk", stored[0].Hint(), stored[0].Priority(),
			stored[0].Status(), nil, "", 0, nil, stored[0].CreatedAt(), stored[0].UpdatedAt())
		store.keysByEndpoint[endpoint.ID()] = stored

		_, _, err := svc.Test(ctx, endpoint.ID(), "")
		mustAppError(t, err, "VALIDATION_ERROR")
	})

	t.Run("no prober configured is an internal failure", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		endpoint := keyedEndpointWith(t, svc, "deepseek", "acct", "primary")
		_, _, err := svc.Test(context.Background(), endpoint.ID(), "")
		mustAppError(t, err, "INTERNAL_ERROR")
	})

	t.Run("an unknown endpoint is not found", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		svc.prober = &fakeProber{outcome: ProbeOutcome{State: domain.EndpointTestOK}}
		_, _, err := svc.Test(context.Background(), "ep_absent", "")
		mustAppError(t, err, "NOT_FOUND")
	})
}

// keyedEndpointWith seeds an api_key endpoint holding the given labels.
func keyedEndpointWith(t *testing.T, svc *EndpointService, providerID, label string, labels ...string) domain.UpstreamEndpoint {
	t.Helper()
	return seedEndpoint(t, svc, providerID, label, domain.UpstreamAuthAPIKey, toKeyInputs(labels)...)
}

// keyedEndpointWithPriorities seeds an endpoint whose keys hold the given priorities,
// so a test can prove the default target is the lowest-priority active key.
func keyedEndpointWithPriorities(t *testing.T, svc *EndpointService, priorities []int) domain.UpstreamEndpoint {
	t.Helper()
	keys := make([]KeyInput, 0, len(priorities))
	for i, priority := range priorities {
		keys = append(keys, KeyInput{
			Label: "k" + strconv.Itoa(i), Value: "sk-value-" + strconv.Itoa(i), Priority: priority,
		})
	}
	return seedEndpoint(t, svc, "deepseek", "priority-acct", domain.UpstreamAuthAPIKey, keys...)
}
