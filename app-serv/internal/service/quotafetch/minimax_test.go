// MiniMax tests (AGENTS.md §2.1): the two-host walk, the two counter meanings, soft answers.
//
// @file      internal/service/quotafetch/minimax_test.go
// @for       Locks the MiniMax quota read: rows and labels, both hosts and their order, refusals.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    Two hosts publish the same fields with opposite meanings and the registry may declare
//
//	both, one or neither, so a flipped bar or a skipped host misleads the operator silently.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
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

const (
	miniMaxTokenPath  = "/v1/token_plan/remains"
	miniMaxCodingPath = "/v1/api/openplatform/coding_plan/remains"
	miniMaxCodingDoc  = `{"model_remains":[{"model_name":"MiniMax-M3","current_interval_total_count":100,` +
		`"current_interval_usage_count":25}]}`
)

type miniMaxFixture struct {
	status int
	body   string
}

// miniMaxStub answers by path so one server stands in for both hosts, recording each read.
type miniMaxStub struct {
	mu     sync.Mutex
	asked  []string
	byPath map[string]miniMaxFixture
	server *httptest.Server
}

func miniMaxStubFor(t *testing.T, byPath map[string]miniMaxFixture) *miniMaxStub {
	t.Helper()
	stub := &miniMaxStub{byPath: byPath}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture := stub.byPath[r.URL.Path]
		stub.mu.Lock()
		stub.asked = append(stub.asked, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		stub.mu.Unlock()
		if fixture.status != 0 {
			w.WriteHeader(fixture.status)
		}
		_, _ = io.WriteString(w, fixture.body)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *miniMaxStub) requests() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.asked, "; ")
}

// miniMaxRow is one window as the card must receive it; `countdown` and `reset` are unread when zero.
type miniMaxRow struct {
	label     string
	used      float64
	total     float64
	unit      string
	reset     time.Time
	countdown time.Duration
}

// TestMiniMax_ReportsEachWindowTheTokenHostStates pins labels, names, clamps and the synthetic 100.
func TestMiniMax_ReportsEachWindowTheTokenHostStates(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []miniMaxRow
	}{
		{name: "two windows of one model, and a model publishing neither is dropped",
			body: `{"base_resp":{"status_code":0},"model_remains":[` +
				`{"model_name":"MiniMax-M3","current_interval_total_count":100,"current_interval_usage_count":30,` +
				`"current_weekly_total_count":500,"current_weekly_usage_count":"125","weekly_end_time":"1759399200"},` +
				`{"model_name":"speech-02-hd-tts-to-speech"}]}`,
			want: []miniMaxRow{{label: "MiniMax M3 (5h)", used: 30, total: 100},
				{label: "MiniMax M3 (7d)", used: 125, total: 500, reset: time.Unix(1759399200, 0)}}},
		{name: "a percent-only pool is a share of 100, camelCase spelling and all",
			body: `{"modelRemains":[{"modelName":"general","currentIntervalRemainingPercent":62.5,` +
				`"currentWeeklyRemainingPercent":"10","remainsTime":60000}]}`,
			want: []miniMaxRow{{label: "M-series (5h)", used: 38, total: 100, unit: "%", countdown: time.Minute},
				{label: "M-series (7d)", used: 90, total: 100, unit: "%"}}},
		{name: "the wildcard pool is named and a use count above its ceiling is held inside it",
			body: `{"model_remains":[{"model_name":"MiniMax-M*","current_interval_total_count":20,` +
				`"current_interval_usage_count":5,"remains_time":3600000,"end_time":"1759399200"},` +
				`{"model_name":"MiniMax-M2.5","current_interval_total_count":8,"current_interval_usage_count":14}]}`,
			want: []miniMaxRow{{label: "M-series (5h)", used: 5, total: 20, countdown: time.Hour},
				{label: "MiniMax M2.5 (5h)", used: 8, total: 8}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := miniMaxStubFor(t, map[string]miniMaxFixture{miniMaxTokenPath: {body: testCase.body}})
			before := time.Now()

			result := fetchMiniMax(context.Background(), Credentials{APIKey: " mm-1 ", Endpoint: stub.server.URL})

			if got := stub.requests(); got != "GET "+miniMaxTokenPath+" Bearer mm-1" {
				t.Fatalf("requests = %q, want the first host asked once with the bearer key", got)
			}
			miniMaxWantRows(t, result, testCase.want, before)
		})
	}
}

// TestMiniMax_FillsEachDeclaredHostSlot covers the declared pairs and pins the walk: first host at
// its own path, the second only when the first cannot serve it, its counter read as what is LEFT.
func TestMiniMax_FillsEachDeclaredHostSlot(t *testing.T) {
	cases := []struct {
		name     string
		declared []string
		want     []string
	}{
		{name: "nothing declared asks both built-ins", want: []string{miniMaxTokenPlanURL, miniMaxCodingPlanURL}},
		{name: "a blank slot keeps its own built-in", declared: []string{"", "https://moved.invalid/coding_plan/remains"},
			want: []string{miniMaxTokenPlanURL, "https://moved.invalid/coding_plan/remains"}},
		{name: "only the first declared keeps the second built-in", declared: []string{" https://moved.invalid/token "},
			want: []string{"https://moved.invalid/token", miniMaxCodingPlanURL}},
		{name: "both declared are asked exactly as declared",
			declared: []string{"https://a.invalid/x", miniMaxCodingPlanURL},
			want:     []string{"https://a.invalid/x", miniMaxCodingPlanURL}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			urls := miniMaxURLs(testCase.declared)
			if got := strings.Join(urls, " "); got != strings.Join(testCase.want, " ") {
				t.Fatalf("hosts = %q, want %q", got, strings.Join(testCase.want, " "))
			}

			stub := miniMaxStubFor(t, map[string]miniMaxFixture{
				miniMaxPathOf(urls[0]): {status: http.StatusNotFound},
				miniMaxPathOf(urls[1]): {body: miniMaxCodingDoc}})
			result := fetchMiniMax(context.Background(), Credentials{APIKey: "mm-1",
				Endpoints: UsageEndpoints{URLs: testCase.declared}, Endpoint: stub.server.URL})

			want := "GET " + miniMaxPathOf(urls[0]) + " Bearer mm-1; GET " + miniMaxPathOf(urls[1]) + " Bearer mm-1"
			if got := stub.requests(); got != want {
				t.Fatalf("requests = %q, want both declared hosts asked in order", got)
			}
			if len(result.Quotas) != 1 || result.Quotas[0].Used != 75 {
				t.Fatalf("rows = %+v, want the coding-plan counter read as what is left (%s)",
					result.Quotas, result.Message)
			}
		})
	}
}
