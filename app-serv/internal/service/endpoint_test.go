// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_test.go
// @for       Table-driven tests for endpoint creation, validation, and the
//
//	conflict paths (SPEC-API-001 §7.5).
//
// @uses      context, errors, testing, internal/domain.
// @reason    AGENTS.md §2.1 requires tests alongside service logic, and §2.4 CDD
//
//	requires the boundary and negative cases a single example would miss:
//	an unknown provider, a missing key on an api_key endpoint, a duplicate
//	account, and the extreme end of a batch. Each case here is a rule a
//	plausible bug would break.
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

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// newEndpointSvc builds the standard test service with a working in-memory store.
func newEndpointSvc(t *testing.T, providers ...string) (*EndpointService, *memEndpointStore) {
	t.Helper()
	if len(providers) == 0 {
		providers = []string{"deepseek"}
	}
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex(providers...), newTestSealer(t), nil)
	return svc, store
}

// TestEndpointService_Create pins the create contract: a known provider produces an
// endpoint whose credential is sealed and whose hint reveals only the tail, and every
// malformed input is refused before a write.
func TestEndpointService_Create(t *testing.T) {
	cases := []struct {
		name     string
		in       CreateInput
		wantCode string
		wantKeys int
	}{
		{
			name: "an api_key endpoint with one key",
			in: CreateInput{ProviderID: "deepseek", Label: "primary", AuthType: domain.UpstreamAuthAPIKey,
				Keys: []KeyInput{{Value: "sk-live-abcd1234"}}},
			wantKeys: 1,
		},
		{
			name: "several keys with explicit priorities",
			in: CreateInput{ProviderID: "deepseek", Label: "multi", AuthType: domain.UpstreamAuthAPIKey,
				Keys: []KeyInput{{Label: "one", Value: "sk-a", Priority: 2}, {Label: "two", Value: "sk-b", Priority: 1}}},
			wantKeys: 2,
		},
		{
			name: "an oauth endpoint needs no key",
			in:   CreateInput{ProviderID: "deepseek", Label: "flow", AuthType: domain.UpstreamAuthOAuth},
		},
		{
			name: "a no_auth endpoint needs no key",
			in:   CreateInput{ProviderID: "deepseek", Label: "free", AuthType: domain.UpstreamAuthNone},
		},
		{
			name:     "an unknown provider is refused",
			in:       CreateInput{ProviderID: "nonexistent", Label: "ghost", AuthType: domain.UpstreamAuthOAuth},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "an api_key endpoint without a key is refused",
			in:       CreateInput{ProviderID: "deepseek", Label: "keyless", AuthType: domain.UpstreamAuthAPIKey},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "a blank provider is refused",
			in:       CreateInput{ProviderID: "   ", Label: "blank", AuthType: domain.UpstreamAuthOAuth},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "a blank label is refused",
			in:       CreateInput{ProviderID: "deepseek", Label: "  ", AuthType: domain.UpstreamAuthOAuth},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "a label past the document bound is refused",
			in:       CreateInput{ProviderID: "deepseek", Label: longLabel(121), AuthType: domain.UpstreamAuthOAuth},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name: "an empty key value is refused",
			in: CreateInput{ProviderID: "deepseek", Label: "empty-value", AuthType: domain.UpstreamAuthAPIKey,
				Keys: []KeyInput{{Value: "   "}}},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name: "the same label twice inside one request is refused",
			in: CreateInput{ProviderID: "deepseek", Label: "dup-keys", AuthType: domain.UpstreamAuthAPIKey,
				Keys: []KeyInput{{Label: "same", Value: "sk-a"}, {Label: "same", Value: "sk-b"}}},
			wantCode: "CONFLICT",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			endpoint, err := svc.Create(context.Background(), tc.in)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if len(endpoint.Keys()) != tc.wantKeys {
				t.Fatalf("keys = %d, want %d", len(endpoint.Keys()), tc.wantKeys)
			}
			if endpoint.Status() != domain.UpstreamEndpointActive {
				t.Fatalf("status = %q, want active", endpoint.Status())
			}
			for _, key := range endpoint.Keys() {
				if key.EncryptedValue() == key.Hint() {
					t.Fatal("the sealed value must not equal the hint")
				}
				if !isSealed(key.EncryptedValue()) {
					t.Fatalf("stored value %q is not sealed ciphertext", key.EncryptedValue())
				}
			}
		})
	}
}

// TestEndpointService_CreateSealsTheCredential asserts the plaintext never reaches
// the aggregate and cannot be recovered from what is stored — the boundary
// SPEC-API-001 §6 draws.
func TestEndpointService_CreateSealsTheCredential(t *testing.T) {
	const secret = "sk-super-secret-value-do-not-store"
	svc, store := newEndpointSvc(t)

	endpoint, err := svc.Create(context.Background(), CreateInput{
		ProviderID: "deepseek", Label: "who", AuthType: domain.UpstreamAuthAPIKey,
		Keys: []KeyInput{{Value: secret}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	key := endpoint.Keys()[0]
	if key.EncryptedValue() == secret {
		t.Fatal("the stored value must never be the plaintext")
	}
	if !isSealed(key.EncryptedValue()) {
		t.Fatalf("stored value = %q, want v1: ciphertext", key.EncryptedValue())
	}
	// The hint reveals the family and the tail only; the body must be absent.
	if key.Hint() == secret {
		t.Fatal("the hint must not be the plaintext")
	}
	stored, ok := store.byID[endpoint.ID()]
	if !ok {
		t.Fatal("the endpoint was not stored")
	}
	if stored.Keys()[0].EncryptedValue() != key.EncryptedValue() {
		t.Fatal("the stored aggregate must carry the sealed value")
	}
}

// TestEndpointService_CreateRejectsDuplicateAccount pins the (provider_id, label)
// rule whose database expression is a unique index, so a caller gets CONFLICT rather
// than a driver message.
func TestEndpointService_CreateRejectsDuplicateAccount(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	in := CreateInput{ProviderID: "deepseek", Label: "primary", AuthType: domain.UpstreamAuthOAuth}

	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	_, err := svc.Create(context.Background(), in)
	mustAppError(t, err, "CONFLICT")
	if !errors.Is(err, domain.ErrEndpointExists) {
		t.Fatalf("error = %v, want it to wrap ErrEndpointExists", err)
	}

	// A different provider may reuse the label: the rule is per provider.
	other, _ := newEndpointSvc(t, "deepseek", "openrouter")
	if _, err := other.Create(context.Background(), CreateInput{
		ProviderID: "openrouter", Label: "primary", AuthType: domain.UpstreamAuthOAuth,
	}); err != nil {
		t.Fatalf("Create() on a second provider error = %v", err)
	}
}

// TestEndpointService_List pins the filter and pagination contract: a filter matches
// what it names, a page past the end reports the total without rows, and an invalid
// status is refused rather than silently matching nothing.
func TestEndpointService_List(t *testing.T) {
	svc, _ := newEndpointSvc(t, "deepseek", "openrouter")
	ctx := context.Background()
	seedEndpoint(t, svc, "deepseek", "one", domain.UpstreamAuthOAuth)
	if _, err := svc.Create(ctx, CreateInput{ProviderID: "deepseek", Label: "two", AuthType: domain.UpstreamAuthOAuth}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, CreateInput{ProviderID: "openrouter", Label: "three", AuthType: domain.UpstreamAuthOAuth}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		provider  string
		status    string
		page      int
		perPage   int
		wantRows  int
		wantTotal int64
		wantCode  string
	}{
		{name: "no filter lists every page row", page: 1, perPage: 10, wantRows: 3, wantTotal: 3},
		{name: "the provider filter narrows", provider: "deepseek", page: 1, perPage: 10, wantRows: 2, wantTotal: 2},
		{name: "an unknown provider matches nothing", provider: "absent", page: 1, perPage: 10, wantRows: 0, wantTotal: 0},
		{name: "the status filter narrows", status: "active", page: 1, perPage: 10, wantRows: 3, wantTotal: 3},
		{name: "a disabled filter matches nothing", status: "disabled", page: 1, perPage: 10, wantRows: 0, wantTotal: 0},
		{name: "a page past the end still reports the total", page: 9, perPage: 10, wantRows: 0, wantTotal: 3},
		{name: "the first of several pages", page: 1, perPage: 2, wantRows: 2, wantTotal: 3},
		{name: "the last partial page", page: 2, perPage: 2, wantRows: 1, wantTotal: 3},
		{name: "an unknown status is a validation failure", status: "retired", page: 1, perPage: 10,
			wantCode: "VALIDATION_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := svc.List(ctx, tc.provider, tc.status, tc.page, tc.perPage)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(rows) != tc.wantRows {
				t.Fatalf("rows = %d, want %d", len(rows), tc.wantRows)
			}
			if total != tc.wantTotal {
				t.Fatalf("total = %d, want %d", total, tc.wantTotal)
			}
		})
	}
}

// TestEndpointService_Update pins the PATCH contract: an omitted field keeps its
// value, an invalid one is refused, and a priority change renumbers the provider's
// siblings so no two endpoints share a slot (§7.5).
func TestEndpointService_Update(t *testing.T) {
	t.Run("partial patches leave omitted fields alone", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		ctx := context.Background()
		endpoint, err := svc.Create(ctx, CreateInput{ProviderID: "deepseek", Label: "keep", AuthType: domain.UpstreamAuthOAuth, Priority: 3})
		if err != nil {
			t.Fatal(err)
		}

		label := "renamed"
		updated, err := svc.Update(ctx, endpoint.ID(), UpdatePatch{Label: &label})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.Label() != "renamed" {
			t.Fatalf("label = %q, want renamed", updated.Label())
		}
		if updated.Priority() != 3 {
			t.Fatalf("priority = %d, want the omitted field untouched at 3", updated.Priority())
		}

		status := "disabled"
		updated, err = svc.Update(ctx, endpoint.ID(), UpdatePatch{Status: &status})
		if err != nil {
			t.Fatalf("Update(status) error = %v", err)
		}
		if updated.Status() != domain.UpstreamEndpointDisabled {
			t.Fatalf("status = %q, want disabled", updated.Status())
		}
		if updated.Label() != "renamed" {
			t.Fatalf("label = %q, want the omitted field untouched", updated.Label())
		}
	})

	t.Run("invalid patches are refused", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		ctx := context.Background()
		endpoint, err := svc.Create(ctx, CreateInput{ProviderID: "deepseek", Label: "bad", AuthType: domain.UpstreamAuthOAuth})
		if err != nil {
			t.Fatal(err)
		}

		blank, zero, retired := "  ", 0, "retired"
		cases := []struct {
			name     string
			patch    UpdatePatch
			wantCode string
		}{
			{"blank label", UpdatePatch{Label: &blank}, "VALIDATION_ERROR"},
			{"zero priority", UpdatePatch{Priority: &zero}, "VALIDATION_ERROR"},
			{"unknown status", UpdatePatch{Status: &retired}, "VALIDATION_ERROR"},
			{"empty patch is a no-op", UpdatePatch{}, ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := svc.Update(ctx, endpoint.ID(), tc.patch)
				if tc.wantCode == "" {
					if err != nil {
						t.Fatalf("Update() error = %v", err)
					}
					return
				}
				mustAppError(t, err, tc.wantCode)
			})
		}
	})

	t.Run("unknown id is not found", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		label := "x"
		_, err := svc.Update(context.Background(), "ep_missing", UpdatePatch{Label: &label})
		mustAppError(t, err, "NOT_FOUND")
	})
}

// TestEndpointService_UpdateRenumbersSiblings pins the rule §7.5 states: moving one
// endpoint's priority reorders the provider's set transactionally, so the moved
// endpoint lands where the operator put it and no two share a slot.
func TestEndpointService_UpdateRenumbersSiblings(t *testing.T) {
	cases := []struct {
		name        string
		moveLabel   string
		newPriority int
		wantOrder   []string
	}{
		{name: "move the third to the front", moveLabel: "c", newPriority: 1, wantOrder: []string{"c", "a", "b"}},
		{name: "move the first to the middle", moveLabel: "a", newPriority: 2, wantOrder: []string{"b", "a", "c"}},
		{name: "move the first to the end", moveLabel: "a", newPriority: 3, wantOrder: []string{"b", "c", "a"}},
		{name: "a priority past the end clamps to last", moveLabel: "a", newPriority: 99, wantOrder: []string{"b", "c", "a"}},
		{name: "an unchanged priority keeps the order", moveLabel: "b", newPriority: 2, wantOrder: []string{"a", "b", "c"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			ctx := context.Background()

			ids := map[string]string{}
			for i, label := range []string{"a", "b", "c"} {
				endpoint, err := svc.Create(ctx, CreateInput{
					ProviderID: "deepseek", Label: label, AuthType: domain.UpstreamAuthOAuth, Priority: i + 1,
				})
				if err != nil {
					t.Fatal(err)
				}
				ids[label] = endpoint.ID()
			}

			if _, err := svc.Update(ctx, ids[tc.moveLabel], UpdatePatch{Priority: &tc.newPriority}); err != nil {
				t.Fatalf("Update() error = %v", err)
			}

			ordered, _, err := svc.List(ctx, "deepseek", "", 1, 10)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			got := make([]string, 0, len(ordered))
			for _, endpoint := range ordered {
				got = append(got, endpoint.Label())
			}
			// The comparison is over the set of priorities, not the stored order,
			// because the stub's List is map-backed: what the rule promises is that
			// each endpoint holds a distinct slot and the moved one holds its new
			// position.
			for i, label := range tc.wantOrder {
				want := i + 1
				if priorityOf(t, ordered, ids[label]) != want {
					t.Fatalf("label %q has priority %d, want %d (order: %v)",
						label, priorityOf(t, ordered, ids[label]), want, got)
				}
			}
		})
	}
}

// TestEndpointService_Delete pins delete and its not-found path.
func TestEndpointService_Delete(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	ctx := context.Background()
	endpoint, err := svc.Create(ctx, CreateInput{ProviderID: "deepseek", Label: "gone", AuthType: domain.UpstreamAuthOAuth})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		id       string
		wantCode string
	}{
		{"an existing endpoint is deleted", endpoint.ID(), ""},
		{"a second delete is not found", endpoint.ID(), "NOT_FOUND"},
		{"an unknown id is not found", "ep_absent", "NOT_FOUND"},
		{"a blank id is a validation failure", "  ", "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.Delete(ctx, tc.id)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				return
			}
			mustAppError(t, err, tc.wantCode)
		})
	}
}

// priorityOf reports the priority an endpoint holds inside a listed page.
func priorityOf(t *testing.T, page []domain.UpstreamEndpoint, id string) int {
	t.Helper()
	for _, endpoint := range page {
		if endpoint.ID() == id {
			return endpoint.Priority()
		}
	}
	t.Fatalf("endpoint %q is absent from the page", id)
	return 0
}

// longLabel returns a label of exactly n characters.
func longLabel(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'x'
	}
	return string(out)
}
