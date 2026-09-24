// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_probe_selection_test.go
// @for       Tests for which key a connectivity test targets and which paths
//
//	cannot probe at all (SPEC-API-001 §7.5).
//
// @uses      context, strconv, testing, internal/domain.
// @reason    §7.5 lets an operator test a named key or the default one, and a key
//
//	inside its backoff is exactly what a manual test is for, so the targeting
//	rules differ from the router's selection on purpose. A refusal that read
//	as a 500, or a probe that ran on a blank credential, would both look
//	like a broken upstream; what the probe records lives in
//	endpoint_probe_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestEndpointService_TestSelectsTheKey pins §7.5's targeting rule: a named key is
// tested as named, the default is the first active key by priority, and a key outside
// its backoff is still testable, which is the whole point of a manual test.
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
