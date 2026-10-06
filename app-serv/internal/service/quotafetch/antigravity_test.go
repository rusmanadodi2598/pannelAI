// Antigravity family tests: the whole read, lookup, models, summary, in call order, the row
// order the card is handed, the free-tier rule, and the reconciliation that marks a session
// spent when its whole model family is.
//
// @file      internal/service/quotafetch/antigravity_test.go
// @for       Locks Antigravity's read order, model allowlist, tier rule and refusal sentences.
// @uses      internal/service/quotafetch, context, net/http, strings, testing, time
// @reason    Three endpoints feed one card, and the two that matter most are the ones a wrong order breaks silently: models before their weekly overlay, and a session row that has to read as spent when every model behind it is.
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

const antigravityModelsBody = `{"models":{
	"gemini-3.8-flash-high":{"displayName":"Gemini 3.8 Flash (High)","quotaInfo":{"remainingFraction":0.5,"resetTime":"2026-10-02T09:00:00Z"}},
	"gemini-3.5-flash-low":{"quotaInfo":{"remainingFraction":0.25}},
	"hidden-internal":{"displayName":"Internal","isInternal":true,"quotaInfo":{"remainingFraction":0.5}},
	"not-on-the-list":{"quotaInfo":{"remainingFraction":0.5}},
	"claude-sonnet-4-6":{"displayName":"Claude Sonnet 4.6","quotaInfo":{"remainingFraction":0,"resetTime":"2026-10-02T10:00:00Z"}}
}}`

const antigravitySpentBody = `{"models":{
	"gemini-3.8-flash-high":{"displayName":"Gemini 3.8 Flash (High)","quotaInfo":{"remainingFraction":0,"resetTime":"2026-10-02T11:00:00Z"}},
	"claude-sonnet-4-6":{"displayName":"Claude Sonnet 4.6","quotaInfo":{"remainingFraction":0.5,"resetTime":"2026-10-02T08:00:00Z"}}
}}`

const antigravityAccountBody = `{"cloudaicompanionProject":"proj-ag","currentTier":{"name":"antigravity-pro"},"paidTier":{"id":"standard-tier"}}`
const antigravityFreeAccountBody = `{"cloudaicompanionProject":"proj-ag","currentTier":{"name":"free"}}`

func antigravityReplies(modelsBody string) map[string]googleReply {
	return map[string]googleReply{
		"/v1internal:loadCodeAssist":           {body: antigravityAccountBody},
		"/v1internal:fetchAvailableModels":     {body: modelsBody},
		"/v1internal:retrieveUserQuotaSummary": {body: antigravitySummaryBody},
	}
}

func antigravityLabels(result Result) string {
	names := make([]string, 0, len(result.Quotas))
	for _, row := range result.Quotas {
		names = append(names, row.Label)
	}
	return strings.Join(names, ",")
}

func antigravityRowFind(rows []Quota, label string) (Quota, bool) {
	for _, row := range rows {
		if row.Label == label {
			return row, true
		}
	}
	return Quota{}, false
}

// Three calls in that order, models first on the card because the reference merged its weekly
// overlay on top of them. The internal model and the one outside the allowlist never arrive.
func TestAntigravityReadsModelsThenWeekly(t *testing.T) {
	stub := newGoogleStub(t, antigravityReplies(antigravityModelsBody))

	result := fetchAntigravity(context.Background(), Credentials{AccessToken: "ya29.ag", Endpoint: stub.server.URL})

	want := []string{"POST /v1internal:loadCodeAssist", "POST /v1internal:fetchAvailableModels",
		"POST /v1internal:retrieveUserQuotaSummary"}
	if got := stub.recorded(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	if !strings.Contains(stub.sentBody(0), `"mode":1`) {
		t.Errorf("lookup body = %q, want the IDE's mode flag", stub.sentBody(0))
	}
	if body := stub.sentBody(1); body != `{"project":"proj-ag"}` {
		t.Errorf("models body = %q, want the project the lookup published", body)
	}
	wantLabels := "Gemini 3.8 Flash (High),gemini-3.5-flash-low,Claude Sonnet 4.6," +
		antigravitySessionGemini + "," + antigravityWeeklyGemini + "," + antigravitySessionClaude
	if got := antigravityLabels(result); got != wantLabels {
		t.Fatalf("rows = %q, want %q (%s)", got, wantLabels, result.Message)
	}
	if result.Plan != "antigravity-pro" {
		t.Errorf("plan = %q, want the tier the lookup published", result.Plan)
	}
}

// Every gemini model spent, the gemini session window still showing room: the session row is
// what ran out, so it is the row that has to say so, with the models' latest reset.
func TestAntigravityReconcilesASpentModelFamily(t *testing.T) {
	stub := newGoogleStub(t, antigravityReplies(antigravitySpentBody))

	result := fetchAntigravity(context.Background(), Credentials{AccessToken: "t", Endpoint: stub.server.URL})

	session, found := antigravityRowFind(result.Quotas, antigravitySessionGemini)
	if !found {
		t.Fatalf("rows = %v, want the gemini session", antigravityLabels(result))
	}
	if session.Used != 1000 {
		t.Errorf("session = %+v, want it marked spent", session)
	}
	if want := time.Date(2026, 10, 2, 11, 0, 0, 0, time.UTC); session.ResetAt.UTC() != want {
		t.Errorf("session reset = %v, want the models' latest reset %v", session.ResetAt, want)
	}
	if weekly, _ := antigravityRowFind(result.Quotas, antigravityWeeklyGemini); weekly.Used != 900 {
		t.Errorf("weekly row changed: %+v, want it left as published", weekly)
	}
	if claude, _ := antigravityRowFind(result.Quotas, antigravitySessionClaude); claude.Used != 1000 {
		t.Errorf("claude session = %+v, want it left as the summary published it", claude)
	}
}

// A free tier has no separate 5-hour window, so its per-model fractions are the weekly limit
// read out of place: the card shows the summary rows and nothing else.
func TestAntigravityFreeTierShowsWeeklyOnly(t *testing.T) {
	stub := newGoogleStub(t, map[string]googleReply{
		"/v1internal:loadCodeAssist":           {body: antigravityFreeAccountBody},
		"/v1internal:fetchAvailableModels":     {body: antigravityModelsBody},
		"/v1internal:retrieveUserQuotaSummary": {body: antigravitySummaryBody},
	})

	result := fetchAntigravity(context.Background(), Credentials{AccessToken: "t", Endpoint: stub.server.URL})

	want := antigravitySessionGemini + "," + antigravityWeeklyGemini + "," + antigravitySessionClaude
	if got := antigravityLabels(result); got != want {
		t.Fatalf("rows = %q, want the weekly buckets only (%s)", got, result.Message)
	}
	if result.Plan != "free" {
		t.Errorf("plan = %q, want the tier the lookup published", result.Plan)
	}
}

// The weekly read is best-effort: a summary endpoint that fails must leave the model rows on
// the card rather than take them down with it.
func TestAntigravitySurvivesAFailedSummary(t *testing.T) {
	stub := newGoogleStub(t, map[string]googleReply{
		"/v1internal:loadCodeAssist":       {body: antigravityAccountBody},
		"/v1internal:fetchAvailableModels": {body: antigravityModelsBody},
	})

	result := fetchAntigravity(context.Background(), Credentials{AccessToken: "t", Endpoint: stub.server.URL})

	want := "Gemini 3.8 Flash (High),gemini-3.5-flash-low,Claude Sonnet 4.6"
	if got := antigravityLabels(result); got != want {
		t.Fatalf("rows = %q, want the model windows alone (%s)", got, result.Message)
	}
}

func TestAntigravitySoftOutcomesStaySoft(t *testing.T) {
	cases := []struct {
		name        string
		replies     map[string]googleReply
		credentials Credentials
		// The summary is always attempted, so an account that publishes nothing costs three
		// calls even when two of them answered nothing worth showing.
		wantCalls   int
		wantMessage string
	}{
		{
			name:        "no token asks nothing",
			credentials: Credentials{},
			wantCalls:   0,
			wantMessage: antigravityNoTokenMsg,
		},
		{
			name:        "an expired quota token keeps its own sentence",
			replies:     map[string]googleReply{"/v1internal:fetchAvailableModels": {status: http.StatusUnauthorized}},
			credentials: Credentials{AccessToken: "t"},
			wantCalls:   2,
			wantMessage: antigravityExpiredMsg,
		},
		{
			name:        "a forbidden quota endpoint still leaves chat working",
			replies:     map[string]googleReply{"/v1internal:fetchAvailableModels": {status: http.StatusForbidden}},
			credentials: Credentials{AccessToken: "t"},
			wantCalls:   2,
			wantMessage: antigravityForbiddenMsg,
		},
		{
			name:        "a server error quotes the provider",
			replies:     map[string]googleReply{"/v1internal:fetchAvailableModels": {status: http.StatusBadGateway, body: "upstream down"}},
			credentials: Credentials{AccessToken: "t"},
			wantCalls:   2,
			wantMessage: "Antigravity quota API error (502): upstream down",
		},
		{
			name:        "an account that publishes nothing is named as one",
			replies:     map[string]googleReply{"/v1internal:fetchAvailableModels": {body: `{}`}},
			credentials: Credentials{AccessToken: "t"},
			wantCalls:   3,
			wantMessage: antigravityNoRowsMsg,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := newGoogleStub(t, testCase.replies)
			credentials := testCase.credentials
			credentials.Endpoint = stub.server.URL

			result := fetchAntigravity(context.Background(), credentials)

			if got := stub.recorded(); len(got) != testCase.wantCalls {
				t.Fatalf("calls = %v, want %d", got, testCase.wantCalls)
			}
			if result.Message != testCase.wantMessage || len(result.Quotas) != 0 {
				t.Fatalf("message = %q rows = %+v, want %q", result.Message, result.Quotas, testCase.wantMessage)
			}
		})
	}
}
