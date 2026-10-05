// Antigravity weekly tests: the quota summary alone, its two envelopes, the four ways a
// bucket names its window, and the rows it is allowed to publish.
//
// @file      internal/service/quotafetch/antigravity_weekly_test.go
// @for       Locks Antigravity's summary parsing and the identification its summary call carries.
// @uses      internal/service/quotafetch, context, net/http, strings, testing, time
// @reason    The summary answers under two envelopes and classifies a window by more than one
//
//	field, so the four row names it produces, and the disabled buckets that
//	must not produce a fifth, are what a card cannot recover from getting
//	wrong.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The summary keeps one bucket per window twice over: a second weekly bucket must lose to the
// first, a disabled week must vanish, and a disabled session must stay and read as spent.
const antigravitySummaryBody = `{"groups":[
	{"displayName":"Gemini","buckets":[
		{"bucketId":"gemini-5-hour","window":"5h","remainingFraction":0.4,"resetTime":"2026-10-02T12:00:00Z"},
		{"bucketId":"gemini-weekly","window":"weekly","remainingFraction":0.1,"resetTime":"2026-10-06T12:00:00Z"},
		{"bucketId":"gemini-weekly-later","window":"weekly","remainingFraction":0.9}
	]},
	{"displayName":"Claude and GPT","buckets":[
		{"bucketId":"claude-weekly","window":"weekly","disabled":true,"remainingFraction":0.5},
		{"bucketId":"claude-5-hour","window":"5h","disabled":true,"remainingFraction":0.8}
	]},
	{"displayName":"Notebook","buckets":[{"bucketId":"other-5h","window":"5h","remainingFraction":1}]}
]}`

func TestAntigravityWeeklyRowsClassifyEveryBucket(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantRows []string
		wantUsed []float64
	}{
		{
			name:     "top-level groups, in the card's own order",
			body:     antigravitySummaryBody,
			wantRows: []string{antigravitySessionGemini, antigravityWeeklyGemini, antigravitySessionClaude},
			wantUsed: []float64{600, 900, 1000},
		},
		{
			name: "the quotaSummary envelope, and a window named only in its text",
			body: `{"quotaSummary":{"groups":[
				{"displayName":"GEMINI","buckets":[{"bucketId":"limit","displayName":"Weekly cap","remainingFraction":0.5}]},
				{"displayName":"GPT","buckets":[{"window":"daily","remainingFraction":"0.25"}]}]}}`,
			wantRows: []string{antigravityWeeklyGemini, antigravitySessionClaude},
			wantUsed: []float64{500, 750},
		},
		{
			name:     "a bucket that states no fraction publishes no window",
			body:     `{"groups":[{"displayName":"Gemini","buckets":[{"bucketId":"five hour","window":"5h"}]}]}`,
			wantRows: nil,
		},
		{
			name:     "a group in no known family publishes no window either",
			body:     `{"groups":[{"displayName":"Notebook","buckets":[{"bucketId":"five hour","window":"5h","remainingFraction":0.5}]}]}`,
			wantRows: nil,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var summary antigravitySummary
			if err := json.Unmarshal([]byte(testCase.body), &summary); err != nil {
				t.Fatalf("decoding fixture: %s", err)
			}
			rows := antigravityWeeklyRows(antigravitySummaryGroups(summary))

			if len(rows) != len(testCase.wantRows) {
				t.Fatalf("rows = %+v, want %v", rows, testCase.wantRows)
			}
			for index, label := range testCase.wantRows {
				if rows[index].Label != label || rows[index].Used != testCase.wantUsed[index] || rows[index].Total != 1000 {
					t.Errorf("row %d = %+v, want %q at %v of 1000", index, rows[index], label, testCase.wantUsed[index])
				}
			}
		})
	}
}

func TestAntigravityWeeklyReadCarriesTheIDEIdentification(t *testing.T) {
	stub := newGoogleStub(t, map[string]googleReply{
		"/v1internal:retrieveUserQuotaSummary": {body: antigravitySummaryBody},
	})

	rows := fetchAntigravityWeekly(context.Background(), Credentials{
		AccessToken: "ya29.ag",
		Endpoint:    stub.server.URL,
	}, "ya29.ag", "proj-ag")

	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want the three windows the summary publishes", rows)
	}
	if want := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC); rows[0].ResetAt.UTC() != want {
		t.Errorf("session reset = %v, want %v", rows[0].ResetAt, want)
	}
	if got := stub.recorded(); len(got) != 1 || got[0] != "POST /v1internal:retrieveUserQuotaSummary" {
		t.Fatalf("calls = %v, want one summary read", got)
	}
	if got := stub.headers.Get("X-Client-Name") + "/" + stub.headers.Get("X-Client-Version"); got != "antigravity/2.11.0" {
		t.Errorf("client identification = %q", got)
	}
	if got := stub.headers.Get("User-Agent"); !strings.HasPrefix(got, "antigravity/ide/") {
		t.Errorf("user agent = %q", got)
	}
	if got := stub.headers.Get("Authorization"); got != "Bearer ya29.ag" {
		t.Errorf("Authorization = %q", got)
	}
}

// A summary that refuses, fails or answers unreadably is no rows at all: the per-model windows
// already assembled are the better answer than nothing.
func TestAntigravityWeeklyFailureYieldsNoRows(t *testing.T) {
	cases := []struct {
		name  string
		reply googleReply
	}{
		{name: "a refusal", reply: googleReply{status: http.StatusForbidden}},
		{name: "a server error", reply: googleReply{status: http.StatusInternalServerError}},
		{name: "a body that is not the summary", reply: googleReply{body: "not json"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := newGoogleStub(t, map[string]googleReply{
				"/v1internal:retrieveUserQuotaSummary": testCase.reply,
			})

			rows := fetchAntigravityWeekly(context.Background(), Credentials{
				AccessToken: "t", Endpoint: stub.server.URL,
			}, "t", "")

			if len(rows) != 0 {
				t.Fatalf("rows = %+v, want none", rows)
			}
			if body := stub.sentBody(0); body != "{}" {
				t.Errorf("body = %q, want no project named when the caller had none", body)
			}
		})
	}
}
