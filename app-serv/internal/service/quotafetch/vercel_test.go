// Vercel family tests (docs/RULLES/TDD.md §2.5): credential gate, auth refusal, provider
// error passthrough, the unfunded sentence, and the two-row happy shape.
//
// @file      internal/service/quotafetch/vercel_test.go
// @for       Locks the vercel-ai-gateway fetcher's messages and quota rows against httptest answers.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    The family's soft sentences and the balance shape are the contract the panel's cards render.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package quotafetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestVercel_FetchesThePublishedCredits(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		status      int
		balance     float64
		used        float64
		wantMessage string
		wantPlan    string
		wantQuotas  int
	}{
		{
			name:        "an unfunded account says so instead of inventing rows",
			body:        `{"balance":"0","total_used":"0"}`,
			status:      http.StatusOK,
			wantMessage: "Vercel AI Gateway connected. No credit allocation found (BYOK or unfunded account).",
			wantPlan:    "Pay-as-you-go",
		},
		{
			name:       "a funded account answers the used and remaining rows",
			body:       `{"balance":"3.20","total_used":"1.80"}`,
			status:     http.StatusOK,
			wantQuotas: 2,
			wantPlan:   "Pay-as-you-go",
		},
		{
			name:        "a refused key names the credential",
			body:        `{"error":"bad key"}`,
			status:      http.StatusUnauthorized,
			wantMessage: "Vercel AI Gateway API key invalid or expired.",
		},
		{
			name:        "a provider error carries the status and the trimmed body",
			body:        `{"message":"upstream down"}`,
			status:      http.StatusInternalServerError,
			wantMessage: `Vercel AI Gateway credits API error (500): {"message":"upstream down"}`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Header.Get("Authorization") != "Bearer key-1" {
					t.Errorf("Authorization = %q, want the bearer key", r.Header.Get("Authorization"))
				}
				w.WriteHeader(testCase.status)
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()

			result := fetchVercel(context.Background(), Credentials{APIKey: "key-1", Endpoint: server.URL})

			if calls.Load() != 1 {
				t.Fatalf("outbound calls = %d, want exactly 1", calls.Load())
			}
			if result.Message != testCase.wantMessage {
				t.Fatalf("message = %q, want %q", result.Message, testCase.wantMessage)
			}
			if result.Plan != testCase.wantPlan {
				t.Fatalf("plan = %q, want %q", result.Plan, testCase.wantPlan)
			}
			if len(result.Quotas) != testCase.wantQuotas {
				t.Fatalf("quotas = %d, want %d", len(result.Quotas), testCase.wantQuotas)
			}
		})
	}
}

func TestVercel_TheFundedRowsCarryTheBalanceShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"balance":"3.20","total_used":"1.80"}`))
	}))
	defer server.Close()

	result := fetchVercel(context.Background(), Credentials{APIKey: "key-1", Endpoint: server.URL})

	used, remaining := result.Quotas[0], result.Quotas[1]
	if used.Label != "Used (USD)" || used.Used != 1.8 || !used.Unlimited {
		t.Fatalf("used row = %+v, want the unlimited spend row", used)
	}
	if remaining.Label != "Remaining (USD)" || remaining.Used != 3.2 || remaining.Total != monthlyCredit || remaining.Unlimited {
		t.Fatalf("remaining row = %+v, want the balance against the monthly allocation", remaining)
	}
}

func TestVercel_AMissingKeyAnswersWithoutCallingTheEndpoint(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	result := fetchVercel(context.Background(), Credentials{Endpoint: server.URL})

	if calls.Load() != 0 {
		t.Fatalf("outbound calls = %d, want none without a key", calls.Load())
	}
	if result.Message != "Vercel AI Gateway API key not available." {
		t.Fatalf("message = %q", result.Message)
	}
}
