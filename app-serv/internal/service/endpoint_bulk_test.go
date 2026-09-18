// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_bulk_test.go
// @for       Table-driven tests for the two batch routes and their all-or-nothing
//
//	rule (SPEC-API-001 §7.5, §8.1).
//
// @uses      context, errors, strconv, testing, time, internal/domain.
// @reason    §8.1's rule is the one that is worst to get wrong: a batch that wrote a
//
//	prefix of its rows leaves a half-imported account list, which is harder to reason
//	about than a refusal. Every negative case here asserts that NOTHING was written,
//	not merely that an error came back.
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
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestEndpointService_BulkCreateIsAllOrNothing is the §8.1 requirement: a batch with
// one bad row writes no endpoint at all, and a valid batch writes every row.
func TestEndpointService_BulkCreateIsAllOrNothing(t *testing.T) {
	cases := []struct {
		name       string
		provider   string
		accounts   []BulkAccountInput
		wantCode   string
		wantStored int
	}{
		{
			name: "a valid batch stores every row",
			accounts: []BulkAccountInput{
				{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}},
				{Label: "two", Keys: []KeyInput{{Value: "sk-2"}}},
				{Label: "three", Keys: []KeyInput{{Value: "sk-3"}}},
			},
			wantStored: 3,
		},
		{
			name: "a duplicate label in the last row writes nothing",
			accounts: []BulkAccountInput{
				{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}},
				{Label: "two", Keys: []KeyInput{{Value: "sk-2"}}},
				{Label: "one", Keys: []KeyInput{{Value: "sk-3"}}},
			},
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name: "a missing key on an api_key row writes nothing",
			accounts: []BulkAccountInput{
				{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}},
				{Label: "two"},
			},
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name: "a blank label in the first row writes nothing",
			accounts: []BulkAccountInput{
				{Label: "  ", Keys: []KeyInput{{Value: "sk-1"}}},
				{Label: "two", Keys: []KeyInput{{Value: "sk-2"}}},
			},
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name:     "an empty batch is refused",
			accounts: nil,
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name:     "an over-large batch is refused",
			accounts: makeAccounts(51),
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name:     "an unknown provider is refused",
			provider: "nonexistent",
			accounts: []BulkAccountInput{{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}}},
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newEndpointSvc(t)
			provider := tc.provider
			if provider == "" {
				provider = "deepseek"
			}

			created, err := svc.BulkCreateAccounts(context.Background(), provider,
				domain.UpstreamAuthAPIKey, tc.accounts)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				if len(created) != 0 {
					t.Fatalf("a refused batch returned %d endpoints, want none", len(created))
				}
			} else if err != nil {
				t.Fatalf("BulkCreateAccounts() error = %v", err)
			}

			if len(store.byID) != tc.wantStored {
				t.Fatalf("stored endpoints = %d, want %d: a refused batch must write nothing",
					len(store.byID), tc.wantStored)
			}
		})
	}
}

// TestEndpointService_BulkCreateAttributesTheBadRow pins §8.1's per-row report for the
// account batch.
func TestEndpointService_BulkCreateAttributesTheBadRow(t *testing.T) {
	svc, _ := newEndpointSvc(t)

	_, err := svc.BulkCreateAccounts(context.Background(), "deepseek", domain.UpstreamAuthAPIKey,
		[]BulkAccountInput{
			{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}},
			{Label: "two", Keys: []KeyInput{{Value: "sk-2"}}},
			{Label: "  ", Keys: []KeyInput{{Value: "sk-3"}}},
		})
	mustAppError(t, err, "VALIDATION_ERROR")

	var indexed BulkRowIndexer
	if !errors.As(err, &indexed) {
		t.Fatalf("error %v does not name the offending row", err)
	}
	index, ok := indexed.BulkRowIndex()
	if !ok || index != 2 {
		t.Fatalf("BulkRowIndex() = (%d, %v), want (2, true)", index, ok)
	}
}

// TestEndpointService_BulkImportOAuth pins the import contract: both tokens are
// sealed, the response carries a hint rather than a token, a re-import of the same
// account updates in place instead of duplicating it (§8.1), and a malformed row
// writes nothing.
func TestEndpointService_BulkImportOAuth(t *testing.T) {
	const accessToken = "ya29.a-real-looking-access-token-value"

	t.Run("importing an account seals the tokens and returns a hint", func(t *testing.T) {
		svc, store := newEndpointSvc(t)
		results, err := svc.BulkImportOAuth(context.Background(), "deepseek", []OAuthAccountInput{{
			Label: "work", AccessToken: accessToken, RefreshToken: "1//refresh-value",
			Account: domain.EndpointAccount{Email: "ops@example.test", WorkspaceID: "ws_1"},
		}})
		if err != nil {
			t.Fatalf("BulkImportOAuth() error = %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("results = %d, want 1", len(results))
		}
		result := results[0]
		if result.Updated {
			t.Fatal("a first import must create rather than update")
		}
		if result.TokenHint == accessToken {
			t.Fatal("the response must carry a hint, never the token")
		}
		if result.TokenHint == "" {
			t.Fatal("the response must carry a hint the panel can show")
		}

		endpoint := result.Endpoint
		if endpoint.AuthType() != domain.UpstreamAuthOAuth {
			t.Fatalf("auth type = %q, want oauth", endpoint.AuthType())
		}
		credential := endpoint.OAuth()
		if credential == nil {
			t.Fatal("an imported endpoint must carry its credential")
		}
		if credential.AccessTokenEncrypted == accessToken || !isSealed(credential.AccessTokenEncrypted) {
			t.Fatal("the access token must be sealed before storage")
		}
		if !isSealed(credential.RefreshTokenEncrypted) {
			t.Fatal("the refresh token must be sealed before storage")
		}
		if credential.AccountEmail != "ops@example.test" {
			t.Fatalf("account email = %q, want the identity that was imported", credential.AccountEmail)
		}
		stored := store.byID[endpoint.ID()]
		if stored.OAuth().AccessTokenEncrypted != credential.AccessTokenEncrypted {
			t.Fatal("the sealed credential must be what was persisted")
		}
	})

	t.Run("a re-import of the same account updates in place", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		ctx := context.Background()
		account := OAuthAccountInput{AccessToken: accessToken,
			Account: domain.EndpointAccount{Email: "ops@example.test"}}

		first, err := svc.BulkImportOAuth(ctx, "deepseek", []OAuthAccountInput{account})
		if err != nil {
			t.Fatal(err)
		}
		rotated := account
		rotated.AccessToken = "ya29.a-rotated-token-value"
		second, err := svc.BulkImportOAuth(ctx, "deepseek", []OAuthAccountInput{rotated})
		if err != nil {
			t.Fatal(err)
		}

		if second[0].Endpoint.ID() != first[0].Endpoint.ID() {
			t.Fatalf("re-import created a second endpoint (%q then %q); §8.1 requires an update",
				first[0].Endpoint.ID(), second[0].Endpoint.ID())
		}
		if !second[0].Updated {
			t.Fatal("a re-import must report itself as an update")
		}
		after, err := svc.Get(ctx, first[0].Endpoint.ID())
		if err != nil {
			t.Fatal(err)
		}
		if after.OAuth().AccessTokenEncrypted == first[0].Endpoint.OAuth().AccessTokenEncrypted {
			t.Fatal("a re-import must replace the stored credential")
		}
	})

	t.Run("an account with no identity is created, not matched to another", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		results, err := svc.BulkImportOAuth(context.Background(), "deepseek", []OAuthAccountInput{
			{AccessToken: accessToken, Account: domain.EndpointAccount{Email: "first@example.test"}},
			{AccessToken: accessToken, Account: domain.EndpointAccount{Email: "second@example.test"}},
			{AccessToken: accessToken},
		})
		if err != nil {
			t.Fatalf("BulkImportOAuth() error = %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("results = %d, want 3", len(results))
		}
		seen := map[string]bool{}
		for _, result := range results {
			if seen[result.Endpoint.ID()] {
				t.Fatal("two rows collapsed onto one endpoint")
			}
			seen[result.Endpoint.ID()] = true
		}
	})

	t.Run("a batch with a bad row writes nothing", func(t *testing.T) {
		cases := []struct {
			name     string
			accounts []OAuthAccountInput
			wantCode string
			wantRow  int
		}{
			{
				name: "a missing access token in the second row",
				accounts: []OAuthAccountInput{
					{AccessToken: accessToken},
					{AccessToken: "   "},
				},
				wantCode: "VALIDATION_ERROR", wantRow: 1,
			},
			{
				name: "a missing access token in the last of three",
				accounts: []OAuthAccountInput{
					{AccessToken: accessToken},
					{AccessToken: "ya29.second"},
					{AccessToken: ""},
				},
				wantCode: "VALIDATION_ERROR", wantRow: 2,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc, store := newEndpointSvc(t)
				results, err := svc.BulkImportOAuth(context.Background(), "deepseek", tc.accounts)
				mustAppError(t, err, tc.wantCode)
				if len(results) != 0 {
					t.Fatalf("a refused batch returned %d rows, want none", len(results))
				}
				if len(store.byID) != 0 {
					t.Fatalf("stored endpoints = %d, want 0: a refused batch must write nothing", len(store.byID))
				}
				var indexed BulkRowIndexer
				if !errors.As(err, &indexed) {
					t.Fatalf("error %v does not name the offending row", err)
				}
				if index, _ := indexed.BulkRowIndex(); index != tc.wantRow {
					t.Fatalf("BulkRowIndex() = %d, want %d", index, tc.wantRow)
				}
			})
		}
	})

	t.Run("expiry is carried through", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		expiry := time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC)
		results, err := svc.BulkImportOAuth(context.Background(), "deepseek", []OAuthAccountInput{{
			AccessToken: accessToken, ExpiresAt: &expiry, Scopes: []string{"chat", "models"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		credential := results[0].Endpoint.OAuth()
		if credential.ExpiresAt == nil || !credential.ExpiresAt.Equal(expiry) {
			t.Fatalf("expires_at = %v, want %v", credential.ExpiresAt, expiry)
		}
		if len(credential.Scopes) != 2 {
			t.Fatalf("scopes = %v, want the two imported", credential.Scopes)
		}
	})
}

// makeAccounts builds a batch of n valid account inputs.
func makeAccounts(n int) []BulkAccountInput {
	out := make([]BulkAccountInput, 0, n)
	for i := range n {
		out = append(out, BulkAccountInput{
			Label: "acct-" + strconv.Itoa(i),
			Keys:  []KeyInput{{Value: "sk-value-" + strconv.Itoa(i)}},
		})
	}
	return out
}
