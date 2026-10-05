// OpenCode Zen and Go tests (docs/RULLES/TDD.md §2.5): the two hosts and the percents they publish.
//
// @file      internal/service/quotafetch/opencode_test.go
// @for       Locks each OpenCode product's own host, its percent windows and its soft answers.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    Zen and Go differ only by host, so a wrong-host read and a percent drawn as a count stay silent.
//
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
	"sync/atomic"
	"testing"
	"time"
)

// openCodeRow is one published period as the card must receive it.
type openCodeRow struct {
	label string
	used  float64
	reset time.Time
}

// openCodeStub answers one product's usage document and counts the reads it got.
type openCodeStub struct {
	body   string
	status int
	calls  atomic.Int64
}

func (s *openCodeStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.calls.Add(1)
		if s.status != 0 {
			w.WriteHeader(s.status)
		}
		_, _ = io.WriteString(w, s.body)
	}))
	t.Cleanup(server.Close)
	return server
}

// read points the product at the stub, which keeps the family's own path.
func (s *openCodeStub) read(t *testing.T, family openCodeFamily, creds Credentials) Result {
	t.Helper()
	creds.Endpoint = s.server(t).URL
	result := fetchOpenCode(family)(context.Background(), creds)
	if got := s.calls.Load(); got != 1 {
		t.Fatalf("outbound calls = %d, want exactly 1 per read", got)
	}
	return result
}

// TestOpenCode_ReportsEachPublishedWindow pins the rows the operator reads and the shapes a
// percent arrives in: a number, a numeric string, a zero, a value past the ceiling, a non-object.
func TestOpenCode_ReportsEachPublishedWindow(t *testing.T) {
	resets := time.Date(2026, 10, 2, 6, 0, 0, 0, time.UTC).UnixMilli()
	cases := []struct {
		name   string
		family openCodeFamily
		body   string
		want   []openCodeRow
	}{
		{
			name: "all three windows, the first with a millisecond reset", family: openCodeZen,
			body: fmt.Sprintf(`{"usage":{"rolling":{"percent":42.5,"resetsAt":%d},`+
				`"weekly":{"percent":12.25},"monthly":{"percent":7}}}`, resets),
			want: []openCodeRow{{"Rolling", 42.5, time.UnixMilli(resets)}, {"Weekly", 12.25, time.Time{}}, {"Monthly", 7, time.Time{}}},
		},
		{name: "a percent sent as a numeric string, reset in seconds", family: openCodeGo,
			body: `{"usage":{"weekly":{"percent":"37.5","resetsAt":"1759399200"}}}`,
			want: []openCodeRow{{"Weekly", 37.5, time.Unix(1759399200, 0)}}},
		{name: "a zero percent is an untouched window, not an absent one", family: openCodeZen,
			body: `{"usage":{"monthly":{"percent":0}}}`, want: []openCodeRow{{"Monthly", 0, time.Time{}}}},
		{
			name: "a share past the ceiling is held inside it, a negative reads as untouched", family: openCodeGo,
			body: `{"usage":{"rolling":{"percent":140},"weekly":{"percent":-8.5},"monthly":{"percent":"nope"}}}`,
			want: []openCodeRow{{"Rolling", 100, time.Time{}}, {"Weekly", 0, time.Time{}}},
		},
		{name: "a period published as a bare string is skipped", family: openCodeZen,
			body: `{"usage":{"rolling":"n/a","weekly":{"percent":3}}}`, want: []openCodeRow{{"Weekly", 3, time.Time{}}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := &openCodeStub{body: testCase.body}
			result := stub.read(t, testCase.family, Credentials{APIKey: " key-1 "})

			if result.Plan != testCase.family.display {
				t.Fatalf("plan = %q, want %q", result.Plan, testCase.family.display)
			}
			if len(result.Quotas) != len(testCase.want) {
				t.Fatalf("rows = %+v, want %d (%s)", result.Quotas, len(testCase.want), result.Message)
			}
			for i, quota := range result.Quotas {
				openCodeWantRow(t, i, quota, testCase.want[i])
			}
		})
	}
}

// openCodeWantRow pins that a published percent reached the card as a share of one full
// allocation: a row of 140 of 140 would tell the operator the window is wider than the product
// states it is, and an unlimited or credit row would be the wrong kind of row entirely.
func openCodeWantRow(t *testing.T, i int, quota Quota, want openCodeRow) {
	t.Helper()
	if quota.Label != want.label || quota.Used != want.used {
		t.Errorf("row %d = %+v, want %s at %v", i, quota, want.label, want.used)
	}
	if quota.Total != 100 || quota.Unit != "%" || !quota.Recurring {
		t.Errorf("row %d = %+v, want a 100%% recurring percent window", i, quota)
	}
	if quota.Unlimited || quota.IsCreditBalance {
		t.Errorf("row %d = %+v, want neither an unlimited nor a credit row", i, quota)
	}
	if !quota.ResetAt.UTC().Equal(want.reset.UTC()) {
		t.Errorf("row %d reset = %v, want %v", i, quota.ResetAt, want.reset.UTC())
	}
}

// TestOpenCode_CallsItsOwnHostOnce is the wrong-host regression: Zen and Go share one handler.
func TestOpenCode_CallsItsOwnHostOnce(t *testing.T) {
	type request struct{ method, path, authorization, agent string }
	cases := []struct {
		name         string
		family       openCodeFamily
		declaredPath string
		headers      map[string]string
		want         request
	}{
		{name: "zen asks the zen host", family: openCodeZen,
			want: request{method: http.MethodGet, path: "/zen/v1/usage", authorization: "Bearer key-1"}},
		{name: "go asks the go host, never zen's", family: openCodeGo,
			want: request{method: http.MethodGet, path: "/zen/go/v1/usage", authorization: "Bearer key-1"}},
		{
			name: "a declared usage url and its headers move the call", family: openCodeZen,
			declaredPath: "/moved/zen/usage", headers: map[string]string{"User-Agent": "opencode-cli/1.2"},
			want: request{method: http.MethodGet, path: "/moved/zen/usage",
				authorization: "Bearer key-1", agent: "opencode-cli/1.2"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls atomic.Int64
			asked := make(chan request, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				asked <- request{method: r.Method, path: r.URL.Path,
					authorization: r.Header.Get("Authorization"), agent: r.Header.Get("User-Agent")}
				_, _ = io.WriteString(w, `{"usage":{"rolling":{"percent":1}}}`)
			}))
			defer server.Close()

			creds := Credentials{APIKey: " key-1 ", UsageHeaders: testCase.headers, Endpoint: server.URL}
			if testCase.declaredPath != "" {
				creds.Endpoints.URL = server.URL + testCase.declaredPath
			}
			result := fetchOpenCode(testCase.family)(context.Background(), creds)

			if got := calls.Load(); got != 1 {
				t.Fatalf("outbound calls = %d, want exactly 1 per read", got)
			}
			if len(result.Quotas) != 1 {
				t.Fatalf("rows = %+v, want the one published window (%s)", result.Quotas, result.Message)
			}
			got := <-asked
			// A bare request carries the runtime's own User-Agent, which proves nothing about this
			// family, so identification is compared only where the entry declares some.
			if testCase.want.agent == "" {
				got.agent = ""
			}
			if got != testCase.want {
				t.Fatalf("the read asked %+v, want %+v", got, testCase.want)
			}
		})
	}
}

// TestOpenCode_SoftAnswers pins that nothing a provider answers becomes a Go failure: a missing key
// asks nothing at all, a refusal names itself, and an unreadable document says what is missing.
func TestOpenCode_SoftAnswers(t *testing.T) {
	cases := []struct {
		name      string
		family    openCodeFamily
		key       string
		status    int
		body      string
		wantCalls int64
		want      string
	}{
		{name: "no key asks the provider nothing", family: openCodeZen, body: `{}`,
			wantCalls: 0, want: "API key not available. Add a key to view usage."},
		{name: "a refused key names the credential", family: openCodeGo, key: "k", wantCalls: 1,
			status: http.StatusUnauthorized, body: `{"error":{"message":"invalid"}}`,
			want: "credential invalid or expired (401)"},
		{name: "a server error quotes the body", family: openCodeZen, key: "k", wantCalls: 1,
			status: http.StatusInternalServerError, body: `upstream exploded`,
			want: "quota API error (500): upstream exploded"},
		{name: "an entitlement refusal names what zen is missing", family: openCodeZen, key: "k", wantCalls: 1,
			status: http.StatusForbidden, body: `{"error":{"type":"EntitlementError"}}`,
			want: "OpenCode Zen billing required for this API key."},
		{name: "the same refusal words go as a subscription", family: openCodeGo, key: "k", wantCalls: 1,
			status: http.StatusForbidden, body: `{"error":{"type":"EntitlementError"}}`,
			want: "OpenCode Go subscription required for this API key."},
		{name: "a plain 403 refuses the key", family: openCodeZen, key: "k", wantCalls: 1,
			status: http.StatusForbidden, body: `{"error":{"type":"PermissionDenied"}}`,
			want: "access forbidden for this API key."},
		{name: "a document with no quota object", family: openCodeZen, key: "k", wantCalls: 1,
			body: `{"plan":"free"}`, want: "did not contain quota data."},
		{name: "a quota object with no usable percent", family: openCodeGo, key: "k", wantCalls: 1,
			body: `{"usage":{"rolling":{"percent":null}}}`, want: "did not contain valid quota data."},
		{name: "a body that is not json stays soft", family: openCodeZen, key: "k", wantCalls: 1,
			body: `<html>captive portal</html>`, want: "did not contain quota data."},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := &openCodeStub{body: testCase.body, status: testCase.status}
			server := stub.server(t)
			result := fetchOpenCode(testCase.family)(context.Background(), Credentials{
				APIKey: testCase.key, Endpoint: server.URL,
			})

			if got := stub.calls.Load(); got != testCase.wantCalls {
				t.Fatalf("outbound calls = %d, want %d", got, testCase.wantCalls)
			}
			if len(result.Quotas) != 0 {
				t.Fatalf("rows = %+v, want a soft answer", result.Quotas)
			}
			if !strings.Contains(result.Message, testCase.want) {
				t.Fatalf("message = %q, want it to carry %q", result.Message, testCase.want)
			}
			if result.Plan != testCase.family.display {
				t.Fatalf("plan = %q, want the product named even on a soft answer", result.Plan)
			}
		})
	}
}
