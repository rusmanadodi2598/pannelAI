// Package oauthhttp performs the OAuth rounds the flow service orchestrates.
//
// @file      internal/service/oauthhttp/oauth_client_state_test.go
// @for       The wire of the state round: the headers the vendor demands, the
//
//	queries it reads its round from, and the codes it answers with.
//
// @uses      context, io, net/http, net/http/httptest, strings, testing, time,
//
//	internal/registry.
//
// @reason    The reference's CodeBuddy module is precise about things a generic
//
//	OAuth client would guess wrong: the platform is a query parameter on the
//	state call, the round handle comes back as `state`, `X-Domain` must equal
//	the provider host, the poll is a GET with the state in the query and no
//	authorization header, and `code: 11217` means "wait" rather than "fail".
//	Each of those is an assertion here, against a real HTTP server, because a
//	fake would pass whatever shape the fake happened to invent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package oauthhttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stateRoundFixture serves one vendor's three endpoints and records what each call
// carried, so an assertion can name the header or query the reference sends.
type stateRoundFixture struct {
	server    *httptest.Server
	oauth     *registry.OAuth
	client    *OAuthHTTPClient
	stateReq  *http.Request
	pollReq   *http.Request
	refreshRe *http.Request
}

// newStateRoundFixture wires a provider whose base URL is the test server, which is
// also where X-Domain must come from.
func newStateRoundFixture(t *testing.T) *stateRoundFixture {
	t.Helper()
	fixture := &stateRoundFixture{}
	mux := http.NewServeMux()
	fixture.server = httptest.NewServer(mux)
	t.Cleanup(fixture.server.Close)

	base := fixture.server.URL
	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		fixture.stateReq = r
		writeStateEnvelope(w, `{"state":"vendor-state-1","authUrl":"`+base+`/login"}`)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("approve here"))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		fixture.pollReq = r
		writeStateEnvelope(w, `{"accessToken":"access-1","refreshToken":"refresh-1","tokenType":"Bearer","expiresIn":7200}`)
	})
	mux.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		fixture.refreshRe = r
		writeStateEnvelope(w, `{"accessToken":"access-2","refreshToken":"refresh-2","expiresIn":7200}`)
	})

	fixture.oauth = &registry.OAuth{
		BaseURL: base, StateURL: base + "/state", TokenURL: base + "/token",
		RefreshURL: base + "/refresh", Platform: "ide", UserAgent: "IDE/2.63.2 CodeBuddy/2.63.2",
		PollIntervalMS: 5000,
	}
	fixture.client = mustTokens(t, fixture.server.Client())
	return fixture
}

// writeStateEnvelope answers with the vendor's doubled envelope shape: a transport
// 200 carrying its verdict in `code`.
func writeStateEnvelope(w http.ResponseWriter, data string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"code":0,"msg":"ok","data":`+data+`}`)
}

func TestStateRoundAsksTheVendorForItsRound(t *testing.T) {
	fixture := newStateRoundFixture(t)

	round, err := fixture.client.StateRound(context.Background(), fixture.oauth)
	if err != nil {
		t.Fatalf("StateRound() error = %v", err)
	}
	if round.State != "vendor-state-1" || !strings.HasSuffix(round.AuthURL, "/login") {
		t.Fatalf("StateRound() = %+v, want the vendor's own state and login URL", round)
	}
	if round.Interval != 5*time.Second {
		t.Fatalf("StateRound() interval = %v, want the entry's 5000ms cadence", round.Interval)
	}
	if got := fixture.stateReq.URL.Query().Get("platform"); got != "ide" {
		t.Fatalf("platform query = %q, want ide: the reference puts it in the URL, not the body", got)
	}
	if fixture.stateReq.Method != http.MethodPost {
		t.Fatalf("method = %s, want POST", fixture.stateReq.Method)
	}
	assertStateHeaders(t, fixture.stateReq, map[string]string{
		"User-Agent": "IDE/2.63.2 CodeBuddy/2.63.2", "X-Product": "SaaS",
		"X-Requested-With": "XMLHttpRequest", "X-No-Authorization": "true", "X-No-User-Id": "true",
	})
	if got := fixture.stateReq.Header.Get("X-Domain"); got != strings.TrimPrefix(fixture.server.URL, "http://") {
		t.Fatalf("X-Domain = %q, want the provider host the entry declares", got)
	}
}

func TestStatePollSendsNoCredentialAndReadsTheToken(t *testing.T) {
	fixture := newStateRoundFixture(t)

	token, pending, err := fixture.client.StatePoll(context.Background(), fixture.oauth, "vendor-state-1")
	if err != nil {
		t.Fatalf("StatePoll() error = %v", err)
	}
	if pending {
		t.Fatal("StatePoll() pending = true, want false for a granted token")
	}
	if token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" {
		t.Fatalf("StatePoll() = %+v, want the vendor's token pair", token)
	}
	if life := time.Until(token.ExpiresAt); life < 119*time.Minute || life > 121*time.Minute {
		t.Fatalf("StatePoll() expiry = %v, want the vendor's 7200s counted from now", token.ExpiresAt)
	}
	if got := fixture.pollReq.URL.Query().Get("state"); got != "vendor-state-1" {
		t.Fatalf("state query = %q, want the round handle", got)
	}
	if fixture.pollReq.Method != http.MethodGet {
		t.Fatalf("method = %s, want GET: the reference polls by query parameter", fixture.pollReq.Method)
	}
	if fixture.pollReq.Header.Get("Authorization") != "" {
		t.Fatal("the poll carried an Authorization header, which the vendor's X-No-Authorization call must not")
	}
	assertStateHeaders(t, fixture.pollReq, map[string]string{
		"X-No-Enterprise-Id": "true", "X-No-Department-Info": "true",
	})
}

func TestStateRefreshCarriesTheTokenInItsHeader(t *testing.T) {
	fixture := newStateRoundFixture(t)

	token, err := fixture.client.StateRefresh(context.Background(), fixture.oauth, "refresh-1")
	if err != nil {
		t.Fatalf("StateRefresh() error = %v", err)
	}
	if token.AccessToken != "access-2" || token.RefreshToken != "refresh-2" || token.ExpiresIn != 7200 {
		t.Fatalf("StateRefresh() = %+v, want the rotated pair with its lifetime", token)
	}
	if got := fixture.refreshRe.Header.Get("X-Refresh-Token"); got != "refresh-1" {
		t.Fatalf("X-Refresh-Token = %q, want the stored token: this endpoint takes no form field", got)
	}
	if got := fixture.refreshRe.Header.Get("X-Auth-Refresh-Source"); got != "plugin" {
		t.Fatalf("X-Auth-Refresh-Source = %q, want plugin", got)
	}
}

// assertStateHeaders checks the header set the vendor reads, naming each mismatch.
func assertStateHeaders(t *testing.T, r *http.Request, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if got := r.Header.Get(key); got != value {
			t.Fatalf("header %s = %q, want %q", key, got, value)
		}
	}
}

// TestStateRoundRefusesAVendorVerdict pins the one thing a status code cannot say:
// this vendor answers HTTP 200 while refusing the request, so `code` is the verdict
// and its `msg` is the sentence the operator needs.
func TestStateRoundRefusesAVendorVerdict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":11005,"msg":"plugin session expired"}`)
	}))
	defer server.Close()

	oauth := &registry.OAuth{BaseURL: server.URL, StateURL: server.URL, TokenURL: server.URL}
	_, err := mustTokens(t, server.Client()).StateRound(context.Background(), oauth)
	if err == nil || !strings.Contains(err.Error(), "plugin session expired") {
		t.Fatalf("StateRound() error = %v, want the vendor's own refusal", err)
	}
}

// TestStatePollWaitsOnThePendingCode pins 11217 as a wait. It arrives on a 200, so
// treating it as an answer would end a round the operator has not finished, and
// treating it as a failure would report a login that is simply still open.
func TestStatePollWaitsOnThePendingCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":11217,"msg":"RetryFetchToken"}`)
	}))
	defer server.Close()

	oauth := &registry.OAuth{BaseURL: server.URL, TokenURL: server.URL}
	_, pending, err := mustTokens(t, server.Client()).StatePoll(context.Background(), oauth, "vendor-state-1")
	if err != nil {
		t.Fatalf("StatePoll() error = %v, want no error for a pending round", err)
	}
	if !pending {
		t.Fatal("StatePoll() pending = false, want true for code 11217")
	}
}

// TestStateRoundRefusesAnEmptyRound pins the failure that would otherwise store
// nothing: a round the vendor claims to have opened without naming either handle.
func TestStateRoundRefusesAnEmptyRound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"data":{}}`)
	}))
	defer server.Close()

	oauth := &registry.OAuth{BaseURL: server.URL, StateURL: server.URL}
	if _, err := mustTokens(t, server.Client()).StateRound(context.Background(), oauth); err == nil {
		t.Fatal("StateRound() = nil error, want a refusal when the vendor returns no state or URL")
	}
}
