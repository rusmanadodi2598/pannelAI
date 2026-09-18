// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_keys_test.go
// @for       Table-driven tests for key CRUD, the key batch, and the all-or-nothing
//
//	rule (SPEC-API-001 §7.5, §8.1).
//
// @uses      context, errors, strconv, testing, internal/domain.
// @reason    §7.5 makes a key a child of the endpoint aggregate with a rule no single
//
//	key can enforce — an api_key endpoint keeps at least one active credential — and
//	§8.1 makes a batch refuse as a whole. Both are rules a plausible bug breaks while
//	every other test still passes, which is why they are pinned here.
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

// TestEndpointService_UpdateKeyOnUnknownIDs pins the not-found paths.
func TestEndpointService_UpdateKeyOnUnknownIDs(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	ctx := context.Background()
	endpoint := keyedEndpoint(t, svc, "primary")

	cases := []struct {
		name       string
		endpointID string
		keyID      string
		wantCode   string
	}{
		{"an unknown key is not found", endpoint.ID(), "uky_absent", "NOT_FOUND"},
		{"an unknown endpoint is not found", "ep_absent", endpoint.Keys()[0].ID(), "NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateKey(ctx, tc.endpointID, tc.keyID, KeyPatch{Label: strPtr("x")})
			mustAppError(t, err, tc.wantCode)
		})
	}
}

// TestEndpointService_RemoveKeyKeepsOneActiveCredential is the §7.5 invariant: an
// api_key endpoint must always retain at least one usable credential, and the refusal
// is a CONFLICT the panel anticipates.
func TestEndpointService_RemoveKeyKeepsOneActiveCredential(t *testing.T) {
	cases := []struct {
		name      string
		authType  domain.UpstreamAuthType
		labels    []string
		removeAll bool
		wantCode  string
		wantKept  int
	}{
		{name: "an api_key endpoint with two keys may drop one", authType: domain.UpstreamAuthAPIKey,
			labels: []string{"a", "b"}, wantKept: 1},
		{name: "an api_key endpoint refuses to drop its last key", authType: domain.UpstreamAuthAPIKey,
			labels: []string{"only"}, removeAll: true, wantCode: "CONFLICT", wantKept: 1},
		{name: "an oauth endpoint may hold no keys", authType: domain.UpstreamAuthOAuth, labels: nil, wantKept: 0},
		{name: "a no_auth endpoint may hold no keys", authType: domain.UpstreamAuthNone, labels: nil, wantKept: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			ctx := context.Background()
			endpoint := seedEndpoint(t, svc, "deepseek", "acct", tc.authType, toKeyInputs(tc.labels)...)

			var err error
			if tc.removeAll {
				for len(endpoint.Keys()) > 0 {
					keys := endpoint.Keys()
					err = svc.RemoveKey(ctx, endpoint.ID(), keys[0].ID())
					if err != nil {
						break
					}
					reloaded, getErr := svc.Get(ctx, endpoint.ID())
					if getErr != nil {
						t.Fatal(getErr)
					}
					endpoint = reloaded
				}
			} else if len(tc.labels) > 0 {
				err = svc.RemoveKey(ctx, endpoint.ID(), endpoint.Keys()[0].ID())
			} else {
				err = svc.RemoveKey(ctx, endpoint.ID(), "uky_absent")
				tc.wantCode = "NOT_FOUND"
			}

			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
			} else if err != nil {
				t.Fatalf("RemoveKey() error = %v", err)
			}

			current, getErr := svc.Get(ctx, endpoint.ID())
			if getErr != nil {
				t.Fatal(getErr)
			}
			if len(current.Keys()) != tc.wantKept {
				t.Fatalf("keys = %d, want %d", len(current.Keys()), tc.wantKept)
			}
		})
	}
}

// TestEndpointService_AddKeyBatchIsAllOrNothing is the §8.1 rule: a batch with one bad
// row writes nothing at all, and a valid batch writes every row.
func TestEndpointService_AddKeyBatchIsAllOrNothing(t *testing.T) {
	cases := []struct {
		name       string
		batch      []KeyInput
		existing   []string
		wantCode   string
		wantStored int
	}{
		{
			name:       "a valid batch stores every row",
			batch:      []KeyInput{{Label: "x", Value: "sk-x"}, {Label: "y", Value: "sk-y"}, {Label: "z", Value: "sk-z"}},
			existing:   []string{"primary"},
			wantStored: 4,
		},
		{
			name:     "a duplicate label inside the batch writes nothing",
			batch:    []KeyInput{{Label: "dup", Value: "sk-a"}, {Label: "dup", Value: "sk-b"}},
			existing: []string{"primary"}, wantCode: "CONFLICT", wantStored: 1,
		},
		{
			name:     "a collision with an existing label writes nothing",
			batch:    []KeyInput{{Label: "fresh", Value: "sk-a"}, {Label: "primary", Value: "sk-b"}},
			existing: []string{"primary"}, wantCode: "CONFLICT", wantStored: 1,
		},
		{
			name:     "an empty value in the last row writes nothing",
			batch:    []KeyInput{{Label: "ok", Value: "sk-a"}, {Label: "bad", Value: "   "}},
			existing: []string{"primary"}, wantCode: "VALIDATION_ERROR", wantStored: 1,
		},
		{
			name:     "an over-long batch writes nothing",
			batch:    makeBatchKeys(101),
			existing: []string{"primary"}, wantCode: "VALIDATION_ERROR", wantStored: 1,
		},
		{
			name:     "an empty batch is refused",
			batch:    nil,
			existing: []string{"primary"}, wantCode: "VALIDATION_ERROR", wantStored: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			ctx := context.Background()
			endpoint := keyedEndpoint(t, svc, tc.existing...)

			added, err := svc.AddKeyBatch(ctx, endpoint.ID(), tc.batch)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				if len(added) != 0 {
					t.Fatalf("a refused batch returned %d rows, want none", len(added))
				}
			} else if err != nil {
				t.Fatalf("AddKeyBatch() error = %v", err)
			}

			current, getErr := svc.Get(ctx, endpoint.ID())
			if getErr != nil {
				t.Fatal(getErr)
			}
			if len(current.Keys()) != tc.wantStored {
				t.Fatalf("stored keys = %d, want %d: a refused batch must write nothing",
					len(current.Keys()), tc.wantStored)
			}
		})
	}
}

// TestEndpointService_AddKeyBatchAttributesTheBadRow pins §8.1's per-row report: the
// refusal names the index that caused it, so a client can show which row failed.
func TestEndpointService_AddKeyBatchAttributesTheBadRow(t *testing.T) {
	cases := []struct {
		name      string
		batch     []KeyInput
		wantIndex int
	}{
		{
			name:      "the offending row is the second",
			batch:     []KeyInput{{Label: "ok", Value: "sk-a"}, {Label: "", Value: "   "}},
			wantIndex: 1,
		},
		{
			name:      "the offending row is the first",
			batch:     []KeyInput{{Value: "   "}, {Label: "ok", Value: "sk-b"}},
			wantIndex: 0,
		},
		{
			name:      "the offending row is the last of four",
			batch:     []KeyInput{{Label: "a", Value: "sk-a"}, {Label: "b", Value: "sk-b"}, {Label: "c", Value: "sk-c"}, {Value: "   "}},
			wantIndex: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			endpoint := keyedEndpoint(t, svc, "primary")

			_, err := svc.AddKeyBatch(context.Background(), endpoint.ID(), tc.batch)
			mustAppError(t, err, "VALIDATION_ERROR")

			var indexed BulkRowIndexer
			if !errors.As(err, &indexed) {
				t.Fatalf("error %v does not name the offending row", err)
			}
			index, ok := indexed.BulkRowIndex()
			if !ok || index != tc.wantIndex {
				t.Fatalf("BulkRowIndex() = (%d, %v), want (%d, true)", index, ok, tc.wantIndex)
			}
		})
	}
}

// toKeyInputs turns labels into key inputs, so a table case can state just the labels.
func toKeyInputs(labels []string) []KeyInput {
	out := make([]KeyInput, 0, len(labels))
	for _, label := range labels {
		out = append(out, KeyInput{Label: label, Value: "sk-" + label})
	}
	return out
}

// makeBatchKeys builds a batch of n distinct keys.
func makeBatchKeys(n int) []KeyInput {
	out := make([]KeyInput, 0, n)
	for i := range n {
		out = append(out, KeyInput{Label: "k" + strconv.Itoa(i), Value: "sk-value-" + strconv.Itoa(i)})
	}
	return out
}

// strPtr returns a pointer to a copy of value.
func strPtr(value string) *string { return &value }

// intPtr returns a pointer to a copy of value.
func intPtr(value int) *int { return &value }
