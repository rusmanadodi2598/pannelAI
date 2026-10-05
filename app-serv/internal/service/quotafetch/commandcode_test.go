// Command Code tests (AGENTS.md §2.1): the credits window the plan cap turns into a share, the two
// rolling windows the billing host states, and the plan word the subscription carries.
//
// @file      internal/service/quotafetch/commandcode_test.go
// @for       Locks the Command Code answer shape: which windows become rows, with what counts, ceiling and reset, under which plan word.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    The provider splits this over three surfaces and states no ceiling on the credits one, so a cap read from the wrong place draws a balance as a share of nothing.
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
	"testing"
	"time"
)

// The three surfaces this family walks, spelled as the provider publishes them: the seam rewrites the
// host, so these are the paths a wrong route is caught against.
const (
	commandCodeWhoamiAPI        = "/alpha/whoami"
	commandCodeCreditsAPI       = "/alpha/billing/credits"
	commandCodeSubscriptionsAPI = "/alpha/billing/subscriptions"
)

// commandCodeFixture is one surface's canned answer.
type commandCodeFixture struct {
	status int
	body   string
}

// commandCodeStub stands in for the billing host on all three routes and records the walk in order.
type commandCodeStub struct {
	server  *httptest.Server
	byRoute map[string]commandCodeFixture
	mu      sync.Mutex
	asked   []string
	header  http.Header
}

func commandCodeStubFor(t *testing.T, byRoute map[string]commandCodeFixture) *commandCodeStub {
	t.Helper()
	stub := &commandCodeStub{byRoute: byRoute}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture := stub.byRoute[r.URL.Path]
		stub.mu.Lock()
		stub.asked = append(stub.asked, r.Method+" "+r.URL.RequestURI())
		if stub.header == nil {
			stub.header = r.Header.Clone()
		}
		stub.mu.Unlock()
		if fixture.status != 0 {
			w.WriteHeader(fixture.status)
		}
		_, _ = io.WriteString(w, fixture.body)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *commandCodeStub) requests() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.asked, "; ")
}

// read points the family at the stub and pins the whole walk the read just made, so a surface that was
// skipped or asked twice fails here rather than showing a short card.
func (s *commandCodeStub) read(t *testing.T, creds Credentials, wantAsked string) Result {
	t.Helper()
	creds.Endpoint = s.server.URL
	result := fetchCommandCode(context.Background(), creds)
	if got := s.requests(); got != wantAsked {
		t.Fatalf("requests = %q, want %q", got, wantAsked)
	}
	return result
}

// commandCodeAnswers builds the three bodies one read walks.
func commandCodeAnswers(whoami, credits, subscriptions string) map[string]commandCodeFixture {
	return map[string]commandCodeFixture{commandCodeWhoamiAPI: {body: whoami},
		commandCodeCreditsAPI: {body: credits}, commandCodeSubscriptionsAPI: {body: subscriptions}}
}

// commandCodeWantRow is one window as the card must receive it; a zero reset is "none published".
type commandCodeWantRow struct {
	label     string
	used      float64
	total     float64
	unlimited bool
	reset     time.Time
}

// TestCommandCode_ReportsTheCapTheWalletAndTheRollingWindows pins the three rows the reference builds from
// this walk: the credits window the plan cap turns into a share, then each rolling window it states.
func TestCommandCode_ReportsTheCapTheWalletAndTheRollingWindows(t *testing.T) {
	rolling := time.Date(2026, 10, 2, 6, 0, 0, 0, time.UTC).UnixMilli()
	period := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	walk := "GET " + commandCodeWhoamiAPI + "?limits=1; GET " + commandCodeCreditsAPI + "?orgId=org-1; GET " +
		commandCodeSubscriptionsAPI + "?orgId=org-1"
	cases := []struct {
		name     string
		credits  string
		subs     string
		wantPlan string
		want     []commandCodeWantRow
	}{
		{
			name: "a capped plan, its three wallets and both rolling windows", wantPlan: "Ultra",
			credits: fmt.Sprintf(`{"credits":{"monthlyCredits":100,"purchasedCredits":50,"freeCredits":"25"},`+
				`"windowLimits":{"fiveHour":{"used":12,"cap":"20","resetAt":%d},`+
				`"weekly":{"used":30,"cap":150,"resetAt":"1759399200"}}}`, rolling),
			subs: `{"data":{"planId":"individual-ultra","currentPeriodEnd":"2026-11-01T00:00:00Z"}}`,
			want: []commandCodeWantRow{{"Credits", 125, 300, false, period},
				{"Session (5h)", 12, 20, false, time.UnixMilli(rolling)},
				{"Weekly", 30, 150, false, time.Unix(1759399200, 0)}},
		},
		{
			name: "a plan this table has no ceiling for is a bare balance", wantPlan: "individual-nova",
			credits: `{"credits":{"monthlyCredits":87.5}}`,
			subs:    `{"data":{"planId":"individual-nova","currentPeriodEnd":1759399200}}`,
			want:    []commandCodeWantRow{{"Credits", 0, 87.5, true, time.Unix(1759399200, 0)}},
		},
		{
			name: "a wallet split across three counters, with no plan published", wantPlan: "Command Code",
			credits: `{"credits":{"monthlyCredits":7,"purchasedCredits":8,"freeCredits":9}}`,
			subs:    `{}`,
			want:    []commandCodeWantRow{{"Credits", 0, 24, true, time.Time{}}},
		},
		{
			name: "a balance above the cap never reads as spent", wantPlan: "Go",
			credits: `{"credits":{"monthlyCredits":40}}`, subs: `{"data":{"planId":"individual-go"}}`,
			want: []commandCodeWantRow{{"Credits", 0, 10, false, time.Time{}}},
		},
		{
			name: "a negative wallet is an empty one, drawn inside its cap", wantPlan: "GOAT",
			credits: `{"credits":{"monthlyCredits":-5,"purchasedCredits":3,"freeCredits":0}}`,
			subs:    `{"data":{"planId":"individual-goat"}}`,
			want:    []commandCodeWantRow{{"Credits", 70, 70, false, time.Time{}}},
		},
		{
			name: "a window above its ceiling is held inside it and an empty one is dropped", wantPlan: "Teams Pro",
			credits: `{"credits":{"monthlyCredits":1},"windowLimits":{"fiveHour":{"used":90,"cap":50,` +
				`"resetAt":"1759399200"},"weekly":{"used":0,"cap":0}}}`,
			subs: `{"data":{"planId":"teams-pro"}}`,
			want: []commandCodeWantRow{{"Credits", 39, 40, false, time.Time{}},
				{"Session (5h)", 50, 50, false, time.Unix(1759399200, 0)}},
		},
		{
			// A window with a use count but no published ceiling is still a window the provider
			// listed; the bar cannot run past a ceiling of nothing, so it reads as empty.
			name: "a use count with no ceiling is kept as a row that cannot fill", wantPlan: "Max",
			credits: `{"windowLimits":{"weekly":{"used":5,"cap":0}}}`,
			subs:    `{"data":{"planId":"individual-max"}}`,
			want: []commandCodeWantRow{{"Credits", 150, 150, false, time.Time{}},
				{"Weekly", 0, 0, false, time.Time{}}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := commandCodeStubFor(t, commandCodeAnswers(`{"org":{"id":"org-1"}}`,
				testCase.credits, testCase.subs))
			result := stub.read(t, Credentials{APIKey: " user_key_1 "}, walk)
			commandCodeWantRows(t, result, testCase.wantPlan, testCase.want)
		})
	}
}

// TestCommandCode_NamesTheCardWithThePublishedPlan pins the provider's own friendly name for each tier it
// sells, the raw plan id for one this table has not seen, and the family word for none.
func TestCommandCode_NamesTheCardWithThePublishedPlan(t *testing.T) {
	cases := []struct{ planID, want string }{
		{`"individual-go"`, "Go"}, {`"individual-goat"`, "GOAT"}, {`"individual-pro"`, "Pro"},
		{`"individual-pro-v1"`, "Pro"}, {`"individual-provider"`, "Provider"}, {`"individual-max"`, "Max"},
		{`"individual-ultra"`, "Ultra"}, {`"teams-pro"`, "Teams Pro"},
		{`"individual-something-new"`, "individual-something-new"}, {"null", "Command Code"},
	}
	noOrg := "GET " + commandCodeWhoamiAPI + "?limits=1; GET " + commandCodeCreditsAPI + "; GET " +
		commandCodeSubscriptionsAPI
	for _, testCase := range cases {
		t.Run(testCase.planID, func(t *testing.T) {
			stub := commandCodeStubFor(t, commandCodeAnswers(`{"org":{}}`, `{}`,
				`{"data":{"planId":`+testCase.planID+`}}`))
			result := stub.read(t, Credentials{APIKey: "k"}, noOrg)
			if result.Plan != testCase.want {
				t.Fatalf("plan = %q, want %q", result.Plan, testCase.want)
			}
		})
	}
}

// TestCommandCode_KeepsReadingWhenAHostAnswersGarbage pins the tolerance the reference has: an unreadable
// whoami leaves no organization to scope by, and an unreadable wallet is an empty balance rather than a
// failed read.
func TestCommandCode_KeepsReadingWhenAHostAnswersGarbage(t *testing.T) {
	stub := commandCodeStubFor(t, commandCodeAnswers(`<html>session</html>`, `<html>credits</html>`,
		`<html>subscriptions</html>`))
	result := stub.read(t, Credentials{APIKey: "k"},
		"GET "+commandCodeWhoamiAPI+"?limits=1; GET "+commandCodeCreditsAPI+"; GET "+commandCodeSubscriptionsAPI)

	commandCodeWantRows(t, result, "Command Code", []commandCodeWantRow{{"Credits", 0, 0, true, time.Time{}}})
}

// commandCodeWantRows compares the rows to the ones the card must receive: absolute counters in the
// provider's own credit, never a percentage, never recurring, and never the money balance the panel
// would print as an amount.
func commandCodeWantRows(t *testing.T, result Result, wantPlan string, want []commandCodeWantRow) {
	t.Helper()
	if result.Plan != wantPlan {
		t.Fatalf("plan = %q, want %q (%s)", result.Plan, wantPlan, result.Message)
	}
	if result.Message != "" {
		t.Fatalf("message = %q, want a hard answer with rows", result.Message)
	}
	if len(result.Quotas) != len(want) {
		t.Fatalf("rows = %+v, want %d", result.Quotas, len(want))
	}
	for index, row := range result.Quotas {
		expect := want[index]
		if row.Label != expect.label || row.Used != expect.used || row.Total != expect.total ||
			row.Unlimited != expect.unlimited || row.Unit != "" || row.Recurring || row.IsCreditBalance {
			t.Errorf("row %d = %+v, want %q at %.2f/%.2f unlimited %v as a plain counter", index, row,
				expect.label, expect.used, expect.total, expect.unlimited)
		}
		if !row.ResetAt.UTC().Equal(expect.reset.UTC()) {
			t.Errorf("row %d reset = %v, want %v", index, row.ResetAt, expect.reset)
		}
	}
}
