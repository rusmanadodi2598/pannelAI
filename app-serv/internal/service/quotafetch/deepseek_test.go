// DeepSeek tests (docs/RULLES/TDD.md §2.5): the wallet rendered as money, never as a window.
//
// @file      internal/service/quotafetch/deepseek_test.go
// @for       Locks the DeepSeek balance read: credit-balance rows, currency labels, plan wording.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    DeepSeek publishes a prepaid wallet, not a capped window, so the failures worth pinning are silent: a balance drawn as a share of a ceiling nobody stated.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// deepSeekWallet is one published currency as the card must receive it.
type deepSeekWallet struct {
	label string
	total float64
	unit  string
}

// deepSeekStub answers the published wallets and counts the reads it got: one surface, one
// request per read.
type deepSeekStub struct {
	body   string
	status int
	calls  atomic.Int64
}

func (s *deepSeekStub) server(t *testing.T) *httptest.Server {
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

// readBalance points the family at the stub, which keeps its own path, and proves the read cost
// exactly one outbound call.
func (s *deepSeekStub) readBalance(t *testing.T, creds Credentials) Result {
	t.Helper()
	creds.Endpoint = s.server(t).URL
	result := fetchDeepSeek(context.Background(), creds)
	if got := s.calls.Load(); got != 1 {
		t.Fatalf("outbound calls = %d, want exactly 1 per read", got)
	}
	return result
}

// TestDeepSeek_ReportsEachWalletAsACreditBalance pins the shape the panel needs: an amount of
// money, never a window. The cases cover a numeric string, several currencies, the camelCase
// amount, a zero, a negative, an unparseable amount and each spelling of the availability flag.
func TestDeepSeek_ReportsEachWalletAsACreditBalance(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantPlan string
		want     []deepSeekWallet
	}{
		{
			name: "a usd balance published as a string, the account available", wantPlan: "DeepSeek",
			body: `{"is_available":true,"balance_infos":[{"currency":"usd","total_balance":"12.34"}]}`,
			want: []deepSeekWallet{{"Balance (USD)", 12.34, "USD"}},
		},
		{
			name: "every currency keeps its own row", wantPlan: "DeepSeek",
			body: `{"is_available":true,"balance_infos":[` +
				`{"currency":"USD","total_balance":100},{"currency":"cny","total_balance":250.5}]}`,
			want: []deepSeekWallet{{"Balance (USD)", 100, "USD"}, {"Balance (CNY)", 250.5, "CNY"}},
		},
		{
			name: "the camelCase amount is read too, and its flag counts", wantPlan: "DeepSeek",
			body: `{"isAvailable":true,"balance_infos":[{"currency":"usd","totalBalance":"7.5"}]}`,
			want: []deepSeekWallet{{"Balance (USD)", 7.5, "USD"}},
		},
		{
			name: "a zero and a negative wallet both read as empty", wantPlan: "DeepSeek",
			body: `{"is_available":true,"balance_infos":[` +
				`{"currency":"USD","total_balance":0},{"currency":"CNY","total_balance":-40}]}`,
			want: []deepSeekWallet{{"Balance (USD)", 0, "USD"}, {"Balance (CNY)", 0, "CNY"}},
		},
		{
			name: "an unreadable amount is an empty wallet", wantPlan: "DeepSeek",
			body: `{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"plenty"}]}`,
			want: []deepSeekWallet{{"Balance (USD)", 0, "USD"}},
		},
		{
			name:     "an omitted availability flag is not read as a funded account",
			body:     `{"balance_infos":[{"currency":"USD","total_balance":5}]}`,
			wantPlan: "DeepSeek (Insufficient Balance)", want: []deepSeekWallet{{"Balance (USD)", 5, "USD"}},
		},
		{
			name: "a refused availability is said out loud", wantPlan: "DeepSeek (Insufficient Balance)",
			body: `{"is_available":false,"balance_infos":[{"currency":"USD","total_balance":5}]}`,
			want: []deepSeekWallet{{"Balance (USD)", 5, "USD"}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := &deepSeekStub{body: testCase.body}
			result := stub.readBalance(t, Credentials{APIKey: " key-1 "})

			if result.Plan != testCase.wantPlan {
				t.Fatalf("plan = %q, want %q", result.Plan, testCase.wantPlan)
			}
			if len(result.Quotas) != len(testCase.want) {
				t.Fatalf("rows = %+v, want %d (%s)", result.Quotas, len(testCase.want), result.Message)
			}
			for i, quota := range result.Quotas {
				want := testCase.want[i]
				if quota.Label != want.label || quota.Total != want.total || quota.Unit != want.unit {
					t.Errorf("row %d = %+v, want %s of %v %s", i, quota, want.label, want.total, want.unit)
				}
				// A prepaid wallet states an amount left, never a share spent, and it refills by
				// being topped up rather than by a cycle, so a row drawn as a capped window
				// would read as a quota the provider never capped and never rolls.
				if !quota.IsCreditBalance || quota.Unlimited || quota.Used != 0 ||
					quota.Recurring || !quota.ResetAt.IsZero() {
					t.Errorf("row %d = %+v, want an uncycled credit balance with no usage drawn", i, quota)
				}
			}
		})
	}
}

// TestDeepSeek_CallsTheBalanceEndpointOnce pins the host this family's entry declares nothing
// for: the built-in answers, and a declared usage URL still moves the call.
func TestDeepSeek_CallsTheBalanceEndpointOnce(t *testing.T) {
	cases := []struct {
		name         string
		creds        Credentials
		declaredPath string
		wantPath     string
		wantAuth     string
	}{
		{
			name:  "the entry declares no usage url, so the built-in balance host is asked",
			creds: Credentials{APIKey: "sk-1"}, wantPath: "/user/balance", wantAuth: "Bearer sk-1",
		},
		{
			name: "a declared usage url moves the call", declaredPath: "/moved/balance",
			creds: Credentials{AccessToken: "tok-1"}, wantPath: "/moved/balance", wantAuth: "Bearer tok-1",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != testCase.wantPath {
					t.Errorf("requested path = %q, want %q", r.URL.Path, testCase.wantPath)
				}
				if r.Method != http.MethodGet {
					t.Errorf("method = %q, want GET", r.Method)
				}
				if got := r.Header.Get("Authorization"); got != testCase.wantAuth {
					t.Errorf("authorization = %q, want %q", got, testCase.wantAuth)
				}
				_, _ = io.WriteString(w, `{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":1}]}`)
			}))
			defer server.Close()

			creds := testCase.creds
			creds.Endpoint = server.URL
			if testCase.declaredPath != "" {
				creds.Endpoints.URL = server.URL + testCase.declaredPath
			}
			result := fetchDeepSeek(context.Background(), creds)

			if got := calls.Load(); got != 1 {
				t.Fatalf("outbound calls = %d, want exactly 1 per read", got)
			}
			if len(result.Quotas) != 1 {
				t.Fatalf("rows = %+v, want the one published wallet (%s)", result.Quotas, result.Message)
			}
		})
	}
}

// TestDeepSeek_SoftAnswers pins that a wallet read never becomes a fault: a missing key costs no
// call at all, a refused or dead endpoint says so, and an unreadable body and an answer with
// nothing to show each name themselves rather than leaving the card blank.
func TestDeepSeek_SoftAnswers(t *testing.T) {
	cases := []struct {
		name      string
		creds     Credentials
		status    int
		body      string
		wantCalls int64
		want      string
	}{
		{name: "no key asks the provider nothing", body: `{}`,
			wantCalls: 0, want: "DeepSeek API key not available. Add a key to view usage."},
		{name: "a refused key names the credential", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			status: http.StatusUnauthorized, body: `{"error":{"message":"bad key"}}`,
			want: "DeepSeek credential invalid or expired (401)."},
		{name: "a forbidden key is the same dead credential", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			status: http.StatusForbidden, body: `{}`, want: "credential invalid or expired (403)"},
		{name: "a server error quotes the body", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			status: http.StatusBadGateway, body: `balance service down`,
			want: "DeepSeek quota API error (502): balance service down"},
		{name: "a body that is not json stays soft", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			body: `<html>captcha</html>`, want: "DeepSeek balance response was not JSON."},
		{name: "a connected account with no wallet says so", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			body: `{"is_available":true,"balance_infos":[]}`, want: "No balance data returned."},
		{name: "a wallet that names no currency is not shown", creds: Credentials{APIKey: "k"}, wantCalls: 1,
			body: `{"is_available":true,"balance_infos":[{"total_balance":5}]}`, want: "No balance data returned."},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := &deepSeekStub{body: testCase.body, status: testCase.status}
			server := stub.server(t)
			creds := testCase.creds
			creds.Endpoint = server.URL
			result := fetchDeepSeek(context.Background(), creds)

			if got := stub.calls.Load(); got != testCase.wantCalls {
				t.Fatalf("outbound calls = %d, want %d", got, testCase.wantCalls)
			}
			if len(result.Quotas) != 0 {
				t.Fatalf("rows = %+v, want a soft answer", result.Quotas)
			}
			if !strings.Contains(result.Message, testCase.want) {
				t.Fatalf("message = %q, want it to carry %q", result.Message, testCase.want)
			}
			if result.Plan != "DeepSeek" {
				t.Fatalf("plan = %q, want the family named even on a soft answer", result.Plan)
			}
		})
	}
}
