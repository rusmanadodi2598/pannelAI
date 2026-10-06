// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_bulk_import_test.go
// @for       The OAuth bulk import that turns a vendor account list into endpoints.
// @uses      testing, internal/domain.
// @reason    Import names fresh endpoints after the identity the vendor states and seals two tokens per row, which the credential-row bulk path does not do.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

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
			Account: domain.RehydrateEndpointAccount(domain.EndpointAccountInput{Email: "ops@example.test", WorkspaceID: "ws_1"}),
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
		if credential.AccessTokenEncrypted() == accessToken || !isSealed(credential.AccessTokenEncrypted()) {
			t.Fatal("the access token must be sealed before storage")
		}
		if !isSealed(credential.RefreshTokenEncrypted()) {
			t.Fatal("the refresh token must be sealed before storage")
		}
		if credential.AccountEmail().String() != "ops@example.test" {
			t.Fatalf("account email = %q, want the identity that was imported", credential.AccountEmail())
		}
		stored := store.byID[endpoint.ID()]
		if stored.OAuth().AccessTokenEncrypted() != credential.AccessTokenEncrypted() {
			t.Fatal("the sealed credential must be what was persisted")
		}
	})

	t.Run("a re-import of the same account updates in place", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		ctx := context.Background()
		account := OAuthAccountInput{AccessToken: accessToken,
			Account: domain.RehydrateEndpointAccount(domain.EndpointAccountInput{Email: "ops@example.test"})}

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
		if after.OAuth().AccessTokenEncrypted() == first[0].Endpoint.OAuth().AccessTokenEncrypted() {
			t.Fatal("a re-import must replace the stored credential")
		}
	})

	t.Run("an account with no identity is created, not matched to another", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		results, err := svc.BulkImportOAuth(context.Background(), "deepseek", []OAuthAccountInput{
			{AccessToken: accessToken, Account: domain.RehydrateEndpointAccount(domain.EndpointAccountInput{Email: "first@example.test"})},
			{AccessToken: accessToken, Account: domain.RehydrateEndpointAccount(domain.EndpointAccountInput{Email: "second@example.test"})},
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
		if credential.ExpiresAt() == nil || !credential.ExpiresAt().Equal(expiry) {
			t.Fatalf("expires_at = %v, want %v", credential.ExpiresAt(), expiry)
		}
		if len(credential.Scopes()) != 2 {
			t.Fatalf("scopes = %v, want the two imported", credential.Scopes())
		}
	})
}

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
