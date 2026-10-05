// GLM tests (docs/RULLES/TDD.md §2.5): the provider's own interval codes and the percent they carry.
//
// @file      internal/service/quotafetch/glm_test.go
// @for       Locks the GLM quota read: which limits become rows, their labels, their host, their refusals.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    GLM meters a plan in percentages against intervals only it names, so a wrong label and a wrong row type stay silent.
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

// glmRow is one published limit as the card must receive it.
type glmRow struct {
	label string
	used  float64
	reset time.Time
}

// glmStub answers one quota document and counts the reads it got: one surface, one request.
type glmStub struct {
	body   string
	status int
	calls  atomic.Int64
}

func (s *glmStub) server(t *testing.T) *httptest.Server {
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

// read points the family at the stub, which keeps its own path, and proves one call per read.
func (s *glmStub) read(t *testing.T, creds Credentials) Result {
	t.Helper()
	creds.Endpoint = s.server(t).URL
	result := fetchGlm(context.Background(), creds)
	if got := s.calls.Load(); got != 1 {
		t.Fatalf("outbound calls = %d, want exactly 1 per read", got)
	}
	return result
}

// TestGlm_ReportsEachPublishedLimitAsAPercentWindow pins the coercion the endpoint demands: a share
// of one allocation, never an absolute counter, in an interval only the provider names. It also pins
// which limits become rows at all, for both code spellings and both types.
func TestGlm_ReportsEachPublishedLimitAsAPercentWindow(t *testing.T) {
	session := time.Date(2026, 10, 2, 6, 0, 0, 0, time.UTC).UnixMilli()
	cases := []struct {
		name     string
		body     string
		wantPlan string
		want     []glmRow
	}{
		{
			name: "a five hour session and the week it belongs to", wantPlan: "Pro",
			body: fmt.Sprintf(`{"data":{"level":"pro","limits":[`+
				`{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":42.5,"nextResetTime":%d},`+
				`{"type":"TOKENS_LIMIT","unit":6,"number":1,"percentage":"37.5","nextResetTime":"1759399200"}]}}`, session),
			want: []glmRow{{"Session (5h)", 42.5, time.UnixMilli(session)},
				{"Weekly (7d)", 37.5, time.Unix(1759399200, 0)}},
		},
		{name: "a credit limit names the ceiling the provider states", wantPlan: "Lite",
			body: `{"data":{"level":"LITE","limits":[{"type":"CREDIT_LIMIT","unit":1,"number":300,"percentage":0}]}}`,
			want: []glmRow{{"Limit (300)", 0, time.Time{}}}},
		{name: "a tokens limit outside those intervals is still a row", wantPlan: "Unknown",
			body: `{"data":{"limits":[{"type":"TOKENS_LIMIT","unit":2,"number":9,"percentage":88}]}}`,
			want: []glmRow{{"Tokens", 88, time.Time{}}}},
		{name: "interval codes published as strings name the same rows", wantPlan: "Max",
			body: `{"data":{"level":"max","limits":[{"type":"TOKENS_LIMIT","unit":"3","number":"5","percentage":10.5}]}}`,
			want: []glmRow{{"Session (5h)", 10.5, time.Time{}}}},
		{
			name: "an unreadable or absent share reads as no use yet", wantPlan: "Unknown",
			body: `{"data":{"limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":"many"},` +
				`{"type":"CREDIT_LIMIT","unit":6,"percentage":null}]}}`,
			want: []glmRow{{"Session (5h)", 0, time.Time{}}, {"Weekly (7d)", 0, time.Time{}}},
		},
		{
			name: "a limit type outside the coding plan meters is dropped, not guessed at", wantPlan: "Pro",
			body: `{"data":{"level":"pro","limits":[` +
				`{"type":"REQUEST_LIMIT","unit":3,"number":5,"percentage":90},` +
				`{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":15}]}}`,
			want: []glmRow{{"Session (5h)", 15, time.Time{}}},
		},
		{
			name: "two weeks published are two rows, in the provider's order", wantPlan: "Unknown",
			body: `{"data":{"limits":[` +
				`{"type":"CREDIT_LIMIT","unit":6,"percentage":20},{"type":"CREDIT_LIMIT","unit":6,"percentage":35}]}}`,
			want: []glmRow{{"Weekly (7d)", 20, time.Time{}}, {"Weekly (7d)", 35, time.Time{}}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := &glmStub{body: testCase.body}
			result := stub.read(t, Credentials{APIKey: "key-1"})

			if result.Plan != testCase.wantPlan {
				t.Fatalf("plan = %q, want %q", result.Plan, testCase.wantPlan)
			}
			if len(result.Quotas) != len(testCase.want) {
				t.Fatalf("rows = %+v, want %d (%s)", result.Quotas, len(testCase.want), result.Message)
			}
			for i, quota := range result.Quotas {
				want := testCase.want[i]
				if quota.Label != want.label || quota.Used != want.used ||
					!quota.ResetAt.UTC().Equal(want.reset.UTC()) {
					t.Errorf("row %d = %+v, want %s at %v resetting %v", i, quota,
						want.label, want.used, want.reset.UTC())
				}
				// A published percentage is a share of one allocation: 42.5 of 42.5 would read as
				// spent where the provider said 42.5% used.
				if quota.Total != 100 || quota.Unit != "%" || !quota.Recurring ||
					quota.Unlimited || quota.IsCreditBalance {
					t.Errorf("row %d = %+v, want a 100%% recurring percent window", i, quota)
				}
			}
		})
	}
}

// TestGlm_CallsTheQuotaEndpointOnce pins the host, the method and the credential shape.
func TestGlm_CallsTheQuotaEndpointOnce(t *testing.T) {
	type request struct{ method, path, authorization, agent string }
	cases := []struct {
		name         string
		creds        Credentials
		declaredPath string
		want         request
	}{
		{
			name: "the built-in quota path is asked once", creds: Credentials{APIKey: " key-1 "},
			want: request{method: http.MethodGet, path: "/api/monitor/usage/quota/limit", authorization: "Bearer key-1"},
		},
		{
			name: "a declared usage url and its headers win", declaredPath: "/moved/glm/quota",
			creds: Credentials{AccessToken: "tok-1", UsageHeaders: map[string]string{"User-Agent": "zai-cli/9"}},
			want: request{method: http.MethodGet, path: "/moved/glm/quota",
				authorization: "Bearer tok-1", agent: "zai-cli/9"},
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
				_, _ = io.WriteString(w, `{"data":{"limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":1}]}}`)
			}))
			defer server.Close()

			creds := testCase.creds
			creds.Endpoint = server.URL
			if testCase.declaredPath != "" {
				creds.Endpoints.URL = server.URL + testCase.declaredPath
			}
			result := fetchGlm(context.Background(), creds)

			if got := calls.Load(); got != 1 {
				t.Fatalf("outbound calls = %d, want exactly 1 per read", got)
			}
			if len(result.Quotas) != 1 {
				t.Fatalf("rows = %+v, want the one published limit (%s)", result.Quotas, result.Message)
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

// TestGlm_SoftAnswers pins the family's soft outcomes: a missing key costs no call at all, a refusal
// and a dead endpoint say so, and an answer that publishes nothing names that rather than blanking.
func TestGlm_SoftAnswers(t *testing.T) {
	cases := []struct {
		name      string
		creds     Credentials
		status    int
		body      string
		wantPlan  string
		wantCalls int64
		want      string
	}{
		{name: "no key asks the provider nothing", body: `{}`,
			wantPlan: "GLM", wantCalls: 0, want: "GLM API key not available."},
		{name: "a refused key names the credential", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			status: http.StatusUnauthorized, body: `{"message":"no"}`, wantPlan: "GLM",
			want: "GLM credential invalid or expired (401)."},
		{name: "a server error quotes the body", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			status: http.StatusServiceUnavailable, body: `quota service down`, wantPlan: "GLM",
			want: "GLM quota API error (503): quota service down"},
		{name: "an answer that publishes nothing says so", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			body: `{"data":{"level":"pro","limits":[]}}`, wantPlan: "Pro",
			want: "GLM published no quota for this account."},
		{name: "a block with no data at all says so", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			body: `{"code":0}`, wantPlan: "Unknown", want: "GLM published no quota for this account."},
		{name: "a body that is not json stays soft", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			body: `<html>login first</html>`, wantPlan: "GLM", want: "GLM error: "},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := &glmStub{body: testCase.body, status: testCase.status}
			creds := testCase.creds
			creds.Endpoint = stub.server(t).URL
			result := fetchGlm(context.Background(), creds)

			if got := stub.calls.Load(); got != testCase.wantCalls {
				t.Fatalf("outbound calls = %d, want %d", got, testCase.wantCalls)
			}
			if len(result.Quotas) != 0 {
				t.Fatalf("rows = %+v, want a soft answer", result.Quotas)
			}
			if !strings.Contains(result.Message, testCase.want) {
				t.Fatalf("message = %q, want it to carry %q", result.Message, testCase.want)
			}
			if result.Plan != testCase.wantPlan {
				t.Fatalf("plan = %q, want %q", result.Plan, testCase.wantPlan)
			}
		})
	}
}
