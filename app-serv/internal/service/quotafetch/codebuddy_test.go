// CodeBuddy family tests (docs/RULLES/TDD.md §2.5): the credential gate, the billing
// envelope, the refill/bonus split the two credit types require, and the cadence labels.
//
// @file      internal/service/quotafetch/codebuddy_test.go
// @for       Locks the codebuddy billing read: headers, envelope, refill versus bonus rows, region wording.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    The refill and bonus credit types must never merge, and each region must word its own messages.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package quotafetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// billingAnswer wraps a payload in the doubled envelope the billing endpoint answers with.
func billingAnswer(inner string) string {
	return fmt.Sprintf(`{"code":0,"data":{"Response":{"Data":%s}}}`, inner)
}

func TestCodeBuddy_TheBillingEndpointCarriesTheFamilyHeadersAndAnEmptyBody(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if got := r.Header.Get("User-Agent"); got != "CLI/2.108.1 CodeBuddy/2.108.1" {
			t.Errorf("User-Agent = %q", got)
		}
		if r.Header.Get("X-Product") != "SaaS" || r.Header.Get("x-codebuddy-request") != "1" {
			t.Error("the registry header set did not arrive")
		}
		if r.Header.Get("Authorization") != "Bearer token-1" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "{}" {
			t.Errorf("body = %q, want {}", body)
		}
		_, _ = w.Write([]byte(billingAnswer(`{"Accounts":[]}`)))
	}))
	defer server.Close()

	result := readCodeBuddy(context.Background(), codebuddyCN, Credentials{AccessToken: "token-1", Endpoint: server.URL})

	if calls.Load() != 1 {
		t.Fatalf("outbound calls = %d, want exactly 1", calls.Load())
	}
	if result.Message != "CodeBuddy CN connected. No credit package found." {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestCodeBuddy_SoftOutcomesStaySoft(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		credentials Credentials
		wantMessage string
	}{
		{
			name:        "a missing credential never calls the endpoint",
			credentials: Credentials{},
			wantMessage: "CodeBuddy CN credential not available.",
		},
		{
			name:        "a refused token names the credential",
			status:      http.StatusForbidden,
			body:        `{"code":1,"msg":"denied"}`,
			credentials: Credentials{AccessToken: "t"},
			wantMessage: "CodeBuddy CN credential invalid or expired (403).",
		},
		{
			name:        "a billing error carries the provider's message",
			status:      http.StatusOK,
			body:        `{"code":1500,"msg":"system busy"}`,
			credentials: Credentials{AccessToken: "t"},
			wantMessage: "CodeBuddy CN quota error: system busy",
		},
		{
			name:        "a billing error without a message says unknown",
			status:      http.StatusOK,
			body:        `{"code":1500}`,
			credentials: Credentials{AccessToken: "t"},
			wantMessage: "CodeBuddy CN quota error: unknown",
		},
		{
			name:        "the intl family words its own region",
			status:      http.StatusOK,
			body:        `{"code":1,"msg":"x"}`,
			credentials: Credentials{APIKey: "k"},
			wantMessage: "CodeBuddy Intl quota error: x",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(testCase.status)
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()

			credentials := testCase.credentials
			credentials.Endpoint = server.URL
			result := Fetch(context.Background(), "codebuddy-cn", credentials)
			if testCase.wantMessage == "CodeBuddy Intl quota error: x" {
				result = Fetch(context.Background(), "codebuddy-intl", credentials)
			}

			if result.Message != testCase.wantMessage {
				t.Fatalf("message = %q, want %q", result.Message, testCase.wantMessage)
			}
		})
	}
}

// A monthly refill pack plus two bonus packs: the refill rolls its cycle long before the
// resource expires (>2d gap), the bonuses end exactly at expiry, and the soonest-expiring
// bonus is Bonus Pack 1.
func codeBuddyFixture() string {
	accounts := `{
		"Accounts":[
			{"PackageName":"GLM Coding Plan","CycleStartTime":"2026-09-01T00:00:00Z","CycleEndTime":"2026-10-01T00:00:00Z",
			 "DeductionEndTime":1798761600000,
			 "CycleCapacityUsedPrecise":"6.54000000","CycleCapacityUsed":6.54,
			 "CycleCapacitySizePrecise":"500.00000000","CycleCapacitySize":500},
			{"CycleStartTime":"2026-09-10T00:00:00Z","CycleEndTime":"2026-09-20T00:00:00Z",
			 "DeductionEndTime":1758326400000,
			 "CapacityUsedPrecise":"12.00000000","CapacityUsed":12,
			 "CapacitySizePrecise":"20.00000000","CapacitySize":20},
			{"CycleStartTime":"2026-09-12T00:00:00Z","CycleEndTime":"2026-09-15T00:00:00Z",
			 "DeductionEndTime":1757980800000,
			 "CapacityUsedPrecise":"1.00000000","CapacityUsed":1,
			 "CapacitySizePrecise":"5.00000000","CapacitySize":5}
		]}`
	return billingAnswer(accounts)
}

func TestCodeBuddy_RefillAndBonusPacksStaySeparate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(codeBuddyFixture()))
	}))
	defer server.Close()

	result := readCodeBuddy(context.Background(), codebuddyCN, Credentials{AccessToken: "t", Endpoint: server.URL})

	if result.Plan != "GLM Coding Plan" {
		t.Fatalf("plan = %q", result.Plan)
	}
	if len(result.Quotas) != 3 {
		t.Fatalf("quotas = %d, want the refill and two bonuses", len(result.Quotas))
	}

	refill := result.Quotas[0]
	if refill.Label != "Monthly" || !refill.Recurring {
		t.Fatalf("refill = %+v, want a recurring Monthly row", refill)
	}
	if refill.Used != 6.54 || refill.Total != 500 {
		t.Fatalf("refill counts = %v/%v, want the cycle balance", refill.Used, refill.Total)
	}
	if refill.ResetAt.UTC() != time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("refill reset = %v, want the next cycle refresh", refill.ResetAt)
	}

	first := result.Quotas[1]
	if first.Label != "Bonus Pack 1" || first.Recurring {
		t.Fatalf("first bonus = %+v, want the soonest-expiring one-shot row", first)
	}
	if first.Used != 1 || first.Total != 5 {
		t.Fatalf("first bonus counts = %v/%v", first.Used, first.Total)
	}
	second := result.Quotas[2]
	if second.Label != "Bonus Pack 2" || second.Used != 12 || second.Total != 20 {
		t.Fatalf("second bonus = %+v", second)
	}
}
