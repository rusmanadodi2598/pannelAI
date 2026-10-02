// Family tests for grok-cli: the billing + user reads (paths, headers, one call each), the
// percentage and exhausted-cap rows, the soft auth/parse outcomes, the no-credential gate, and
// the weekly-pool frame fallback.
//
// @file      internal/service/quotafetch/grok_test.go
// @for       Locks the Grok CLI reads, row shapes and soft paths against stubbed endpoints.
// @uses      internal/service/quotafetch, net/http, net/http/httptest, sync, testing
// @reason    A wrong host or dropped header is silent, and rows are percentages with a synthetic ceiling.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// grokVendor stubs the billing, user and gRPC-web surfaces on one host and records every call.
type grokVendor struct {
	server      *httptest.Server
	billing     atomic.Int64
	user        atomic.Int64
	grpc        atomic.Int64
	mu          sync.Mutex
	billingPath string
	userPath    string
	header      http.Header
	billingBody string
	billingCode int
	grpcBody    []byte
	grpcCode    int
}

func newGrokVendor() *grokVendor {
	vendor := &grokVendor{billingBody: `{"config":{}}`}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/billing", func(w http.ResponseWriter, r *http.Request) {
		vendor.billing.Add(1)
		vendor.mu.Lock()
		vendor.billingPath, vendor.header = r.URL.Path, r.Header.Clone()
		vendor.mu.Unlock()
		if vendor.billingCode != 0 {
			w.WriteHeader(vendor.billingCode)
		}
		_, _ = w.Write([]byte(vendor.billingBody))
	})
	asked := func(w http.ResponseWriter, r *http.Request) {
		vendor.user.Add(1)
		vendor.mu.Lock()
		vendor.userPath = r.URL.Path
		vendor.mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}
	mux.HandleFunc("/v1/user", asked)
	mux.HandleFunc("/v1/declared-user", asked)
	mux.HandleFunc("/grok_api_v2.GrokBuildBilling/GetGrokCreditsConfig", func(w http.ResponseWriter, r *http.Request) {
		vendor.grpc.Add(1)
		if vendor.grpcCode != 0 {
			w.WriteHeader(vendor.grpcCode)
		}
		_, _ = w.Write(vendor.grpcBody)
	})
	vendor.server = httptest.NewServer(mux)
	return vendor
}

func grokRow(quotas []Quota, label string) (Quota, bool) {
	for _, quota := range quotas {
		if quota.Label == label {
			return quota, true
		}
	}
	return Quota{}, false
}

func TestGrok_ReadsBillingAndUserWithFamilyHeaders(t *testing.T) {
	vendor := newGrokVendor()
	defer vendor.server.Close()
	vendor.billingBody = `{"config":{"monthlyLimit":{"val":100},"includedUsed":{"val":20},` +
		`"onDemandCap":{"val":50},"onDemandUsed":{"val":10},"creditUsagePercent":{"val":40},` +
		`"billingPeriodEnd":"2026-10-31T00:00:00Z","subscriptionTier":"super_grok"}}`
	result := fetchGrok(context.Background(), Credentials{AccessToken: "tok",
		ProviderSpecificData: map[string]string{"email": "a@b.c", "userId": "u9"}, Endpoint: vendor.server.URL})

	if vendor.billing.Load() != 1 || vendor.user.Load() != 1 || vendor.grpc.Load() != 0 {
		t.Fatalf("calls billing/user/grpc = %d/%d/%d, want 1/1/0", vendor.billing.Load(), vendor.user.Load(), vendor.grpc.Load())
	}
	if vendor.billingPath != "/v1/billing" || vendor.userPath != "/v1/user" {
		t.Fatalf("paths = %q/%q, want /v1/billing and /v1/user", vendor.billingPath, vendor.userPath)
	}
	for key, want := range map[string]string{
		"Authorization": "Bearer tok", "User-Agent": "grok-shell/0.2.99 (linux; x86_64)",
		"x-xai-token-auth": "xai-grok-cli", "x-grok-client-identifier": "grok-shell",
		"x-grok-client-version": "0.2.99", "x-grok-client-mode": "headless", "x-email": "a@b.c", "x-userid": "u9",
	} {
		if got := vendor.header.Get(key); got != want {
			t.Errorf("header %s = %q, want %q", key, got, want)
		}
	}
	monthly, ok := grokRow(result.Quotas, "Monthly included")
	if !ok || monthly.Used != 20 || monthly.Total != 100 || monthly.Unit != "" {
		t.Fatalf("Monthly included = %+v, want 20 of 100", monthly)
	}
	weekly, ok := grokRow(result.Quotas, "Weekly SuperGrok")
	if !ok || weekly.Used != 40 || weekly.Total != 100 || weekly.Unit != "%" {
		t.Fatalf("Weekly SuperGrok = %+v, want 40 of 100 unit %%", weekly)
	}
	if result.Plan != "super_grok" {
		t.Fatalf("plan = %q, want the provider tier verbatim", result.Plan)
	}
}

// A declared user_url is asked verbatim: the built-in must not win over a path the registry names,
// which is the exact defect this family carries (`user_url` was declared and never delivered).
func TestGrok_AsksTheDeclaredUserURL(t *testing.T) {
	vendor := newGrokVendor()
	defer vendor.server.Close()
	fetchGrok(context.Background(), Credentials{AccessToken: "tok", Endpoints: UsageEndpoints{
		URL:     vendor.server.URL + "/v1/billing?format=credits",
		UserURL: vendor.server.URL + "/v1/declared-user",
	}})
	if vendor.user.Load() != 1 || vendor.userPath != "/v1/declared-user" {
		t.Fatalf("user read = %d at %q, want 1 at the declared /v1/declared-user",
			vendor.user.Load(), vendor.userPath)
	}
}

// With no declared user_url the user read is rebuilt from the billing host, keeping both reads on one origin.
func TestGrok_DerivesUserHostFromBillingOrigin(t *testing.T) {
	vendor := newGrokVendor()
	defer vendor.server.Close()
	fetchGrok(context.Background(), Credentials{AccessToken: "tok", Endpoints: UsageEndpoints{URL: vendor.server.URL + "/v1/billing?format=credits"}})
	if vendor.user.Load() != 1 || vendor.userPath != "/v1/user" {
		t.Fatalf("user read = %d at %q, want 1 at /v1/user derived from the billing origin", vendor.user.Load(), vendor.userPath)
	}
}

func TestGrok_ExhaustedCapBecomesSyntheticFullBar(t *testing.T) {
	vendor := newGrokVendor()
	defer vendor.server.Close()
	vendor.billingBody = `{"config":{"onDemandCap":{"val":0},"onDemandUsed":{"val":7}}}`
	ondemand, ok := grokRow(fetchGrok(context.Background(), Credentials{AccessToken: "tok", Endpoint: vendor.server.URL}).Quotas, "On-demand")
	if !ok || ondemand.Used != 1 || ondemand.Total != 1 || ondemand.Unit != "%" || ondemand.Unlimited {
		t.Fatalf("On-demand = %+v, want synthetic 1/1 unit %%", ondemand)
	}
}

func TestGrok_PrepaidRowReportsBalanceWithNoReset(t *testing.T) {
	vendor := newGrokVendor()
	defer vendor.server.Close()
	vendor.billingBody = `{"config":{"prepaidBalance":{"val":12.5}}}`
	prepaid, ok := grokRow(fetchGrok(context.Background(), Credentials{AccessToken: "tok", Endpoint: vendor.server.URL}).Quotas, "Prepaid")
	if !ok || prepaid.Used != 0 || prepaid.Total != 12.5 || !prepaid.ResetAt.IsZero() {
		t.Fatalf("Prepaid = %+v, want 0 of 12.5 with no reset", prepaid)
	}
}

// With no REST allotment the family reads the weekly pool frame: valid adds the row, malformed soft-fails.
func TestGrok_FallbackReadsWeeklyPoolFrame(t *testing.T) {
	cases := []struct {
		name     string
		grpcBody []byte
		wantRow  bool
		wantMsg  string
	}{
		{name: "valid frame adds the row", grpcBody: grokFrameBuildCredits(0.3, 1_700_000_000, 0, true), wantRow: true},
		{name: "malformed frame soft-fails", grpcBody: []byte{0x00, 0x00, 0x00, 0x7f, 0xff, 0x0a}, wantMsg: "could not be decoded"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			vendor := newGrokVendor()
			defer vendor.server.Close()
			vendor.grpcBody = testCase.grpcBody
			result := fetchGrok(context.Background(), Credentials{AccessToken: "tok", Endpoint: vendor.server.URL})
			if vendor.billing.Load() != 1 || vendor.user.Load() != 1 || vendor.grpc.Load() != 1 {
				t.Fatalf("calls billing/user/grpc = %d/%d/%d, want 1/1/1", vendor.billing.Load(), vendor.user.Load(), vendor.grpc.Load())
			}
			weekly, present := grokRow(result.Quotas, "Weekly SuperGrok")
			if testCase.wantRow && (!present || weekly.Total != 100 || weekly.Unit != "%") {
				t.Fatalf("weekly row = %+v present=%v, want 100 %% ceiling", weekly, present)
			}
			if testCase.wantMsg != "" && !strings.Contains(result.Message, testCase.wantMsg) {
				t.Fatalf("message = %q, want it to mention %q", result.Message, testCase.wantMsg)
			}
		})
	}
}

func TestGrok_SoftOutcomesStaySoft(t *testing.T) {
	cases := []struct {
		name        string
		credentials Credentials
		code        int
		body        string
		wantCalls   int64
		wantMessage string
	}{
		{name: "no credential makes no call", wantCalls: 0, wantMessage: "Grok CLI access token not available."},
		{name: "refused credential names it", credentials: Credentials{AccessToken: "tok"}, code: http.StatusUnauthorized, wantCalls: 1, wantMessage: "Grok CLI credential invalid or expired (401)."},
		{name: "server error quotes the body", credentials: Credentials{AccessToken: "tok"}, code: http.StatusInternalServerError, body: "boom", wantCalls: 1, wantMessage: "Grok CLI quota API error (500): boom"},
		{name: "non-JSON body soft-fails", credentials: Credentials{AccessToken: "tok"}, body: "<html>nope</html>", wantCalls: 1, wantMessage: "Grok CLI billing response was not JSON."},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			vendor := newGrokVendor()
			defer vendor.server.Close()
			vendor.billingCode, vendor.billingBody = testCase.code, testCase.body
			credentials := testCase.credentials
			credentials.Endpoint = vendor.server.URL
			result := fetchGrok(context.Background(), credentials)
			if vendor.billing.Load() != testCase.wantCalls {
				t.Fatalf("billing calls = %d, want %d", vendor.billing.Load(), testCase.wantCalls)
			}
			if result.Message != testCase.wantMessage {
				t.Fatalf("message = %q, want %q", result.Message, testCase.wantMessage)
			}
		})
	}
}
