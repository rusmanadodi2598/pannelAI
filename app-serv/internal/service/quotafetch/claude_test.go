// Claude family tests: the percent windows in the card's order, the OAuth beta header the
// endpoint demands, the cooldown a 429 answers with, and the organization fallback.
//
// @file      internal/service/quotafetch/claude_test.go
// @for       Locks the Claude usage read: row order, headers, the 429 cooldown and the legacy fallback.
// @uses      internal/service/quotafetch, io, net/http, net/http/httptest, sync, testing, time
// @reason    This card is rendered from the row order it is handed, its usage endpoint rate-limits independently of chat, and its fallback changes host, so a read that keeps the labels and loses any of those three still looks fine.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const claudeUsageBody = `{
	"five_hour":{"utilization":37,"resets_at":"2026-10-02T18:00:00Z"},
	"seven_day":{"utilization":80,"resets_at":"2026-10-05T18:00:00Z"},
	"seven_day_sonnet":{"utilization":45,"resets_at":null},
	"seven_day_opus":{"utilization":12,"resets_at":"2026-10-04T00:00:00Z"},
	"limits":[{"kind":"weekly_scoped","percent":64,"resets_at":"2026-10-06T00:00:00Z",
		"scope":{"model":{"display_name":" Fable "}}},
		{"kind":"monthly_scoped","percent":10,"scope":{"model":{"display_name":"Ignored"}}}]
}`

const claudeSettingsBody = `{"plan":"claude_max","organization_id":"org-7","organization_name":"Acme"}`

// claudeStub answers every path a Claude read can reach and records what it was asked. The
// record is mutex-guarded because the -race gate treats an unsynchronised handler write as
// the bug it is.
type claudeStub struct {
	server  *httptest.Server
	replies map[string]claudeReply

	mu      sync.Mutex
	calls   []string
	headers http.Header
}

type claudeReply struct {
	status  int
	body    string
	headers map[string]string
}

func newClaudeStub(t *testing.T, replies map[string]claudeReply) *claudeStub {
	t.Helper()
	stub := &claudeStub{replies: replies}
	stub.server = httptest.NewServer(http.HandlerFunc(stub.handle))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *claudeStub) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.calls = append(s.calls, r.Method+" "+r.URL.Path)
	if len(s.calls) == 1 {
		s.headers = r.Header.Clone()
	}
	reply, known := s.replies[r.URL.Path]
	s.mu.Unlock()

	if !known {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "unexpected path "+r.URL.Path)
		return
	}
	for key, value := range reply.headers {
		w.Header().Set(key, value)
	}
	if reply.status != 0 {
		w.WriteHeader(reply.status)
	}
	_, _ = io.WriteString(w, reply.body)
}

func (s *claudeStub) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func claudeUsageReplies(body string) map[string]claudeReply {
	return map[string]claudeReply{"/api/oauth/usage": {body: body}}
}

func TestClaudeReadsPercentWindowsInReferenceOrder(t *testing.T) {
	stub := newClaudeStub(t, claudeUsageReplies(claudeUsageBody))

	result := fetchClaude(context.Background(), Credentials{AccessToken: "tok", Endpoint: stub.server.URL})

	want := []struct {
		label string
		used  float64
		reset time.Time
	}{
		{"session (5h)", 37, time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)},
		{"weekly (7d)", 80, time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)},
		{"weekly opus (7d)", 12, time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)},
		{"weekly sonnet (7d)", 45, time.Time{}},
		{"weekly fable (7d)", 64, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)},
	}
	if result.Plan != "Claude Code" || len(result.Quotas) != len(want) {
		t.Fatalf("plan=%q rows=%+v, want %d windows (%s)", result.Plan, result.Quotas, len(want), result.Message)
	}
	for index, expected := range want {
		row := result.Quotas[index]
		if row.Label != expected.label || row.Used != expected.used {
			t.Errorf("row %d = %+v, want %q at %v%%", index, row, expected.label, expected.used)
		}
		if row.Total != 100 || row.Unit != "%" || !row.Recurring {
			t.Errorf("row %d = %+v, want a recurring 100%% window", index, row)
		}
		if row.ResetAt.UTC() != expected.reset {
			t.Errorf("row %d reset = %v, want %v", index, row.ResetAt, expected.reset)
		}
	}
	if got := stub.recorded(); len(got) != 1 || got[0] != "GET /api/oauth/usage" {
		t.Fatalf("calls = %v, want one read of the OAuth usage endpoint", got)
	}
}

func TestClaudeSendsTheOAuthBetaFlagAndVersion(t *testing.T) {
	stub := newClaudeStub(t, claudeUsageReplies(claudeUsageBody))

	fetchClaude(context.Background(), Credentials{
		AccessToken: "tok-42",
		Endpoint:    stub.server.URL,
		UsageHeaders: map[string]string{
			"Anthropic-Beta":    "claude-code-20250219,interleaved-thinking-2025-05-14",
			"Anthropic-Version": "1999-01-01",
			"X-Trace":           "kept",
		},
	})

	if got := stub.headers.Get("Authorization"); got != "Bearer tok-42" {
		t.Errorf("Authorization = %q", got)
	}
	if got := stub.headers.Get("Anthropic-Beta"); got != "oauth-2025-04-20" {
		t.Errorf("anthropic-beta = %q, want the single OAuth flag", got)
	}
	if got := stub.headers.Get("Anthropic-Version"); got != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want the family's own API version", got)
	}
	if got := stub.headers.Get("X-Trace"); got != "kept" {
		t.Errorf("declared transport header lost: %q", got)
	}
}

// A 429 must cool down rather than cascade: the reference stops asking this token, and a
// fallback here would spend the same quota on a second endpoint.
func TestClaudeRateLimitedNamesTheCooldownAndMakesNoSecondCall(t *testing.T) {
	cases := []struct {
		name        string
		retryAfter  string
		wantMessage string
	}{
		{name: "the endpoint states its own wait", retryAfter: "45", wantMessage: claudeRateLimitedPrefix + "45s."},
		{name: "no header falls back to the reference cooldown", retryAfter: "", wantMessage: claudeRateLimitedPrefix + "180s."},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := newClaudeStub(t, map[string]claudeReply{
				"/api/oauth/usage": {status: http.StatusTooManyRequests, headers: map[string]string{"Retry-After": testCase.retryAfter}},
				"/v1/settings":     {body: claudeSettingsBody},
			})

			result := fetchClaude(context.Background(), Credentials{AccessToken: "tok", Endpoint: stub.server.URL})

			if !strings.HasPrefix(result.Message, testCase.wantMessage) {
				t.Fatalf("message = %q, want it to start with %q", result.Message, testCase.wantMessage)
			}
			if len(result.Quotas) != 0 {
				t.Fatalf("quotas = %+v, want a soft cooldown answer", result.Quotas)
			}
			if got := stub.recorded(); len(got) != 1 {
				t.Fatalf("calls = %v, want exactly one: a 429 must not be retried elsewhere", got)
			}
		})
	}
}

func TestClaudeSoftOutcomesStaySoft(t *testing.T) {
	cases := []struct {
		name        string
		replies     map[string]claudeReply
		credentials Credentials
		wantCalls   int
		wantMessage string
	}{
		{
			name:        "no token asks nothing",
			credentials: Credentials{},
			wantCalls:   0,
			wantMessage: claudeNoTokenMessage,
		},
		{
			name:        "a refused token is named once, not re-asked",
			replies:     map[string]claudeReply{"/api/oauth/usage": {status: http.StatusUnauthorized}, "/v1/settings": {body: claudeSettingsBody}},
			credentials: Credentials{AccessToken: "tok"},
			wantCalls:   1,
			wantMessage: "Claude credential invalid or expired (401).",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := newClaudeStub(t, testCase.replies)
			credentials := testCase.credentials
			credentials.Endpoint = stub.server.URL

			result := fetchClaude(context.Background(), credentials)

			if got := stub.recorded(); len(got) != testCase.wantCalls {
				t.Fatalf("calls = %v, want %d", got, testCase.wantCalls)
			}
			if result.Message != testCase.wantMessage || len(result.Quotas) != 0 {
				t.Fatalf("message = %q rows = %+v, want %q", result.Message, result.Quotas, testCase.wantMessage)
			}
		})
	}
}
