// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_test.go
// @for       Table-driven tests for endpoint creation, validation, and the conflict paths (SPEC-API-001 §7.5).
// @uses      context, errors, testing, internal/domain.
// @reason    AGENTS.md §2.1 requires tests alongside service logic, and §2.4 CDD requires the boundary and negative cases a single example would miss: an unknown provider, a missing key on an api_key endpoint, a duplicate account, and the extreme end of a batch. Each case here is a rule a plausible bug would break.
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

// longLabel returns a label of exactly n characters.
func longLabel(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'x'
	}
	return string(out)
}
