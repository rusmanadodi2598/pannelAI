//go:build integration && live

// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_live_test.go
// @for       The live fixtures of Qoder's credential path, and the proof that a Personal Access Token becomes a job token that reads an account.
// @uses      bytes, context, encoding/json, io, net/http, os, strings, testing, time.
// @reason    A parser written against a documented shape is a guess until the service answers. This file asks the real service the two questions the exchange and the identity read depend on, and keeps the fixtures the signed cases beside it reuse. It carries the `integration,live` tags because it spends a credential and reaches outside the process; with the tag active and no token set it fails rather than passing quietly.
//
//	PANNELAI_QODER_PAT='pt-…' \
//	  go test -tags=integration,live ./internal/provider/ -run QoderLive
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// The live proof's own addresses. The token prefixes, the inference hosts and the
// exchange path are the connector's constants, read from the production file so a
// probe cannot pass against a URL the gateway would never call.
const (
	qoderLiveOpenAPI       = "https://openapi.qoder.sh"
	qoderLiveUserInfoPath  = "/api/v1/userinfo"
	qoderLiveModelListPath = "/algo/api/v2/model/list"
	qoderLiveCLIClient     = "qodercli/1.0.0"
)

const (
	qoderLiveTimeout        = 25 * time.Second
	qoderLiveBodyReadLimit  = 4 << 20
	qoderLiveRefusalPreview = 300
)

// qoderLiveJobToken is the exchange answer, kept to the fields the gateway reads.
type qoderLiveJobToken struct {
	Token      string `json:"token"`
	RefreshTok string `json:"refresh_token"`
	ExpiresAt  string `json:"expires_at"`
	ExpiresIn  int64  `json:"expires_in"`
}

// qoderLiveIdentity is the userinfo answer.
type qoderLiveIdentity struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Organization string `json:"organization_id"`
}

// qoderLivePAT reads the token this proof runs on, or fails the test: a tagged run
// that passed quietly without a credential would prove nothing.
func qoderLivePAT(t *testing.T) string {
	t.Helper()
	pat := strings.TrimSpace(os.Getenv("PANNELAI_QODER_PAT"))
	if pat == "" {
		t.Fatal("PANNELAI_QODER_PAT must be set to run the live Qoder proof")
	}
	if !strings.HasPrefix(pat, qoderTokenPAT) {
		t.Fatalf("the token must be a Personal Access Token (%s…), got one prefixed %q",
			qoderTokenPAT, truncateForLive(pat, 3))
	}
	return pat
}

// qoderLiveClient sends one request with the timeout the vendor's own client uses.
func qoderLiveClient(t *testing.T, request *http.Request) (*http.Response, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), qoderLiveTimeout)
	defer cancel()

	response, err := http.DefaultClient.Do(request.WithContext(ctx))
	if err != nil {
		t.Fatalf("sending %s: %v", request.URL.String(), err)
	}
	defer func() {
		// reason: the body is read in full here and handed to the caller.
		_ = response.Body.Close()
	}()
	body, err := io.ReadAll(io.LimitReader(response.Body, qoderLiveBodyReadLimit))
	if err != nil {
		t.Fatalf("reading the answer from %s: %v", request.URL.Path, err)
	}
	return response, body
}

// qoderLiveExchangeRequest builds the one POST a Personal Access Token must pass
// through before it can sign anything: plain JSON, the CLI's own headers, and no
// COSY signature, a PAT cannot sign, which is the whole reason the exchange exists.
func qoderLiveExchangeRequest(t *testing.T, pat string) *http.Request {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"personal_token": pat})
	if err != nil {
		t.Fatalf("encoding the exchange body: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, qoderLiveOpenAPI+qoderJobTokenExchangePath, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("building the exchange request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", qoderLiveCLIClient)
	request.Header.Set("Cosy-Version", qoderIDEVersion)
	request.Header.Set("Cosy-ClientType", qoderClientType)
	return request
}

// qoderLiveExchange runs the exchange and hands back the job token, so the cases that
// need an account do not each spend their own.
func qoderLiveExchange(t *testing.T, pat string) string {
	t.Helper()
	response, body := qoderLiveClient(t, qoderLiveExchangeRequest(t, pat))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("exchange = HTTP %d: %s", response.StatusCode,
			truncateForLive(string(body), qoderLiveRefusalPreview))
	}
	var answer qoderLiveJobToken
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("decoding the exchange answer: %v (%s)", err,
			truncateForLive(string(body), qoderLiveRefusalPreview))
	}
	if answer.Token == "" {
		t.Fatal("the exchange answered without a token")
	}
	return answer.Token
}

// qoderLiveAccount reads the identity a job token belongs to.
func qoderLiveAccount(t *testing.T, jobToken string) qoderLiveIdentity {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, qoderLiveOpenAPI+qoderLiveUserInfoPath, nil)
	if err != nil {
		t.Fatalf("building the identity request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+jobToken)
	request.Header.Set("Accept", "application/json")

	response, body := qoderLiveClient(t, request)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("userinfo = HTTP %d: %s", response.StatusCode,
			truncateForLive(string(body), qoderLiveRefusalPreview))
	}
	var account qoderLiveIdentity
	if err := json.Unmarshal(body, &account); err != nil {
		t.Fatalf("decoding the identity: %v", err)
	}
	return account
}

// TestQoderLivePATExchangesForAJobToken proves the exchange shape the gateway's
// parser will be written against: a `jt-` token, an RFC3339 `expires_at`, and an
// `expires_in` counted in milliseconds.
func TestQoderLivePATExchangesForAJobToken(t *testing.T) {
	response, body := qoderLiveClient(t, qoderLiveExchangeRequest(t, qoderLivePAT(t)))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("exchange = HTTP %d: %s", response.StatusCode,
			truncateForLive(string(body), qoderLiveRefusalPreview))
	}

	var answer qoderLiveJobToken
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("decoding the exchange answer: %v (%s)", err,
			truncateForLive(string(body), qoderLiveRefusalPreview))
	}
	if !strings.HasPrefix(answer.Token, qoderTokenJob) {
		t.Fatalf("token = %q…, want the %s prefix", truncateForLive(answer.Token, 6), qoderTokenJob)
	}
	if _, err := time.Parse(time.RFC3339, answer.ExpiresAt); err != nil {
		t.Fatalf("expires_at = %q, want an RFC3339 instant the parser can read: %v", answer.ExpiresAt, err)
	}
	// The reference adds this value to a millisecond clock without converting it,
	// which would store a five-second lifetime where the vendor means a day.
	if answer.ExpiresIn < 60_000 || answer.ExpiresIn > 86_400_000 {
		t.Fatalf("expires_in = %d, want a lifetime in milliseconds", answer.ExpiresIn)
	}
	t.Logf("job token: expires_in=%d ms, expires_at=%s, refresh token present: %v",
		answer.ExpiresIn, answer.ExpiresAt, answer.RefreshTok != "")
}

// TestQoderLiveJobTokenReadsItsAccount proves the identity both connect paths
// consult: the userinfo endpoint answers with the id a COSY signature signs as, and
// answers it to a plain bearer header rather than to a signed request.
func TestQoderLiveJobTokenReadsItsAccount(t *testing.T) {
	account := qoderLiveAccount(t, qoderLiveExchange(t, qoderLivePAT(t)))

	if account.ID == "" {
		t.Fatal("the identity carries no id, so nothing can be signed as it")
	}
	t.Logf("id present, name=%q, email present=%v, organization=%q",
		account.Name, account.Email != "", account.Organization)
}

func truncateForLive(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
