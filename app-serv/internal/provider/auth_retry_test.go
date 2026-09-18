// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/auth_retry_test.go
// @for       Table-driven tests for credential placement, retry decisions, and
//
//	quota classification.
//
// @uses      testing, net/http, internal/registry.
// @reason    These are the three connector decisions that used to be per-provider
//
//	branches in the core: which header carries the credential, whether an
//	upstream outcome is worth retrying, and whether a rejection means the
//	account is out of quota rather than briefly unhappy. Pinning them
//	here is what makes the branch unnecessary.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package provider

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestDefault_ApplyAuth covers the rule that made a per-provider branch
// unnecessary: the header and scheme come from the registry entry, and a
// provider may read a different header per credential family.
func TestDefault_ApplyAuth(t *testing.T) {
	cases := []struct {
		name       string
		auth       registry.AuthConfig
		cred       Credential
		wantHeader string
		wantValue  string
		wantAbsent bool
		wantErr    bool
	}{
		{
			name:       "default Authorization bearer",
			auth:       registry.AuthConfig{},
			cred:       Credential{APIKey: "sk-x"},
			wantHeader: "Authorization",
			wantValue:  "Bearer sk-x",
		},
		{
			name:       "an explicit bearer scheme",
			auth:       registry.AuthConfig{Header: "Authorization", Scheme: "bearer"},
			cred:       Credential{APIKey: "sk-x"},
			wantHeader: "Authorization",
			wantValue:  "Bearer sk-x",
		},
		{
			name:       "a raw scheme sends the value untouched",
			auth:       registry.AuthConfig{Header: "x-api-key", Scheme: "raw"},
			cred:       Credential{APIKey: "sk-x"},
			wantHeader: "x-api-key",
			wantValue:  "sk-x",
		},
		{
			name:       "an empty scheme on a non-default header is treated as raw",
			auth:       registry.AuthConfig{Header: "x-api-key"},
			cred:       Credential{APIKey: "sk-x"},
			wantHeader: "x-api-key",
			wantValue:  "sk-x",
		},
		{
			name:       "a custom scheme is prefixed",
			auth:       registry.AuthConfig{Header: "Authorization", Scheme: "Token"},
			cred:       Credential{APIKey: "sk-x"},
			wantHeader: "Authorization",
			wantValue:  "Token sk-x",
		},
		{
			name:       "the per-family key header wins for a static key",
			auth:       registry.AuthConfig{Header: "Authorization", Scheme: "bearer", APIKey: &registry.AuthScheme{Header: "x-api-key", Scheme: "raw"}},
			cred:       Credential{APIKey: "sk-x"},
			wantHeader: "x-api-key",
			wantValue:  "sk-x",
		},
		{
			name:       "the per-family oauth header wins for a token",
			auth:       registry.AuthConfig{Header: "x-api-key", Scheme: "raw", OAuth: &registry.AuthScheme{Header: "Authorization", Scheme: "bearer"}},
			cred:       Credential{AccessToken: "tok"},
			wantHeader: "Authorization",
			wantValue:  "Bearer tok",
		},
		{
			name:       "an access token is preferred when both are present",
			auth:       registry.AuthConfig{Header: "Authorization", Scheme: "bearer"},
			cred:       Credential{APIKey: "sk-x", AccessToken: "tok"},
			wantHeader: "Authorization",
			wantValue:  "Bearer tok",
		},
		{
			// A zero-value Credential is how a `no_auth` provider is served, so
			// this must send nothing rather than fail: the free-tier providers
			// on the owner's list reach their upstream with no credential at
			// all, and a strict reading here would break every one of them.
			name:       "an empty credential sends nothing",
			auth:       registry.AuthConfig{},
			cred:       NoCredential("ep_1"),
			wantAbsent: true,
		},
		{
			// The configuration mistake that matters: the entry routes OAuth
			// through one header and static keys through another, so a static
			// key has no correct placement and must be refused rather than sent
			// with the wrong header.
			name: "a static key on a token-only provider is refused",
			auth: registry.AuthConfig{
				OAuth: &registry.AuthScheme{Header: "Authorization", Scheme: "bearer"},
			},
			cred:    StaticKey("ep_1", "uky_1", "sk-x"),
			wantErr: true,
		},
		{
			// The bug this type now prevents: an OAuth token must take the OAuth
			// header even when a static-key header is also declared. Before the
			// family was stated explicitly, the static-key branch ran first and
			// sent the token in x-api-key.
			name: "an oauth token takes the oauth header when both are declared",
			auth: registry.AuthConfig{
				APIKey: &registry.AuthScheme{Header: "x-api-key", Scheme: "raw"},
				OAuth:  &registry.AuthScheme{Header: "Authorization", Scheme: "bearer"},
			},
			cred:       OAuthToken("ep_1", "uky_1", "tok"),
			wantHeader: "Authorization",
			wantValue:  "Bearer tok",
		},
		{
			name:    "a token on a key-only provider is refused",
			auth:    registry.AuthConfig{APIKey: &registry.AuthScheme{Header: "x-api-key", Scheme: "raw"}},
			cred:    OAuthToken("ep_1", "uky_1", "tok"),
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := registry.Provider{ID: "p", Transport: registry.Transport{Auth: tc.auth}}
			connector := NewDefault(entry)
			req, err := http.NewRequest(http.MethodPost, "https://p.test/v1", nil)
			if err != nil {
				t.Fatalf("building the request: %v", err)
			}
			if err := connector.ApplyAuth(req, tc.cred); err != nil {
				if tc.wantErr {
					return
				}
				t.Fatalf("ApplyAuth() error = %v", err)
			}
			if tc.wantErr {
				t.Fatal("ApplyAuth() = nil error, want an error")
			}
			if tc.wantAbsent {
				if got := req.Header.Get("Authorization"); got != "" {
					t.Fatalf("Authorization = %q, want it absent", got)
				}
				return
			}
			if got := req.Header.Get(tc.wantHeader); got != tc.wantValue {
				t.Fatalf("header %s = %q, want %q", tc.wantHeader, got, tc.wantValue)
			}
		})
	}
}

// TestBase_ShouldRetryHonoursRetryAfter pins the transport default, including
// both permitted forms of the header and the case where it is absent.
func TestBase_ShouldRetryHonoursRetryAfter(t *testing.T) {
	base := Base{ID: "p"}

	cases := []struct {
		name       string
		status     int
		retryAfter string
		wantRetry  bool
		wantWait   bool
	}{
		{name: "429 retries", status: http.StatusTooManyRequests, wantRetry: true},
		{name: "502 retries", status: http.StatusBadGateway, wantRetry: true},
		{name: "503 retries", status: http.StatusServiceUnavailable, wantRetry: true},
		{name: "504 retries", status: http.StatusGatewayTimeout, wantRetry: true},
		{name: "400 does not retry", status: http.StatusBadRequest},
		{name: "401 does not retry", status: http.StatusUnauthorized},
		{name: "404 does not retry", status: http.StatusNotFound},
		{name: "429 with a seconds header carries a wait", status: http.StatusTooManyRequests, retryAfter: "30", wantRetry: true, wantWait: true},
		{name: "429 with a nonsense header still retries immediately", status: http.StatusTooManyRequests, retryAfter: "soon", wantRetry: true},
		{name: "429 with a negative header retries immediately", status: http.StatusTooManyRequests, retryAfter: "-5", wantRetry: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{}
			if tc.retryAfter != "" {
				header.Set("Retry-After", tc.retryAfter)
			}
			got := base.ShouldRetry(tc.status, header)
			if got.Retry != tc.wantRetry {
				t.Fatalf("ShouldRetry(%d).Retry = %v, want %v", tc.status, got.Retry, tc.wantRetry)
			}
			if tc.wantWait && got.After <= 0 {
				t.Fatalf("ShouldRetry(%d).After = %v, want a positive wait", tc.status, got.After)
			}
			if !tc.wantWait && got.After != 0 {
				t.Fatalf("ShouldRetry(%d).After = %v, want no wait", tc.status, got.After)
			}
		})
	}
}

func TestBase_IsQuotaError(t *testing.T) {
	base := Base{ID: "p"}
	cases := []struct {
		status int
		want   bool
	}{
		{status: http.StatusPaymentRequired, want: true},
		{status: http.StatusForbidden, want: true},
		{status: http.StatusTooManyRequests},
		{status: http.StatusUnauthorized},
		{status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			if got := base.IsQuotaError(tc.status, nil); got != tc.want {
				t.Fatalf("IsQuotaError(%d) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
