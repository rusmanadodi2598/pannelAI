// Groq family tests: the header-only quota read, its duration-to-instant reset, the bearer
// models call, and the soft outcomes that must never touch the network.
//
// @file      internal/service/quotafetch/groq_test.go
// @for       Locks the Groq rate-limit header read: rows, reset conversion, auth, soft paths.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    Groq publishes quota only in x-ratelimit-* headers with a duration reset, so a regression that reads the body or the wrong host would silently blank the card.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroq_ReadsBothRateLimitWindowsFromHeaders(t *testing.T) {
	var calls atomic.Int64
	var method, path, auth atomic.Value
	method.Store("")
	path.Store("")
	auth.Store("")
	before := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		method.Store(r.Method)
		path.Store(r.URL.Path)
		auth.Store(r.Header.Get("Authorization"))
		w.Header().Set("x-ratelimit-limit-requests", "200")
		w.Header().Set("x-ratelimit-remaining-requests", "34")
		w.Header().Set("x-ratelimit-reset-requests", "2m59.56s")
		w.Header().Set("x-ratelimit-limit-tokens", "7000")
		w.Header().Set("x-ratelimit-remaining-tokens", "6000")
		w.Header().Set("x-ratelimit-reset-tokens", "7.66s")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	result := fetchGroq(context.Background(), Credentials{APIKey: "gk-1", Endpoint: server.URL})
	after := time.Now()

	if calls.Load() != 1 {
		t.Fatalf("outbound calls = %d, want exactly 1", calls.Load())
	}
	if got := method.Load().(string); got != http.MethodGet {
		t.Errorf("method = %q, want GET", got)
	}
	if got := path.Load().(string); got != "/openai/v1/models" {
		t.Errorf("path = %q, want /openai/v1/models", got)
	}
	if got := auth.Load().(string); got != "Bearer gk-1" {
		t.Errorf("Authorization = %q, want Bearer gk-1", got)
	}
	if result.Plan != "Groq" || result.Message != "" {
		t.Fatalf("plan/message = %q/%q", result.Plan, result.Message)
	}

	requests, ok := groqRow(result.Quotas, "Requests")
	if !ok {
		t.Fatalf("no Requests row in %+v", result.Quotas)
	}
	if requests.Used != 166 || requests.Total != 200 || requests.Unit != "requests" || !requests.Recurring {
		t.Fatalf("Requests = %+v, want used 166/total 200 unit requests recurring", requests)
	}
	if !groqWithin(requests.ResetAt, before.Add(170*time.Second), after.Add(185*time.Second)) {
		t.Fatalf("Requests reset = %v, want ~now+2m59.56s", requests.ResetAt)
	}

	tokens, ok := groqRow(result.Quotas, "Tokens")
	if !ok {
		t.Fatalf("no Tokens row in %+v", result.Quotas)
	}
	if tokens.Used != 1000 || tokens.Total != 7000 || tokens.Unit != "tokens" {
		t.Fatalf("Tokens = %+v, want used 1000/total 7000 unit tokens", tokens)
	}
	if !groqWithin(tokens.ResetAt, before.Add(5*time.Second), after.Add(10*time.Second)) {
		t.Fatalf("Tokens reset = %v, want ~now+7.66s", tokens.ResetAt)
	}
}

// A malformed reset header must not blank the window it belongs to: the counts still parse
// and only the reset instant is dropped.
func TestGroq_MalformedOrMissingResetStillYieldsRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-requests", "100")
		w.Header().Set("x-ratelimit-remaining-requests", "100")
		w.Header().Set("x-ratelimit-reset-requests", "soon-ish")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	result := fetchGroq(context.Background(), Credentials{APIKey: "gk", Endpoint: server.URL})
	requests, ok := groqRow(result.Quotas, "Requests")
	if !ok {
		t.Fatalf("malformed reset dropped the row: %+v", result)
	}
	if requests.Used != 0 || requests.Total != 100 || !requests.ResetAt.IsZero() {
		t.Fatalf("Requests = %+v, want counts kept and reset zero", requests)
	}
	if _, present := groqRow(result.Quotas, "Tokens"); present {
		t.Error("a Tokens window was invented from absent headers")
	}
}

func TestGroq_SoftOutcomesStaySoft(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		headers     map[string]string
		credentials Credentials
		wantCalls   int64
		wantMessage string
	}{
		{
			name:        "a missing key never calls the endpoint",
			credentials: Credentials{},
			wantCalls:   0,
			wantMessage: "Groq API key not available. Add a key to view usage.",
		},
		{
			name:        "a refused key names the credential",
			status:      http.StatusUnauthorized,
			credentials: Credentials{APIKey: "gk"},
			wantCalls:   1,
			wantMessage: "Groq credential invalid or expired (401).",
		},
		{
			name:        "a server error carries the provider body",
			status:      http.StatusInternalServerError,
			body:        "boom",
			credentials: Credentials{APIKey: "gk"},
			wantCalls:   1,
			wantMessage: "Groq quota API error (500): boom",
		},
		{
			name:        "a good read without rate-limit data is a soft note",
			status:      http.StatusOK,
			body:        `{"data":[]}`,
			credentials: Credentials{APIKey: "gk"},
			wantCalls:   1,
			wantMessage: "Groq connected. No rate-limit data reported for this key yet.",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				for key, value := range testCase.headers {
					w.Header().Set(key, value)
				}
				if testCase.status != 0 {
					w.WriteHeader(testCase.status)
				}
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()

			credentials := testCase.credentials
			credentials.Endpoint = server.URL
			result := fetchGroq(context.Background(), credentials)

			if calls.Load() != testCase.wantCalls {
				t.Fatalf("outbound calls = %d, want %d", calls.Load(), testCase.wantCalls)
			}
			if result.Message != testCase.wantMessage {
				t.Fatalf("message = %q, want %q", result.Message, testCase.wantMessage)
			}
		})
	}
}

// The declared registry URL wins over the built-in, and the built-in path stays the fallback
// when nothing is declared, the same single source the other families follow.
func TestGroq_ReadsItsPathFromTheDeclaredURLOrTheBuiltIn(t *testing.T) {
	cases := []struct {
		name     string
		creds    Credentials
		wantPath string
	}{
		{
			name:     "the built-in path when nothing is declared",
			creds:    Credentials{APIKey: "gk"},
			wantPath: "/openai/v1/models",
		},
		{
			name:     "the declared usage URL wins",
			creds:    Credentials{APIKey: "gk", Endpoints: UsageEndpoints{URL: "https://example.invalid/openai/v2/models"}},
			wantPath: "/openai/v2/models",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var path atomic.Value
			path.Store("")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path.Store(r.URL.Path)
				w.Header().Set("x-ratelimit-limit-requests", "1")
				w.Header().Set("x-ratelimit-remaining-requests", "1")
			}))
			defer server.Close()

			credentials := testCase.creds
			if credentials.Endpoints.URL != "" {
				credentials.Endpoints.URL = server.URL + "/openai/v2/models"
			} else {
				credentials.Endpoint = server.URL
			}
			fetchGroq(context.Background(), credentials)
			if got := path.Load().(string); got != testCase.wantPath {
				t.Fatalf("requested path = %q, want %q", got, testCase.wantPath)
			}
		})
	}
}

func groqRow(quotas []Quota, label string) (Quota, bool) {
	for _, quota := range quotas {
		if quota.Label == label {
			return quota, true
		}
	}
	return Quota{}, false
}

// groqWithin reports whether an instant falls inside [lo, hi], used to assert a reset that is
// now+duration without pinning the exact wall-clock second the read happened.
func groqWithin(instant, lo, hi time.Time) bool {
	return !instant.Before(lo) && !instant.After(hi)
}
