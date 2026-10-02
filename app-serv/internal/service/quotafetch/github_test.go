// GitHub family tests: both response shapes, the `token` auth scheme plus the required
// version/editor headers, and the error-flavoured soft outcomes.
//
// @file      internal/service/quotafetch/github_test.go
// @for       Locks the GitHub Copilot usage read: snapshot and free shapes, token scheme, headers, soft paths.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    GitHub authenticates with a `token` scheme rather than a bearer and answers two different payload shapes, so a wrong scheme or host is the regression to catch.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const githubSnapshotBody = `{
	"copilot_plan":"copilot_business",
	"quota_reset_date":"2026-11-01T00:00:00Z",
	"quota_snapshots":{
		"chat":{"entitlement":300,"remaining":120},
		"completions":{"entitlement":1000,"remaining":1000,"unlimited":true},
		"premium_interactions":{"entitlement":300,"remaining":40}
	}
}`

const githubFreeBody = `{
	"copilot_plan":"copilot_free",
	"limited_user_reset_date":"2026-10-15T00:00:00Z",
	"monthly_quotas":{"chat":30,"completions":2000},
	"limited_user_quotas":{"chat":5,"completions":100}
}`

func TestGitHub_PaidSnapshotsUseEntitlementMinusRemaining(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(githubSnapshotBody))
	}))
	defer server.Close()

	result := fetchGitHub(context.Background(), Credentials{AccessToken: "ghp-1", Endpoint: server.URL})

	if result.Plan != "copilot_business" {
		t.Fatalf("plan = %q", result.Plan)
	}
	want := []struct {
		label       string
		used, total float64
		unlimited   bool
	}{
		{"chat", 180, 300, false},
		{"completions", 0, 1000, true},
		{"premium_interactions", 260, 300, false},
	}
	for _, expected := range want {
		row, ok := githubRowFind(result.Quotas, expected.label)
		if !ok {
			t.Fatalf("no %q row in %+v", expected.label, result.Quotas)
		}
		if row.Used != expected.used || row.Total != expected.total || row.Unlimited != expected.unlimited {
			t.Errorf("%q = %+v, want used %v total %v unlimited %v", expected.label, row, expected.used, expected.total, expected.unlimited)
		}
		if !row.Recurring || row.ResetAt.UTC() != time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) {
			t.Errorf("%q reset/recurring = %v/%v", expected.label, row.ResetAt, row.Recurring)
		}
	}
}

func TestGitHub_FreeQuotasReadUsedAndCeilingDirectly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(githubFreeBody))
	}))
	defer server.Close()

	result := fetchGitHub(context.Background(), Credentials{AccessToken: "ghp-1", Endpoint: server.URL})

	if result.Plan != "copilot_free" || len(result.Quotas) != 2 {
		t.Fatalf("plan=%q rows=%+v, want copilot_free and two windows", result.Plan, result.Quotas)
	}
	chat, _ := githubRowFind(result.Quotas, "chat")
	if chat.Used != 5 || chat.Total != 30 || chat.Unlimited {
		t.Errorf("chat = %+v, want used 5 total 30", chat)
	}
	completions, _ := githubRowFind(result.Quotas, "completions")
	if completions.Used != 100 || completions.Total != 2000 {
		t.Errorf("completions = %+v, want used 100 total 2000", completions)
	}
	if chat.ResetAt.UTC() != time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) {
		t.Errorf("chat reset = %v", chat.ResetAt)
	}
}

// The whole card depends on the `token` scheme and the identifying headers GitHub rejects a
// call without — and exactly one request per read.
func TestGitHub_AuthenticatesWithTheTokenSchemeAndApiHeaders(t *testing.T) {
	var calls atomic.Int64
	var gotAuth, gotAPIVersion, gotEditor, gotMethod, gotPath atomic.Value
	gotAuth.Store("")
	gotAPIVersion.Store("")
	gotEditor.Store("")
	gotMethod.Store("")
	gotPath.Store("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		gotAuth.Store(r.Header.Get("Authorization"))
		gotAPIVersion.Store(r.Header.Get("X-GitHub-Api-Version"))
		gotEditor.Store(r.Header.Get("Editor-Version"))
		gotMethod.Store(r.Method)
		gotPath.Store(r.URL.Path)
		_, _ = w.Write([]byte(githubSnapshotBody))
	}))
	defer server.Close()

	fetchGitHub(context.Background(), Credentials{AccessToken: "ghp-secret", Endpoint: server.URL})

	if calls.Load() != 1 {
		t.Fatalf("outbound calls = %d, want exactly 1", calls.Load())
	}
	if got := gotAuth.Load().(string); got != "token ghp-secret" {
		t.Errorf("Authorization = %q, want the token scheme", got)
	}
	if got := gotMethod.Load().(string); got != http.MethodGet {
		t.Errorf("method = %q, want GET", got)
	}
	if got := gotPath.Load().(string); got != "/copilot_internal/user" {
		t.Errorf("path = %q, want /copilot_internal/user", got)
	}
	if got := gotAPIVersion.Load().(string); got == "" {
		t.Error("X-GitHub-Api-Version header missing")
	}
	if got := gotEditor.Load().(string); got == "" {
		t.Error("Editor-Version header missing")
	}
}

func TestGitHub_SoftOutcomesStaySoft(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		credentials Credentials
		wantCalls   int64
		wantMessage string
	}{
		{
			name:        "a missing token never calls the endpoint",
			credentials: Credentials{},
			wantCalls:   0,
			wantMessage: "GitHub quota error: no access token available. Re-authorize the connection.",
		},
		{
			name:        "a refused token names the credential",
			status:      http.StatusUnauthorized,
			credentials: Credentials{AccessToken: "t"},
			wantCalls:   1,
			wantMessage: "GitHub credential invalid or expired (401).",
		},
		{
			name:        "a server error carries the provider body",
			status:      http.StatusBadGateway,
			body:        "upstream down",
			credentials: Credentials{AccessToken: "t"},
			wantCalls:   1,
			wantMessage: "GitHub quota API error (502): upstream down",
		},
		{
			name:        "a payload with neither shape is a soft parse note",
			status:      http.StatusOK,
			body:        `{"copilot_plan":"x"}`,
			credentials: Credentials{AccessToken: "t"},
			wantCalls:   1,
			wantMessage: "GitHub Copilot connected. Unable to parse quota data.",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if testCase.status != 0 {
					w.WriteHeader(testCase.status)
				}
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()

			credentials := testCase.credentials
			credentials.Endpoint = server.URL
			result := fetchGitHub(context.Background(), credentials)

			if calls.Load() != testCase.wantCalls {
				t.Fatalf("outbound calls = %d, want %d", calls.Load(), testCase.wantCalls)
			}
			if result.Message != testCase.wantMessage {
				t.Fatalf("message = %q, want %q", result.Message, testCase.wantMessage)
			}
		})
	}
}

func TestGitHub_ReadsItsPathFromTheDeclaredURLOrTheBuiltIn(t *testing.T) {
	cases := []struct {
		name     string
		declared bool
		wantPath string
	}{
		{name: "built-in path when nothing is declared", declared: false, wantPath: "/copilot_internal/user"},
		{name: "declared usage URL wins", declared: true, wantPath: "/copilot_internal/moved"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var path atomic.Value
			path.Store("")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path.Store(r.URL.Path)
				_, _ = w.Write([]byte(githubSnapshotBody))
			}))
			defer server.Close()

			credentials := Credentials{AccessToken: "t"}
			if testCase.declared {
				credentials.Endpoints.URL = server.URL + "/copilot_internal/moved"
			} else {
				credentials.Endpoint = server.URL
			}
			fetchGitHub(context.Background(), credentials)
			if got := path.Load().(string); got != testCase.wantPath {
				t.Fatalf("requested path = %q, want %q", got, testCase.wantPath)
			}
		})
	}
}

func githubRowFind(quotas []Quota, label string) (Quota, bool) {
	for _, quota := range quotas {
		if quota.Label == label {
			return quota, true
		}
	}
	return Quota{}, false
}
