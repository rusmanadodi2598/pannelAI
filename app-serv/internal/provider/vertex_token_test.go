// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/vertex_token_test.go
// @for       The service-account exchange: its request, its answers, and the bound on which a minted token is reused.
// @uses      context, encoding/json, net/http, net/http/httptest, net/url, testing, time.
// @reason    The exchange is the one call a service-account credential makes to Google, and its two failure modes are opposite: a malformed key must be refused before any egress, while a refused grant must carry Google's own reason rather than an opaque status. The reuse bound is pinned too, because minting per request would multiply the gateway's latency by a round trip it does not need.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// vertexExchangeCall is one request the fake token endpoint saw.
type vertexExchangeCall struct {
	contentType   string
	authorization string
	grantType     string
	assertion     string
}

// newVertexTokenServer serves one scripted answer and records every request it
// took.
func newVertexTokenServer(t *testing.T, status int, answer vertexTokenAnswer) (*vertexTokenClient, *[]vertexExchangeCall) {
	t.Helper()
	calls := &[]vertexExchangeCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// reason: a request the endpoint cannot parse is a test bug the
		// assertions below report; reading the form here keeps the handler honest.
		_ = r.ParseForm()
		*calls = append(*calls, vertexExchangeCall{
			contentType:   r.Header.Get("Content-Type"),
			authorization: r.Header.Get("Authorization"),
			grantType:     r.PostForm.Get("grant_type"),
			assertion:     r.PostForm.Get("assertion"),
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(server.Close)

	client, err := NewVertexTokenClient(server.Client())
	if err != nil {
		t.Fatalf("NewVertexTokenClient() error = %v", err)
	}
	client.tokenURL = server.URL
	return client, calls
}

// TestVertexToken_MintsOnceAndReuses pins the exchange request and the cache:
// one call per credential while the stated lifetime lasts.
func TestVertexToken_MintsOnceAndReuses(t *testing.T) {
	client, calls := newVertexTokenServer(t, http.StatusOK, vertexTokenAnswer{
		AccessToken: "ya29.test-token", ExpiresIn: 3600,
	})
	_, pemText, _ := vertexTestKey(t)
	credential := vertexTestJSON(t, pemText)

	for i := 0; i < 2; i++ {
		token, err := client.AccessToken(context.Background(), credential)
		if err != nil {
			t.Fatalf("AccessToken() call %d error = %v", i+1, err)
		}
		if token != "ya29.test-token" {
			t.Fatalf("token = %q, want the minted one", token)
		}
	}
	if len(*calls) != 1 {
		t.Fatalf("the exchange ran %d times, want once per credential", len(*calls))
	}

	call := (*calls)[0]
	if call.contentType != "application/x-www-form-urlencoded" {
		t.Fatalf("content-type = %q, want the form encoding the grant reads", call.contentType)
	}
	if call.grantType != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
		t.Fatalf("grant_type = %q", call.grantType)
	}
	if call.authorization != "" {
		t.Fatalf("authorization = %q, want the assertion in the form alone", call.authorization)
	}
	if parts := strings.Split(call.assertion, "."); len(parts) != 3 {
		t.Fatalf("the assertion has %d parts, want a signed JWT", len(parts))
	}
}

// TestVertexToken_RemintsAfterExpiry pins the reuse bound: a token inside the
// buffer is exchanged again rather than spent on a call that could outlive it.
func TestVertexToken_RemintsAfterExpiry(t *testing.T) {
	client, calls := newVertexTokenServer(t, http.StatusOK, vertexTokenAnswer{
		AccessToken: "ya29.test-token", ExpiresIn: 3600,
	})
	_, pemText, _ := vertexTestKey(t)
	credential := vertexTestJSON(t, pemText)

	base := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	client.now = func() time.Time { return base }
	if _, err := client.AccessToken(context.Background(), credential); err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}

	// One hour minus the buffer is the last usable instant, so a minute later
	// the cached token must not be handed out.
	client.now = func() time.Time { return base.Add(56 * time.Minute) }
	if _, err := client.AccessToken(context.Background(), credential); err != nil {
		t.Fatalf("AccessToken() after expiry error = %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("the exchange ran %d times, want a second mint past the buffer", len(*calls))
	}
}

// TestVertexToken_RefusesMalformedCredentialBeforeAnyCall pins that a key that
// cannot sign never reaches Google: no request is made at all.
func TestVertexToken_RefusesMalformedCredentialBeforeAnyCall(t *testing.T) {
	client, calls := newVertexTokenServer(t, http.StatusOK, vertexTokenAnswer{AccessToken: "ya29.unused"})
	_, pemText, _ := vertexTestKey(t)
	valid := vertexTestJSON(t, pemText)

	cases := []struct {
		name       string
		credential string
	}{
		{name: "empty", credential: ""},
		{name: "a raw API key", credential: "AIzaSyD-example-key-value"},
		{name: "another key type", credential: `{"type":"authorized_user","client_email":"a@b","private_key":"x"}`},
		{name: "a key with no private key", credential: `{"type":"service_account","client_email":"a@b"}`},
		{name: "a valid shape with an unreadable key", credential: replacePrivateKey(t, valid)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := client.AccessToken(context.Background(), tc.credential); err == nil {
				t.Fatal("AccessToken() = nil error, want a refusal")
			}
		})
	}
	if len(*calls) != 0 {
		t.Fatalf("the exchange ran %d times for credentials that cannot sign", len(*calls))
	}
}

// TestVertexToken_RefusalNamesGooglesReason pins that a refused grant reports
// the vendor's own reason rather than an opaque status.
func TestVertexToken_RefusalNamesGooglesReason(t *testing.T) {
	client, _ := newVertexTokenServer(t, http.StatusBadRequest, vertexTokenAnswer{
		Error: "invalid_grant", ErrorDescription: "Invalid JWT Signature.",
	})
	_, pemText, _ := vertexTestKey(t)

	_, err := client.AccessToken(context.Background(), vertexTestJSON(t, pemText))
	if err == nil {
		t.Fatal("AccessToken() = nil error, want the refusal")
	}
	if !strings.Contains(err.Error(), "Invalid JWT Signature.") {
		t.Fatalf("error = %q, want Google's own reason", err)
	}
}

// TestNewVertexTokenClient_RefusesNilClient pins the egress-guard rule: a
// defaulted client would carry no guard and no redirect policy.
func TestNewVertexTokenClient_RefusesNilClient(t *testing.T) {
	if _, err := NewVertexTokenClient(nil); err == nil {
		t.Fatal("NewVertexTokenClient(nil) = nil error, want a refusal")
	}
}

// replacePrivateKey swaps a valid credential's key for an unreadable one, so a
// credential of the right shape still fails at the parse.
func replacePrivateKey(t *testing.T, credential string) string {
	var parsed map[string]string
	// reason: the helper builds a credential the test itself wrote, so a
	// decode failure here is a test bug the caller's assertion reports.
	_ = json.Unmarshal([]byte(credential), &parsed)
	parsed["private_key"] = "not-a-pem-key"
	raw, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("the credential the test wrote could not be re-encoded: %v", err)
	}
	return string(raw)
}
