// Zed family tests: the `<user_id> <access_token>` scheme, the polymorphic usage limits,
// the token-billing note, the trial label and the overdue override.
//
// @file      internal/service/quotafetch/zed_test.go
// @for       Locks the Zed quota read: auth scheme, unlimited and zero limits, plan labels, soft paths.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    Zed authenticates with a combined user-id/token header and sends limits in three shapes, so both the scheme and the "unlimited"/zero coercion are regressions worth pinning.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
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

const zedSnapshotBody = `{"plan":{"plan_v3":"zed_pro","subscription_period":{"ended_at":"2026-11-01T00:00:00Z"},` +
	`"usage":{"edit_predictions":{"used":120,"limit":500},"model_requests":{"used":10,"limit":{"limited":1000}}}}}`

func TestZed_RendersBucketsAndUsesTheCombinedAuthHeader(t *testing.T) {
	var calls atomic.Int64
	var gotAuth, gotMethod, gotPath atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		gotAuth.Store(r.Header.Get("Authorization"))
		gotMethod.Store(r.Method)
		gotPath.Store(r.URL.Path)
		_, _ = w.Write([]byte(zedSnapshotBody))
	}))
	defer server.Close()

	result := fetchZed(context.Background(), Credentials{AccessToken: "tok-abc", ProviderSpecificData: map[string]string{"userId": "user-42"}, Endpoint: server.URL})

	if calls.Load() != 1 {
		t.Fatalf("outbound calls = %d, want exactly 1", calls.Load())
	}
	if got := gotAuth.Load().(string); got != "user-42 tok-abc" {
		t.Errorf("Authorization = %q, want the combined user-id token scheme", got)
	}
	if got := gotMethod.Load().(string); got != http.MethodGet {
		t.Errorf("method = %q, want GET", got)
	}
	if got := gotPath.Load().(string); got != "/client/users/me" {
		t.Errorf("path = %q, want /client/users/me", got)
	}
	if result.Plan != "Zed Pro" {
		t.Fatalf("plan = %q, want Zed Pro", result.Plan)
	}
	edits, ok := zedRowFind(result.Quotas, "Edit Predictions")
	if !ok || edits.Used != 120 || edits.Total != 500 || edits.Unlimited {
		t.Fatalf("Edit Predictions = %+v, want used 120 total 500", edits)
	}
	if edits.ResetAt.UTC() != time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) || !edits.Recurring {
		t.Errorf("Edit Predictions reset/recurring = %v/%v", edits.ResetAt, edits.Recurring)
	}
	models, ok := zedRowFind(result.Quotas, "Hosted Model Requests")
	if !ok || models.Used != 10 || models.Total != 1000 {
		t.Fatalf("Hosted Model Requests = %+v, want used 10 total 1000", models)
	}
}

// The string "unlimited" is a window with no ceiling: Unlimited set, Total zero, no bar.
func TestZed_UnlimitedModelRequestsHaveNoCeiling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"plan":{"plan_v3":"zed_business","usage":{"model_requests":{"used":5,"limit":"unlimited"}}}}`))
	}))
	defer server.Close()

	result := fetchZed(context.Background(), Credentials{AccessToken: "tok", ProviderSpecificData: map[string]string{"userId": "u"}, Endpoint: server.URL})
	models, ok := zedRowFind(result.Quotas, "Hosted Model Requests")
	if !ok || !models.Unlimited || models.Total != 0 || models.Used != 5 {
		t.Fatalf("model requests = %+v, want unlimited with no ceiling", models)
	}
}

// A zero model-request limit is token billing, not a spent quota: no Hosted Model Requests
// row, an explanatory note, and Edit Predictions still reported.
func TestZed_ZeroLimitIsTokenBillingNotAZeroRow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"plan":{"plan_v3":"zed_pro","usage":{` +
			`"edit_predictions":{"used":3,"limit":100},"model_requests":{"used":0,"limit":{"limited":0}}}}}`))
	}))
	defer server.Close()

	result := fetchZed(context.Background(), Credentials{AccessToken: "tok", ProviderSpecificData: map[string]string{"userId": "u"}, Endpoint: server.URL})
	if _, present := zedRowFind(result.Quotas, "Hosted Model Requests"); present {
		t.Error("a token-billed zero limit was rendered as a request quota row")
	}
	if result.Message != zedTokenBillingNote {
		t.Fatalf("message = %q, want the token-billing note", result.Message)
	}
	if _, ok := zedRowFind(result.Quotas, "Edit Predictions"); !ok {
		t.Error("Edit Predictions dropped alongside the token-billing note")
	}
}

func TestZed_TrialLabelAndOverdueOverride(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantPlan string
		wantMsg  string
	}{
		{
			name:     "trial appended when the plan does not already say trial",
			body:     `{"plan":{"plan_v3":"zed_pro","trial_started_at":"2026-01-01T00:00:00Z","usage":{"edit_predictions":{"used":1,"limit":10}}}}`,
			wantPlan: "Zed Pro (Trial active)",
		},
		{
			name:     "trial plan not double-labelled",
			body:     `{"plan":{"plan_v3":"zed_pro_trial","trial_started_at":"2026-01-01T00:00:00Z","usage":{"edit_predictions":{"used":1,"limit":10}}}}`,
			wantPlan: "Zed Pro Trial",
		},
		{
			name:     "overdue invoices override the note",
			body:     `{"plan":{"plan_v3":"zed_student","has_overdue_invoices":true,"usage":{"edit_predictions":{"used":1,"limit":10}}}}`,
			wantPlan: "Zed Student",
			wantMsg:  zedOverdueNote,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()
			result := fetchZed(context.Background(), Credentials{AccessToken: "tok", ProviderSpecificData: map[string]string{"userId": "u"}, Endpoint: server.URL})
			if result.Plan != testCase.wantPlan {
				t.Errorf("plan = %q, want %q", result.Plan, testCase.wantPlan)
			}
			if result.Message != testCase.wantMsg {
				t.Errorf("message = %q, want %q", result.Message, testCase.wantMsg)
			}
		})
	}
}

func TestZed_SoftOutcomesStaySoft(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		credentials Credentials
		wantCalls   int64
		wantMessage string
	}{
		{
			name:        "a missing access token never calls the endpoint",
			credentials: Credentials{ProviderSpecificData: map[string]string{"userId": "u"}},
			wantCalls:   0,
			wantMessage: "Zed access token not available. Re-connect Zed to view quota.",
		},
		{
			name:        "a missing user id never calls the endpoint",
			credentials: Credentials{AccessToken: "tok"},
			wantCalls:   0,
			wantMessage: "Zed credential is missing user id. Re-connect Zed to view quota.",
		},
		{
			name:        "a refused credential names it",
			status:      http.StatusUnauthorized,
			credentials: Credentials{AccessToken: "tok", ProviderSpecificData: map[string]string{"userId": "u"}},
			wantCalls:   1,
			wantMessage: "Zed credential invalid or expired (401).",
		},
		{
			name:        "a server error carries the provider's own sentence",
			status:      http.StatusInternalServerError,
			body:        `{"message":"nope"}`,
			credentials: Credentials{AccessToken: "tok", ProviderSpecificData: map[string]string{"userId": "u"}},
			wantCalls:   1,
			wantMessage: `Zed quota API error (500): {"message":"nope"}`,
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
			result := fetchZed(context.Background(), credentials)
			if calls.Load() != testCase.wantCalls {
				t.Fatalf("outbound calls = %d, want %d", calls.Load(), testCase.wantCalls)
			}
			if result.Message != testCase.wantMessage {
				t.Fatalf("message = %q, want %q", result.Message, testCase.wantMessage)
			}
		})
	}
}

func TestZed_ReadsItsPathFromTheDeclaredURLOrTheBuiltIn(t *testing.T) {
	cases := []struct {
		name     string
		declared bool
		wantPath string
	}{
		{name: "built-in path when nothing is declared", declared: false, wantPath: "/client/users/me"},
		{name: "declared usage URL wins", declared: true, wantPath: "/client/moved"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var path atomic.Value
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path.Store(r.URL.Path)
				_, _ = w.Write([]byte(`{"plan":{"plan_v3":"zed_pro","usage":{}}}`))
			}))
			defer server.Close()
			credentials := Credentials{AccessToken: "tok", ProviderSpecificData: map[string]string{"userId": "u"}}
			if testCase.declared {
				credentials.Endpoints.URL = server.URL + "/client/moved"
			} else {
				credentials.Endpoint = server.URL
			}
			fetchZed(context.Background(), credentials)
			if got := path.Load().(string); got != testCase.wantPath {
				t.Fatalf("requested path = %q, want %q", got, testCase.wantPath)
			}
		})
	}
}

func zedRowFind(quotas []Quota, label string) (Quota, bool) {
	for _, quota := range quotas {
		if quota.Label == label {
			return quota, true
		}
	}
	return Quota{}, false
}
