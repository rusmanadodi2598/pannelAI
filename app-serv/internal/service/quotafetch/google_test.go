// Gemini CLI tests and the shared Cloud Code Assist client: the two-step project lookup, the
// single-step read when the connection already stores a project, the headers and bodies the
// endpoint demands, and the platform enum every bootstrap call carries.
//
// @file      internal/service/quotafetch/google_test.go
// @for       Locks the Gemini CLI quota read and the project resolution both Google families share.
// @uses      internal/service/quotafetch, io, net/http, net/http/httptest, strings, sync, testing, time
// @reason    The quota endpoint only answers for a project it is told, and the project comes
//
//	from a second endpoint, so the two-call and one-call shapes are the
//	regression worth pinning: a fetch that always asks twice burns a call,
//	and one that never looks the project up answers nothing at all.
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

const geminiQuotaBody = `{"buckets":[
	{"modelId":"gemini-3.1-pro-preview","remainingFraction":0.25,"resetTime":"2026-10-02T15:00:00Z"},
	{"modelId":"gemini-2.5-flash","remainingFraction":"0.5","resetTime":"1777000000000"},
	{"modelId":"bucket-without-a-fraction","resetTime":"2026-10-02T15:00:00Z"},
	{"remainingFraction":0.9}
]}`

const googleAccountBody = `{"cloudaicompanionProject":{"id":"proj-9"},
	"currentTier":{"name":"legacy-tier"},"paidTier":{"id":"standard-tier"}}`

// googleStub answers each Cloud Code Assist path and records the call it received, bodies
// included — the project a call names is half of what these families get wrong.
type googleStub struct {
	server  *httptest.Server
	replies map[string]googleReply

	mu      sync.Mutex
	calls   []string
	bodies  []string
	headers http.Header
}

type googleReply struct {
	status int
	body   string
}

func newGoogleStub(t *testing.T, replies map[string]googleReply) *googleStub {
	t.Helper()
	stub := &googleStub{replies: replies}
	stub.server = httptest.NewServer(http.HandlerFunc(stub.handle))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *googleStub) handle(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	s.mu.Lock()
	s.calls = append(s.calls, r.Method+" "+r.URL.Path)
	s.bodies = append(s.bodies, string(raw))
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
	if reply.status != 0 {
		w.WriteHeader(reply.status)
	}
	_, _ = io.WriteString(w, reply.body)
}

func (s *googleStub) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func (s *googleStub) sentBody(index int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= len(s.bodies) {
		return ""
	}
	return s.bodies[index]
}

func geminiReplies() map[string]googleReply {
	return map[string]googleReply{
		"/v1internal:retrieveUserQuota": {body: geminiQuotaBody},
		"/v1internal:loadCodeAssist":    {body: googleAccountBody},
	}
}

// A stored project is the whole point of the two-step: with one, the quota call is the only
// call, it names that trimmed project, and the buckets arrive in the published order. The plan
// stays the default the reference starts from, because no lookup was made to read a tier.
func TestGeminiCLIWithAStoredProjectAsksOnceAndReadsBucketsInOrder(t *testing.T) {
	stub := newGoogleStub(t, geminiReplies())

	result := fetchGeminiCLI(context.Background(), Credentials{
		AccessToken:          "ya29.tok",
		Endpoint:             stub.server.URL,
		ProviderSpecificData: map[string]string{googleProjectDataKey: " proj-7 "},
	})

	calls := stub.recorded()
	if len(calls) != 1 || calls[0] != "POST /v1internal:retrieveUserQuota" {
		t.Fatalf("calls = %v, want only the quota read", calls)
	}
	if body := stub.sentBody(0); body != `{"project":"proj-7"}` {
		t.Fatalf("body = %q, want the trimmed stored project", body)
	}
	if got := stub.headers.Get("Authorization"); got != "Bearer ya29.tok" {
		t.Errorf("Authorization = %q", got)
	}
	if got := stub.headers.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, the quota endpoint reads a JSON body", got)
	}

	want := []struct {
		label string
		used  float64
	}{
		{"gemini-3.1-pro-preview", 750},
		{"gemini-2.5-flash", 500},
	}
	if result.Plan != "Free" || len(result.Quotas) != len(want) {
		t.Fatalf("plan=%q rows=%+v, want two buckets (%s)", result.Plan, result.Quotas, result.Message)
	}
	for index, expected := range want {
		row := result.Quotas[index]
		if row.Label != expected.label || row.Used != expected.used || row.Total != 1000 || !row.Recurring {
			t.Errorf("row %d = %+v, want %q at %v of 1000", index, row, expected.label, expected.used)
		}
	}
	if result.Quotas[0].ResetAt.UTC() != time.Date(2026, 10, 2, 15, 0, 0, 0, time.UTC) {
		t.Errorf("first reset = %v, want the RFC3339 timestamp read", result.Quotas[0].ResetAt)
	}
	if result.Quotas[1].ResetAt.IsZero() {
		t.Error("second reset: a millisecond timestamp must still be read")
	}
}

func TestGeminiCLIWithoutAProjectLooksItUpFirst(t *testing.T) {
	stub := newGoogleStub(t, geminiReplies())

	result := fetchGeminiCLI(context.Background(), Credentials{AccessToken: "ya29.tok", Endpoint: stub.server.URL})

	want := []string{"POST /v1internal:loadCodeAssist", "POST /v1internal:retrieveUserQuota"}
	if got := stub.recorded(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	first := stub.sentBody(0)
	if !strings.Contains(first, `"ideType":9`) || !strings.Contains(first, `"pluginType":2`) {
		t.Errorf("bootstrap body = %q, want the CLIENT_METADATA identification", first)
	}
	if strings.Contains(first, `"mode"`) {
		t.Errorf("bootstrap body = %q, want no mode flag on the CLI lookup", first)
	}
	if body := stub.sentBody(1); body != `{"project":"proj-9"}` {
		t.Fatalf("quota body = %q, want the project the lookup published", body)
	}
	if result.Plan != "legacy-tier" {
		t.Errorf("plan = %q, want the tier the lookup published", result.Plan)
	}
}

func TestGeminiCLISoftOutcomesStaySoft(t *testing.T) {
	cases := []struct {
		name        string
		replies     map[string]googleReply
		credentials Credentials
		wantCalls   int
		wantPlan    string
		wantMessage string
	}{
		{
			name:        "no token asks nothing",
			credentials: Credentials{},
			wantCalls:   0,
			wantPlan:    "Free",
			wantMessage: googleNoTokenMessage,
		},
		{
			name:        "a lookup that names no project ends there",
			replies:     map[string]googleReply{"/v1internal:loadCodeAssist": {body: `{}`}},
			credentials: Credentials{AccessToken: "t"},
			wantCalls:   1,
			wantPlan:    "Free",
			wantMessage: googleNoProjectNote,
		},
		{
			name:        "a refused quota call keeps the reference sentence",
			replies:     map[string]googleReply{"/v1internal:retrieveUserQuota": {status: http.StatusUnauthorized}},
			credentials: Credentials{AccessToken: "t", ProviderSpecificData: map[string]string{googleProjectDataKey: "p"}},
			wantCalls:   1,
			wantPlan:    "Free",
			wantMessage: "Gemini CLI quota error (401).",
		},
		{
			name:        "an answer with no usable bucket is named as one",
			replies:     map[string]googleReply{"/v1internal:retrieveUserQuota": {body: `{"buckets":[]}`}},
			credentials: Credentials{AccessToken: "t", ProviderSpecificData: map[string]string{googleProjectDataKey: "p"}},
			wantCalls:   1,
			wantPlan:    "Free",
			wantMessage: googleNoBucketsNote,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := newGoogleStub(t, testCase.replies)
			credentials := testCase.credentials
			credentials.Endpoint = stub.server.URL

			result := fetchGeminiCLI(context.Background(), credentials)

			if got := stub.recorded(); len(got) != testCase.wantCalls {
				t.Fatalf("calls = %v, want %d", got, testCase.wantCalls)
			}
			if result.Plan != testCase.wantPlan || result.Message != testCase.wantMessage || len(result.Quotas) != 0 {
				t.Fatalf("plan=%q message=%q rows=%+v, want plan %q and %q",
					result.Plan, result.Message, result.Quotas, testCase.wantPlan, testCase.wantMessage)
			}
		})
	}
}
