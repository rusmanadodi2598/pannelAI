// Soft-answer tests for the MiniMax family.
//
// @file      internal/service/quotafetch/minimax_soft_test.go
// @for       Proves MiniMax's refusals stay soft sentences and its windows are filled per declared host.
// @uses      internal/service/quotafetch, net/http/httptest, testing, time.
// @reason    A dead key and an unimplemented family look identical to a page that broke,
//
//	unless each is its own answer: the reference renders the provider's sentence
//	on the card, so the wording and the host slot it arrives from are pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMiniMax_SoftAnswers(t *testing.T) {
	cases := []struct {
		name     string
		creds    Credentials
		byPath   map[string]miniMaxFixture
		requests int
		want     string
	}{
		{name: "no key asks the provider nothing", creds: Credentials{}, want: "MiniMax API key not available."},
		{name: "a refused key names the credential", creds: Credentials{APIKey: "mm"}, requests: 1,
			byPath: map[string]miniMaxFixture{miniMaxTokenPath: {status: http.StatusUnauthorized}},
			want:   "MiniMax API key invalid or inactive. Use an active Token/Coding Plan key."},
		{name: "an in-body 1004 is the same dead plan", creds: Credentials{APIKey: "mm"}, requests: 1,
			byPath: map[string]miniMaxFixture{miniMaxTokenPath: {body: `{"base_resp":{"status_code":1004}}`}},
			want:   "MiniMax API key invalid or inactive"},
		{name: "plan prose in a non-json body is read as one", creds: Credentials{APIKey: "mm"}, requests: 1,
			byPath: map[string]miniMaxFixture{miniMaxTokenPath: {body: `<html>Coding Plan required</html>`}},
			want:   "MiniMax API key invalid or inactive"},
		{name: "an upstream code quotes its own message", creds: Credentials{APIKey: "mm"}, requests: 1,
			byPath: map[string]miniMaxFixture{miniMaxTokenPath: {body: `{"base_resp":{"status_code":2013,"status_msg":"quota exhausted"}}`}},
			want:   "MiniMax connected. quota exhausted"},
		{name: "an upstream code with no message is still named", creds: Credentials{APIKey: "mm"}, requests: 1,
			byPath: map[string]miniMaxFixture{miniMaxTokenPath: {body: `{"baseResp":{"statusCode":2013}}`}},
			want:   "MiniMax connected. Upstream quota API error"},
		{name: "an answer with no quota says so, json or not", creds: Credentials{APIKey: "mm"}, requests: 1,
			byPath: map[string]miniMaxFixture{miniMaxTokenPath: {body: `{"model_remains":[]}`}},
			want:   "MiniMax connected. No quota data was returned."},
		{name: "an error the other host cannot fix is reported", creds: Credentials{APIKey: "mm"}, requests: 1,
			byPath: map[string]miniMaxFixture{miniMaxTokenPath: {status: http.StatusBadRequest}},
			want:   "MiniMax connected. MiniMax usage endpoint error (400)"},
		{name: "both hosts failing keeps the last error", creds: Credentials{APIKey: "mm"}, requests: 2,
			byPath: map[string]miniMaxFixture{miniMaxTokenPath: {status: http.StatusServiceUnavailable},
				miniMaxCodingPath: {status: http.StatusInternalServerError}},
			want: "MiniMax connected. MiniMax usage endpoint error (500)"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := miniMaxStubFor(t, testCase.byPath)
			creds := testCase.creds
			if creds.APIKey != "" {
				creds.Endpoint = stub.server.URL
			}

			result := fetchMiniMax(context.Background(), creds)

			if got := len(stub.asked); got != testCase.requests {
				t.Fatalf("outbound requests = %d, want %d (%q)", got, testCase.requests, stub.requests())
			}
			if len(result.Quotas) != 0 {
				t.Fatalf("rows = %+v, want a soft answer", result.Quotas)
			}
			if !strings.Contains(result.Message, testCase.want) {
				t.Fatalf("message = %q, want it to carry %q", result.Message, testCase.want)
			}
		})
	}
}

// miniMaxWantRows compares the rows to the ones the card must receive, pinning a bounded window.
func miniMaxWantRows(t *testing.T, result Result, want []miniMaxRow, readAt time.Time) {
	t.Helper()
	if len(result.Quotas) != len(want) {
		t.Fatalf("rows = %+v, want %d (%s)", result.Quotas, len(want), result.Message)
	}
	for index, row := range result.Quotas {
		expect := want[index]
		if row.Label != expect.label || row.Used != expect.used || row.Total != expect.total ||
			row.Unit != expect.unit || row.Unlimited || row.IsCreditBalance || row.Recurring {
			t.Errorf("row %d = %+v, want %q at %.2f/%.2f unit %q as a plain bounded window", index, row,
				expect.label, expect.used, expect.total, expect.unit)
		}
		if expect.countdown > 0 {
			if span := row.ResetAt.Sub(readAt); span < expect.countdown-time.Second || span > expect.countdown+3*time.Second {
				t.Errorf("row %d reset = %v, want about %v out", index, row.ResetAt, expect.countdown)
			}
		}
		if !expect.reset.IsZero() && !row.ResetAt.UTC().Equal(expect.reset.UTC()) {
			t.Errorf("row %d reset = %v, want %v", index, row.ResetAt, expect.reset)
		}
	}
}

func miniMaxPathOf(fullURL string) string {
	host := fullURL[strings.Index(fullURL, "//")+2:]
	if slash := strings.Index(host, "/"); slash >= 0 {
		return host[slash:]
	}
	return "/"
}
