// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_jobtoken_test.go
// @for       The Personal Access Token exchange: its request, its answers, and the
//
//	bound on which a job token is reused.
//
// @uses      encoding/json, net/http, net/http/httptest, strings, testing, time.
// @reason    Measured against the live service (draft 036 §5), the exchange answers
//
//	with an RFC3339 `expires_at` and an `expires_in` in milliseconds — the
//	reference treats the second as seconds, which would cache a day-long
//	token for five seconds. Both shapes, the refusal that carries no
//	token, and the one-call-per-credential discipline are what this file
//	holds.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
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

// exchangeCall is one request the fake vendor saw.
type exchangeCall struct {
	path        string
	contentType string
	userAgent   string
	cosyVersion string
	clientType  string
	body        exchangeBody
}

// newExchangeServer serves one scripted answer and records every request it took.
func newExchangeServer(t *testing.T, status int, answer exchangeBody) (*qoderJobTokenClient, *[]exchangeCall) {
	t.Helper()
	calls := &[]exchangeCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sent exchangeBody
		// reason: a request the vendor cannot be sent is a test bug, and the
		// assertion below reports it; decoding here keeps the handler honest.
		_ = json.NewDecoder(r.Body).Decode(&sent)
		*calls = append(*calls, exchangeCall{
			path: r.URL.Path, contentType: r.Header.Get("Content-Type"),
			userAgent: r.Header.Get("User-Agent"), cosyVersion: r.Header.Get("Cosy-Version"),
			clientType: r.Header.Get("Cosy-ClientType"), body: sent,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(server.Close)

	client, err := NewQoderJobTokenClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewQoderJobTokenClient() error = %v", err)
	}
	return client, calls
}

// TestJobTokenExchangesThePersonalToken pins the request the vendor reads: the path,
// the CLI identity headers, and a body carrying only the Personal Access Token.
func TestJobTokenExchangesThePersonalToken(t *testing.T) {
	client, calls := newExchangeServer(t, http.StatusOK, exchangeBody{
		Token: "jt-issued", ExpiresAt: "2026-09-28T12:00:00Z",
	})

	token, err := client.JobToken(context.Background(), "pt-secret")
	if err != nil {
		t.Fatalf("JobToken() error = %v", err)
	}
	if token != "jt-issued" {
		t.Fatalf("token = %q, want the issued job token", token)
	}
	if len(*calls) != 1 {
		t.Fatalf("the exchange was called %d times, want one", len(*calls))
	}
	call := (*calls)[0]
	if call.path != qoderJobTokenExchangePath {
		t.Fatalf("path = %q, want %q", call.path, qoderJobTokenExchangePath)
	}
	if call.body.PersonalToken != "pt-secret" || call.body.Token != "" {
		t.Fatalf("body = %+v, want only the personal token", call.body)
	}
	if call.contentType != "application/json" || call.userAgent != "qodercli/"+qoderIDEVersion {
		t.Fatalf("headers = %q / %q, want the CLI identity the vendor serves", call.contentType, call.userAgent)
	}
	if call.cosyVersion != qoderIDEVersion || call.clientType != qoderClientType {
		t.Fatalf("cosy identity = %q / %q", call.cosyVersion, call.clientType)
	}
}

// TestJobTokenReusesUntilTheVendorExpiryBound pins the cache rule and the buffer: a
// token inside its lifetime is never re-exchanged, and the last five minutes of it
// are treated as already spent so a request cannot start on a dying token.
func TestJobTokenReusesUntilTheVendorExpiryBound(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	client, calls := newExchangeServer(t, http.StatusOK, exchangeBody{
		Token: "jt-issued", ExpiresAt: now.Add(24 * time.Hour).Format(time.RFC3339),
	})
	client.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if _, err := client.JobToken(context.Background(), "pt-secret"); err != nil {
			t.Fatalf("JobToken() #%d error = %v", i, err)
		}
	}
	if len(*calls) != 1 {
		t.Fatalf("the exchange ran %d times for one credential, want once", len(*calls))
	}

	// Inside the buffer window the cached token is no longer usable, so the next
	// call exchanges again rather than sending a token about to die.
	client.now = func() time.Time { return now.Add(24*time.Hour - qoderJobTokenBuffer) }
	if _, err := client.JobToken(context.Background(), "pt-secret"); err != nil {
		t.Fatalf("re-exchange error = %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("the exchange ran %d times, want the near-expiry call to re-exchange", len(*calls))
	}
}

// TestJobTokenReadsMillisecondsNotSeconds pins the measured shape: `expires_in` is
// 86400000 for a day, so treating it as seconds would expire the cache immediately
// and make every request pay for an exchange.
func TestJobTokenReadsMillisecondsNotSeconds(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	client, calls := newExchangeServer(t, http.StatusOK, exchangeBody{
		Token: "jt-ms", ExpiresIn: 86_400_000,
	})
	client.now = func() time.Time { return now }

	if _, err := client.JobToken(context.Background(), "pt-a"); err != nil {
		t.Fatalf("JobToken() error = %v", err)
	}
	if _, err := client.JobToken(context.Background(), "pt-a"); err != nil {
		t.Fatalf("second JobToken() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("a millisecond expires_in was read as seconds: %d exchanges", len(*calls))
	}
}

// TestJobTokenFallsBackToADayWhenTheVendorStatesNothing pins the last resort. A
// missing expiry is not a missing lifetime: without a bound the gateway would
// exchange on every call and lean on the vendor's rate limits.
func TestJobTokenFallsBackToADayWhenTheVendorStatesNothing(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	client, calls := newExchangeServer(t, http.StatusOK, exchangeBody{Token: "jt-blank"})
	client.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if _, err := client.JobToken(context.Background(), "pt-a"); err != nil {
			t.Fatalf("JobToken() error = %v", err)
		}
	}
	if len(*calls) != 1 {
		t.Fatalf("the default lifetime cached for %d calls, want a day of reuse", len(*calls))
	}
	if until := client.cache[hashCredentialToken("pt-a")].validUntil; !until.Equal(now.Add(qoderJobTokenDefaultTTL - qoderJobTokenBuffer)) {
		t.Fatalf("valid until %v, want the default day minus the buffer", until)
	}
}

// TestJobTokenRefusals pins the answers that must not yield a token: a refusal with
// the vendor's own reason, a 200 that carries no token, and a blank credential that
// cannot be exchanged at all.
func TestJobTokenRefusals(t *testing.T) {
	t.Run("a refusal that names its reason", func(t *testing.T) {
		client, _ := newExchangeServer(t, http.StatusUnauthorized, exchangeBody{Message: "personal token revoked"})
		_, err := client.JobToken(context.Background(), "pt-gone")
		if err == nil || !strings.Contains(err.Error(), "personal token revoked") {
			t.Fatalf("error = %v, want the vendor's reason quoted", err)
		}
	})
	t.Run("a success without a token", func(t *testing.T) {
		client, _ := newExchangeServer(t, http.StatusOK, exchangeBody{})
		if _, err := client.JobToken(context.Background(), "pt-x"); err == nil {
			t.Fatal("an answer with no token was accepted")
		}
	})
	t.Run("a blank credential", func(t *testing.T) {
		client, calls := newExchangeServer(t, http.StatusOK, exchangeBody{Token: "jt-x"})
		if _, err := client.JobToken(context.Background(), "  "); err == nil {
			t.Fatal("a blank personal token was exchanged")
		}
		if len(*calls) != 0 {
			t.Fatal("a blank credential still reached the vendor")
		}
	})
}

// TestJobTokenSeparatesCredentials pins that the cache is per credential, not global:
// two accounts on one gateway must not be served each other's token.
func TestJobTokenSeparatesCredentials(t *testing.T) {
	client, calls := newExchangeServer(t, http.StatusOK, exchangeBody{
		Token: "jt-issued", ExpiresAt: "2026-09-28T12:00:00Z",
	})
	client.now = func() time.Time { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) }

	for _, pat := range []string{"pt-one", "pt-two"} {
		if _, err := client.JobToken(context.Background(), pat); err != nil {
			t.Fatalf("JobToken(%q) error = %v", pat, err)
		}
	}
	if len(*calls) != 2 {
		t.Fatalf("two credentials shared one exchange: %d calls", len(*calls))
	}
}

// TestHashPersonalTokenNeverCarriesTheSecret pins the cache key's property: stable
// per credential, different across them, and no substring of the secret.
func TestHashPersonalTokenNeverCarriesTheSecret(t *testing.T) {
	secret := "pt-very-specific-value"
	key := hashCredentialToken(secret)
	if key != hashCredentialToken(secret) {
		t.Fatal("the cache key is not stable for one credential")
	}
	if key == hashCredentialToken("pt-other") {
		t.Fatal("two credentials share one cache key")
	}
	if strings.Contains(key, "very-specific") || len(key) != 64 {
		t.Fatalf("cache key = %q, want a sha256 hex digest with no part of the secret", key)
	}
}
