// Package quotafetch reads the quota a provider publishes for one of its connections.
//
// @file      internal/service/quotafetch/qoder_test.go
// @for       The Qoder quota read: its buckets, its credential exchange, and the
//
//	answers a card should render rather than a failure to route.
//
// @uses      context, encoding/json, fmt, io, net/http, net/http/httptest, strings,
//
//	testing, time.
//
// @reason    This family publishes credits in two buckets and refuses a raw Personal
//
//	Access Token, so the two failures worth pinning are a read that
//	presents the stored credential (looks like a dead account) and a read
//	that turns the vendor's absolute `remaining` into a share (looks like
//	a number). Both are silent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package quotafetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// qoderVendor is the quota service and the token exchange on one host, which is how
// the real service is laid out: the exchange host is the usage host's origin, so a
// single stub covers both.
type qoderVendor struct {
	server      *httptest.Server
	quotaBody   string
	quotaStatus int
	bearer      string
	exchanges   int
	exchanged   string
}

func newQoderVendor(t *testing.T, quotaBody string, quotaStatus int) *qoderVendor {
	t.Helper()
	vendor := &qoderVendor{quotaBody: quotaBody, quotaStatus: quotaStatus}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/quota/usage", func(w http.ResponseWriter, r *http.Request) {
		if vendor.quotaStatus != 0 {
			w.WriteHeader(vendor.quotaStatus)
			_, _ = io.WriteString(w, `{"message":"upstream said no"}`)
			return
		}
		vendor.bearer = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, vendor.quotaBody)
	})
	mux.HandleFunc("/api/v1/jobToken/exchange", func(w http.ResponseWriter, r *http.Request) {
		vendor.exchanges++
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var sent struct {
			PersonalToken string `json:"personal_token"`
		}
		_ = json.Unmarshal(raw, &sent)
		vendor.exchanged = sent.PersonalToken
		_, _ = io.WriteString(w, `{"token":"jt-issued","expires_at":"2099-01-01T00:00:00Z"}`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	vendor.server = server
	return vendor
}

// read runs one family's fetch against this vendor. The Endpoint override keeps the
// family's path and points the host at the stub, which is also where the exchange
// goes, because the exchange host is derived from the endpoint's own origin.
func (v *qoderVendor) read(t *testing.T, creds Credentials) Result {
	t.Helper()
	creds.Endpoint = v.server.URL
	result := fetchQoder("qoder", "")(context.Background(), creds)
	return result
}

// TestFetchQoderReportsBothBuckets pins the read the panel needs: personal and
// organization as separate windows, the vendor's millisecond expiry carried onto both,
// and the absolute `remaining` kept out of numbers a card renders as a share.
func TestFetchQoderReportsBothBuckets(t *testing.T) {
	expires := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC).UnixMilli()
	body := fmt.Sprintf(`{"userQuota":{"total":500,"used":152,"remaining":348,"unit":"credits"},
		"orgResourcePackage":{"total":1000,"used":20,"remaining":980,"unit":"credits"},
		"totalUsagePercentage":14,"isQuotaExceeded":false,"expiresAt":%d}`, expires)
	vendor := newQoderVendor(t, body, 0)

	result := vendor.read(t, Credentials{AccessToken: "dt-device"})

	if len(result.Quotas) != 2 {
		t.Fatalf("quotas = %+v, want both buckets (%s)", result.Quotas, result.Message)
	}
	if got := result.Quotas[0]; got.Label != "Personal" || got.Used != 152 || got.Total != 500 {
		t.Fatalf("personal = %+v, want 152 of 500", got)
	}
	if got := result.Quotas[1]; got.Label != "Organization" || got.Used != 20 || got.Total != 1000 {
		t.Fatalf("organization = %+v", got)
	}
	want := time.UnixMilli(expires).UTC()
	for _, quota := range result.Quotas {
		if !quota.ResetAt.UTC().Equal(want) {
			t.Fatalf("%s reset = %v, want %v", quota.Label, quota.ResetAt, want)
		}
	}
	if strings.Contains(fmt.Sprint(result.Quotas), "348") {
		t.Fatalf("the absolute remaining leaked into the reported numbers: %+v", result.Quotas)
	}
	if vendor.bearer != "Bearer dt-device" {
		t.Fatalf("authorization = %q, want the stored device token presented as-is", vendor.bearer)
	}
	if vendor.exchanges != 0 {
		t.Fatalf("the token was exchanged %d times, want none for a device token", vendor.exchanges)
	}
}

// TestFetchQoderExchangesAPersonalAccessTokenFirst pins the credential rule: a raw
// `pt-` is never shown to the quota endpoint, which refuses it; the job token is.
func TestFetchQoderExchangesAPersonalAccessTokenFirst(t *testing.T) {
	vendor := newQoderVendor(t, `{"userQuota":{"total":100,"used":5}}`, 0)

	result := vendor.read(t, Credentials{APIKey: "pt-secret"})

	if len(result.Quotas) != 1 {
		t.Fatalf("quotas = %+v, want the personal bucket only (%s)", result.Quotas, result.Message)
	}
	if vendor.exchanges != 1 || vendor.exchanged != "pt-secret" {
		t.Fatalf("exchanges = %d of %q, want the stored personal token exchanged once", vendor.exchanges, vendor.exchanged)
	}
	if vendor.bearer != "Bearer jt-issued" {
		t.Fatalf("authorization = %q, want the exchanged job token", vendor.bearer)
	}
}

// TestFetchQoderReadsStringCounts pins the tolerance: the vendor answers counts as
// numbers on some shapes and numeric strings on others, and a read that only accepted
// one would drop the bucket rather than report it.
func TestFetchQoderReadsStringCounts(t *testing.T) {
	vendor := newQoderVendor(t, `{"userQuota":{"total":"1250.5","used":"12.25"}}`, 0)
	result := vendor.read(t, Credentials{AccessToken: "dt-x"})

	if len(result.Quotas) != 1 {
		t.Fatalf("quotas = %+v, want one bucket read from strings (%s)", result.Quotas, result.Message)
	}
	if result.Quotas[0].Total != 1250.5 || result.Quotas[0].Used != 12.25 {
		t.Fatalf("counts = %+v, want the string values parsed", result.Quotas[0])
	}
}

// TestFetchQoderSoftAnswers pins that a quota read never becomes a Go error: a missing
// credential, a refused call, and an account with no published allocation each answer
// the vendor's own sentence, which is what the card renders.
func TestFetchQoderSoftAnswers(t *testing.T) {
	cases := []struct {
		name   string
		vendor func(t *testing.T) *qoderVendor
		creds  Credentials
		want   string
	}{
		{
			name:   "no credential at all",
			vendor: func(t *testing.T) *qoderVendor { return newQoderVendor(t, `{}`, 0) },
			creds:  Credentials{},
			want:   "credential not available",
		},
		{
			name:   "the endpoint refuses the credential",
			vendor: func(t *testing.T) *qoderVendor { return newQoderVendor(t, `{}`, http.StatusForbidden) },
			creds:  Credentials{AccessToken: "dt-gone"},
			want:   "invalid or expired",
		},
		{
			name:   "a server error quotes the body",
			vendor: func(t *testing.T) *qoderVendor { return newQoderVendor(t, `{}`, http.StatusBadGateway) },
			creds:  Credentials{AccessToken: "dt-x"},
			want:   "502",
		},
		{
			name: "nothing is published",
			vendor: func(t *testing.T) *qoderVendor {
				return newQoderVendor(t, `{"userQuota":{"total":0,"used":0}}`, 0)
			},
			creds: Credentials{AccessToken: "dt-x"},
			want:  "no quota",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result := testCase.vendor(t).read(t, testCase.creds)
			if len(result.Quotas) != 0 {
				t.Fatalf("quotas = %+v, want a soft answer", result.Quotas)
			}
			if !strings.Contains(result.Message, testCase.want) {
				t.Fatalf("message = %q, want it to mention %q", result.Message, testCase.want)
			}
		})
	}
}

// TestQoderFamiliesAreDispatched pins the wiring: both regions resolve to a fetcher,
// so a Qoder connection is not reported as "not implemented" for a family that works.
func TestQoderFamiliesAreDispatched(t *testing.T) {
	for _, family := range []string{"qoder", "qoder-cn"} {
		if _, ok := familyFetchers[family]; !ok {
			t.Fatalf("family %q has no fetcher", family)
		}
		if endpoint := familyEndpoints[family]; !strings.HasPrefix(endpoint, "https://") {
			t.Fatalf("family %q endpoint = %q, want an absolute url", family, endpoint)
		}
	}
}
