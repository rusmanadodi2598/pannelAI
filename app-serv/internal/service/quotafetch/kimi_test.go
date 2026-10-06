// Kimi tests (AGENTS.md §2.1): the two windows this surface publishes, as absolute counters, and the
// tier word the card is titled with.
//
// @file      internal/service/quotafetch/kimi_test.go
// @for       Locks the Kimi answer shape: which published windows become rows, with what counts, ceiling and reset, under which plan word.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    Kimi meters one credential as a rolling window beside rate limits and names its tier with its own codes, so a flipped bar or an invented plan name reaches the provider card silently.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// kimiUsagePath is the built-in surface's path. The test seam keeps the family's path and rewrites the
// host, so a wrong route is caught against this, and the built-in host against a literal in the other file.
const kimiUsagePath = "/coding/v1/usages"

// kimiStub answers the usage surface and records every read it got: one surface, one request.
type kimiStub struct {
	status int
	body   string
	server *httptest.Server
	calls  atomic.Int64
	mu     sync.Mutex
	asked  []string
	header http.Header
}

func kimiStubFor(t *testing.T, status int, body string) *kimiStub {
	t.Helper()
	stub := &kimiStub{status: status, body: body}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls.Add(1)
		stub.mu.Lock()
		stub.asked = append(stub.asked, r.Method+" "+r.URL.RequestURI())
		if stub.header == nil {
			stub.header = r.Header.Clone()
		}
		stub.mu.Unlock()
		if stub.status != 0 {
			w.WriteHeader(stub.status)
		}
		_, _ = io.WriteString(w, stub.body)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *kimiStub) requests() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.asked, "; ")
}

// read points the family at the stub and proves one read costs exactly one outbound call.
func (s *kimiStub) read(t *testing.T, creds Credentials) Result {
	t.Helper()
	creds.Endpoint = s.server.URL
	result := fetchKimi(context.Background(), creds)
	if got := s.calls.Load(); got != 1 {
		t.Fatalf("outbound calls = %d, want exactly 1 per read (%s)", got, s.requests())
	}
	return result
}

// kimiWantRow is one published window as the card must receive it; a zero reset is "none published".
type kimiWantRow struct {
	label string
	used  float64
	total float64
	reset time.Time
}

// TestKimi_ReportsTheWindowAndTheRateLimitItPublishes pins the two rows this surface has: the rolling
// window then the rate limit, both as absolute counters, drawn from the remaining count the provider
// states when it states one.
func TestKimi_ReportsTheWindowAndTheRateLimitItPublishes(t *testing.T) {
	rolling := time.Date(2026, 10, 2, 6, 0, 0, 0, time.UTC).UnixMilli()
	cases := []struct {
		name     string
		body     string
		wantPlan string
		wantMsg  string
		want     []kimiWantRow
	}{
		{
			name: "the rolling window and the rate limit beside it", wantPlan: "Allegro",
			body: fmt.Sprintf(`{"usage":{"limit":100,"used":25,"remaining":75,"resetTime":%d},`+
				`"limits":[{"detail":{"limit":"500","remaining":"400","reset_at":"1759399200"}}],`+
				`"user":{"membership":{"level":"LEVEL_ADVANCED"}}}`, rolling),
			want: []kimiWantRow{{"Weekly", 25, 100, time.UnixMilli(rolling)},
				{"Ratelimit", 100, 500, time.Unix(1759399200, 0)}},
		},
		{
			// The reference drew this row's bar from `remaining` and printed the provider's own use
			// count beside it; this port has one field for both and folds the bar into the count.
			name: "the remaining count draws the bar when both are published", wantPlan: "Kimi Coding",
			body: `{"usage":{"limit":100,"used":20,"remaining":30}}`,
			want: []kimiWantRow{{"Weekly", 70, 100, time.Time{}}},
		},
		{
			name: "the last published rate limit is the one kept", wantPlan: "Kimi Coding",
			body: `{"limits":[{"detail":{"limit":10,"remaining":0}},{"detail":{"limit":300,"remaining":"270"}}]}`,
			want: []kimiWantRow{{"Ratelimit", 30, 300, time.Time{}}},
		},
		{
			// With no remaining count published the port keeps the provider's own counter; the
			// reference had nothing to derive from and would have drawn the row as unused.
			name: "a rate limit that states only its use count keeps that count", wantPlan: "Kimi Coding",
			body: `{"limits":[{"detail":{"limit":100,"used":30}}]}`,
			want: []kimiWantRow{{"Ratelimit", 30, 100, time.Time{}}},
		},
		{
			name: "a count outside the published window is held inside it", wantPlan: "Kimi Coding",
			body: `{"usage":{"limit":100,"used":140},"limits":[{"detail":{"limit":100,"remaining":130}}]}`,
			want: []kimiWantRow{{"Weekly", 100, 100, time.Time{}}, {"Ratelimit", 0, 100, time.Time{}}},
		},
		{
			name: "resetTime wins over the other two spellings", wantPlan: "Vivace",
			body: fmt.Sprintf(`{"usage":{"limit":10,"used":1,"resetTime":%d,"reset_at":"1759399200",`+
				`"resetAt":"2026-11-01T00:00:00Z"},"limits":[{"detail":{"limit":10,"remaining":5,`+
				`"resetAt":"2026-11-01T00:00:00Z"}}],"user":{"membership":{"level":"LEVEL_STANDARD"}}}`, rolling),
			want: []kimiWantRow{{"Weekly", 1, 10, time.UnixMilli(rolling)},
				{"Ratelimit", 5, 10, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}},
		},
		{
			name: "a window with no readable ceiling is not a row", wantPlan: "Moderato",
			body: `{"usage":{"limit":"many","used":5},"limits":[{"detail":{"limit":null}}],` +
				`"user":{"membership":{"level":"LEVEL_BASIC"}}}`,
			wantMsg: "Kimi Coding. Usage tracked per request.",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result := kimiStubFor(t, 0, testCase.body).read(t, Credentials{APIKey: "km-1"})
			kimiWantRows(t, result, testCase.wantPlan, testCase.wantMsg, testCase.want)
		})
	}
}

// TestKimi_NamesTheCardWithTheMembershipTier pins the published tier words and the fallback for one this
// table has not seen, which is the provider's own word lowercased rather than an invented name.
func TestKimi_NamesTheCardWithTheMembershipTier(t *testing.T) {
	cases := []struct{ level, want string }{
		{`"LEVEL_BASIC"`, "Moderato"}, {`"LEVEL_INTERMEDIATE"`, "Allegretto"},
		{`"LEVEL_ADVANCED"`, "Allegro"}, {`"LEVEL_STANDARD"`, "Vivace"},
		{`"LEVEL_EXPERT"`, "expert"}, {`"UNSUBSCRIBED"`, "unsubscribed"},
		{`""`, "Kimi Coding"}, {`"LEVEL_"`, "Kimi Coding"},
	}
	for _, testCase := range cases {
		t.Run(testCase.level, func(t *testing.T) {
			result := kimiStubFor(t, 0, `{"user":{"membership":{"level":`+testCase.level+`}}}`).
				read(t, Credentials{APIKey: "km-1"})
			if result.Plan != testCase.want {
				t.Fatalf("plan = %q, want %q", result.Plan, testCase.want)
			}
		})
	}
}

// kimiWantRows compares the rows to the ones the card must receive, pinning an absolute counter with a
// ceiling: no unit, no percentage window, nothing unlimited, nothing recurring the provider never said.
func kimiWantRows(t *testing.T, result Result, wantPlan, wantMessage string, want []kimiWantRow) {
	t.Helper()
	if result.Plan != wantPlan {
		t.Fatalf("plan = %q, want %q (%s)", result.Plan, wantPlan, result.Message)
	}
	if result.Message != wantMessage {
		t.Fatalf("message = %q, want %q", result.Message, wantMessage)
	}
	if len(result.Quotas) != len(want) {
		t.Fatalf("rows = %+v, want %d", result.Quotas, len(want))
	}
	for index, row := range result.Quotas {
		expect := want[index]
		if row.Label != expect.label || row.Used != expect.used || row.Total != expect.total ||
			row.Unit != "" || row.Unlimited || row.Recurring || row.IsCreditBalance {
			t.Errorf("row %d = %+v, want %q at %.2f/%.2f as a plain bounded counter", index, row,
				expect.label, expect.used, expect.total)
		}
		if !row.ResetAt.UTC().Equal(expect.reset.UTC()) {
			t.Errorf("row %d reset = %v, want %v", index, row.ResetAt, expect.reset)
		}
	}
}
