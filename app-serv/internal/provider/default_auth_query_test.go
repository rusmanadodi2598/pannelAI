// Package provider adapts a registry entry to the outbound HTTP call.
//
// @file      internal/provider/default_auth_query_test.go
// @for       The query-parameter credential placement, and the header placement
//
//	it must not disturb.
//
// @uses      internal/registry, net/http, net/http/httptest, strings, testing.
// @reason    Draft 017 §4.2 measured that `app-serv` could not express a provider
//
//	that carries its credential in the query string: `AuthConfig` had only
//	Header and Scheme, so the gemini family — whose models endpoint reads
//	`?key=` — had no declaration for it. The reference applies exactly this
//	rule in its models route (models/route.js:189, :634-637).
//
//	The rule is asserted against the header path too, because the two are
//	mutually exclusive in the reference: a request that sent both would
//	leak the credential into a URL and a header at once.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// authQueryRequest builds one request against an entry declaring the given auth.
func authQueryRequest(t *testing.T, auth registry.AuthConfig) *http.Request {
	t.Helper()
	entry := registry.Provider{
		ID: "gemini", Category: "apikey", AuthType: registry.AuthAPIKey,
		Transport: registry.Transport{
			Format: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta/models",
			Auth: auth,
		},
	}
	req := httptest.NewRequest(http.MethodGet, "https://generativelanguage.googleapis.com/v1beta/models", nil)
	if err := NewDefault(entry).ApplyAuth(req, StaticKey("ep_01TEST", "key_01TEST", "sk-live-abcdef")); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	return req
}

// TestApplyAuth_AuthQuery covers the placement rule and its exclusivity.
func TestApplyAuth_AuthQuery(t *testing.T) {
	const secret = "sk-live-abcdef"
	cases := []struct {
		name        string
		auth        registry.AuthConfig
		wantQuery   string
		wantHeader  string
		wantNoQuery bool
	}{
		{
			name:      "a query-param credential sets the query and no auth header",
			auth:      registry.AuthConfig{AuthQuery: "key"},
			wantQuery: "key=" + secret,
		},
		{
			name:       "a header credential sets no query",
			auth:       registry.AuthConfig{Header: "x-api-key", Scheme: "raw"},
			wantHeader: "x-api-key",
		},
		{
			name:      "the query wins when both are declared, matching the reference",
			auth:      registry.AuthConfig{AuthQuery: "key", Header: "x-goog-api-key", Scheme: "raw"},
			wantQuery: "key=" + secret,
		},
		{
			name:        "no auth declared leaves both untouched",
			auth:        registry.AuthConfig{},
			wantNoQuery: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := authQueryRequest(t, tc.auth)
			query := req.URL.RawQuery
			if tc.wantQuery != "" && !strings.Contains(query, tc.wantQuery) {
				t.Fatalf("query = %q, want it to carry %q", query, tc.wantQuery)
			}
			if tc.wantHeader != "" {
				if got := req.Header.Get(tc.wantHeader); got != secret {
					t.Fatalf("header %s = %q, want the credential", tc.wantHeader, got)
				}
			}
			if tc.auth.AuthQuery != "" {
				// A query placement must not also send an auth header: the value
				// would then be in two places, one of which is a URL.
				for _, header := range []string{"Authorization", "x-api-key", "x-goog-api-key"} {
					if got := req.Header.Get(header); got != "" {
						t.Fatalf("a query-placed credential also sent %s: %q", header, got)
					}
				}
			}
			if tc.wantNoQuery && query != "" {
				t.Fatalf("query = %q, want empty", query)
			}
		})
	}
}

// TestApplyAuth_AuthQueryWithAnEmptyCredential pins that an absent credential
// sends nothing at all rather than an empty parameter.
func TestApplyAuth_AuthQueryWithAnEmptyCredential(t *testing.T) {
	entry := registry.Provider{
		ID: "gemini", Category: "apikey", AuthType: registry.AuthNone,
		Transport: registry.Transport{Format: "gemini", Auth: registry.AuthConfig{AuthQuery: "key"}},
	}
	req := httptest.NewRequest(http.MethodGet, "https://example.test/v1/models", nil)
	if err := NewDefault(entry).ApplyAuth(req, NoCredential("ep_01TEST")); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	if got := req.URL.RawQuery; got != "" {
		t.Fatalf("query = %q, want empty for an absent credential", got)
	}
}
