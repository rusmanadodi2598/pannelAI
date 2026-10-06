// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_keys_test.go
// @for       Table-driven tests for key CRUD, the key batch, and the all-or-nothing rule (SPEC-API-001 §7.5, §8.1).
// @uses      context, errors, strconv, testing, internal/domain.
// @reason    §7.5 makes a key a child of the endpoint aggregate with a rule no single key can enforce, an api_key endpoint keeps at least one active credential, and §8.1 makes a batch refuse as a whole. Both are rules a plausible bug breaks while every other test still passes, which is why they are pinned here.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// keyedEndpoint seeds an api_key endpoint holding the given labels.
func keyedEndpoint(t *testing.T, svc *EndpointService, labels ...string) domain.UpstreamEndpoint {
	t.Helper()
	keys := make([]KeyInput, 0, len(labels))
	for _, label := range labels {
		keys = append(keys, KeyInput{Label: label, Value: "sk-" + label})
	}
	return seedEndpoint(t, svc, "deepseek", "acct", domain.UpstreamAuthAPIKey, keys...)
}

// TestEndpointService_AddKey pins add: the value is sealed, the hint reveals the tail
// only, and a duplicate label or an empty value is refused.
func TestEndpointService_AddKey(t *testing.T) {
	cases := []struct {
		name     string
		input    KeyInput
		existing []string
		wantCode string
	}{
		{name: "a named key", input: KeyInput{Label: "secondary", Value: "sk-new-value"}},
		{name: "an unnamed key gets a positional label", input: KeyInput{Value: "sk-anonymous-value-1234"}},
		{name: "an explicit priority is kept", input: KeyInput{Label: "early", Value: "sk-early", Priority: 1}},
		{name: "a duplicate label is refused", input: KeyInput{Label: "primary", Value: "sk-dup"},
			existing: []string{"primary"}, wantCode: "CONFLICT"},
		{name: "an empty value is refused", input: KeyInput{Label: "empty", Value: "   "},
			wantCode: "VALIDATION_ERROR"},
		{name: "a label past the bound is refused", input: KeyInput{Label: longLabel(121), Value: "sk-long"},
			wantCode: "VALIDATION_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			existing := tc.existing
			if len(existing) == 0 {
				existing = []string{"primary"}
			}
			endpoint := keyedEndpoint(t, svc, existing...)

			key, err := svc.AddKey(context.Background(), endpoint.ID(), tc.input)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("AddKey() error = %v", err)
			}
			if !isSealed(key.EncryptedValue()) {
				t.Fatalf("stored value %q is not sealed", key.EncryptedValue())
			}
			if len(tc.input.Value) > 7 && key.Hint() == tc.input.Value {
				t.Fatal("a value long enough to conceal must not be echoed as its own hint")
			}
			if key.Status() != domain.UpstreamKeyActive {
				t.Fatalf("status = %q, want active", key.Status())
			}
		})
	}
}

// TestEndpointService_ListKeys pins the child paging contract, including a page past
// the end reporting the total.
func TestEndpointService_ListKeys(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	endpoint := keyedEndpoint(t, svc, "a", "b", "c")

	cases := []struct {
		name     string
		page     int
		perPage  int
		wantRows int
		wantTot  int64
	}{
		{name: "the first page", page: 1, perPage: 2, wantRows: 2, wantTot: 3},
		{name: "the last partial page", page: 2, perPage: 2, wantRows: 1, wantTot: 3},
		{name: "past the end", page: 5, perPage: 2, wantRows: 0, wantTot: 3},
		{name: "a per_page of one", page: 3, perPage: 1, wantRows: 1, wantTot: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := svc.ListKeys(context.Background(), endpoint.ID(), tc.page, tc.perPage)
			if err != nil {
				t.Fatalf("ListKeys() error = %v", err)
			}
			if len(rows) != tc.wantRows {
				t.Fatalf("rows = %d, want %d", len(rows), tc.wantRows)
			}
			if total != tc.wantTot {
				t.Fatalf("total = %d, want %d", total, tc.wantTot)
			}
		})
	}
}

// TestEndpointService_UpdateKey pins the write-only rule: an omitted value keeps the
// stored credential, a supplied one replaces it, and a status may not hand-set the
// health the circuit breaker owns.
func TestEndpointService_UpdateKey(t *testing.T) {
	cases := []struct {
		name        string
		patch       KeyPatch
		wantCode    string
		wantRotated bool
		wantStatus  string
	}{
		{name: "a label change keeps the credential", patch: KeyPatch{Label: strPtr("renamed")}},
		{name: "a value change rotates the credential", patch: KeyPatch{Value: strPtr("sk-rotated")}, wantRotated: true},
		{name: "a blank value is treated as omitted", patch: KeyPatch{Value: strPtr("   ")}},
		{name: "a priority change", patch: KeyPatch{Priority: intPtr(5)}},
		{name: "disabling is allowed", patch: KeyPatch{Status: strPtr("disabled")}, wantStatus: "disabled"},
		{name: "setting the error status is refused", patch: KeyPatch{Status: strPtr("error")}, wantCode: "VALIDATION_ERROR"},
		{name: "an unknown status is refused", patch: KeyPatch{Status: strPtr("retired")}, wantCode: "VALIDATION_ERROR"},
		{name: "a zero priority is refused", patch: KeyPatch{Priority: intPtr(0)}, wantCode: "VALIDATION_ERROR"},
		{name: "a blank label is refused", patch: KeyPatch{Label: strPtr("  ")}, wantCode: "VALIDATION_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			ctx := context.Background()
			endpoint := keyedEndpoint(t, svc, "primary", "secondary")
			target := endpoint.Keys()[0]
			original := target.EncryptedValue()

			updated, err := svc.UpdateKey(ctx, endpoint.ID(), target.ID(), tc.patch)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("UpdateKey() error = %v", err)
			}
			if tc.wantRotated {
				if updated.EncryptedValue() == original {
					t.Fatal("a supplied value must replace the stored credential")
				}
				if !isSealed(updated.EncryptedValue()) {
					t.Fatalf("rotated value %q is not sealed", updated.EncryptedValue())
				}
				return
			}
			if updated.EncryptedValue() != original {
				t.Fatal("an omitted value must keep the stored credential")
			}
			if tc.wantStatus != "" && string(updated.Status()) != tc.wantStatus {
				t.Fatalf("status = %q, want %q", updated.Status(), tc.wantStatus)
			}
		})
	}
}
