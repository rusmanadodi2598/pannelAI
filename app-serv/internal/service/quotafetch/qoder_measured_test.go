// Package quotafetch reads the quota a provider publishes for one of its connections.
//
// @file      internal/service/quotafetch/qoder_measured_test.go
// @for       The Qoder quota mapping, pinned against the answer the live service
//
//	actually gave.
//
// @uses      encoding/json, fmt, strings, testing, time.
// @reason    Every one of these cases came from a service answer rather than from
//
//	the reference's source: float-shaped counts, a plan name in `userType`,
//	an exceeded account that publishes only zeros, and a year-9999 reset
//	sentinel. An assumption about any of them is worth a named case,
//	because each one fails quietly on a card.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-28
package quotafetch

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// qoderMeasuredAnswer is the real answer the quota endpoint gave on 2026-09-28 for a
// credits plan whose allocation is spent: numbers arrive with a `.0`, the plan name is
// `userType`, the reset is a year-9999 sentinel, and there is no organization bucket
// at all. It is kept here verbatim because each of those four facts broke an
// assumption made from the reference's source rather than from the service.
const qoderMeasuredAnswer = `{"userId":"019f76f3-2cdd","userType":"personal_standard","usageType":"credits",
 "totalUsagePercentage":0.0,"isQuotaExceeded":true,"expiresAt":253402214400000,
 "upgradeUrl":"https://qoder.com/pricing?client=qoder","outerProviders":[],
 "userQuota":{"total":0.0,"used":0.0,"remaining":0.0,"percentage":0.0,"unit":"credits"},
 "isPlanQuotaProrated":false}`

// TestFetchQoderReadsTheMeasuredAnswer pins the three things the live service
// actually does: it names the plan, it reports an exceeded account with no published
// allocation as exceeded rather than as "nothing published", and it drops the
// sentinel reset date instead of showing a card that expires in the year 9999.
func TestFetchQoderReadsTheMeasuredAnswer(t *testing.T) {
	vendor := newQoderVendor(t, qoderMeasuredAnswer, 0)

	result := vendor.read(t, Credentials{APIKey: "pt-secret"})

	if result.Plan != "personal_standard" {
		t.Fatalf("plan = %q, want the vendor's userType", result.Plan)
	}
	if len(result.Quotas) != 0 {
		t.Fatalf("quotas = %+v, want no zero bucket reported", result.Quotas)
	}
	if !strings.Contains(result.Message, "exceeded") {
		t.Fatalf("message = %q, want the exceeded fact", result.Message)
	}
}

// TestFetchQoderReportsAPublishedAllocation is the same shape with credits in it, so
// the plan and reset rules are checked against an account that does have a number.
func TestFetchQoderReportsAPublishedAllocation(t *testing.T) {
	resets := time.Now().Add(20 * 24 * time.Hour).UnixMilli()
	vendor := newQoderVendor(t, fmt.Sprintf(
		`{"userType":"pro","isQuotaExceeded":false,"expiresAt":%d,
		  "userQuota":{"total":100.0,"used":40.0,"remaining":60.0,"unit":"credits"}}`, resets), 0)

	result := vendor.read(t, Credentials{AccessToken: "dt-device"})

	if len(result.Quotas) != 1 {
		t.Fatalf("quotas = %+v, want the personal bucket (%s)", result.Quotas, result.Message)
	}
	got := result.Quotas[0]
	if got.Total != 100 || got.Used != 40 {
		t.Fatalf("counts = %+v, want 40 of 100", got)
	}
	if got.ResetAt.UnixMilli() != resets {
		t.Fatalf("reset = %v, want the published instant", got.ResetAt)
	}
	if got.Recurring {
		t.Fatal("a published expiry is not a recurring window; the row must not claim one")
	}
}

// TestQoderResetDropsTheSentinel pins the horizon rule directly, including the
// absence of a reset and a value just inside the horizon.
func TestQoderResetDropsTheSentinel(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"the year-9999 sentinel", `253402214400000`, false},
		{"a real future date", fmt.Sprint(time.Now().Add(48 * time.Hour).UnixMilli()), true},
		{"absent", `null`, false},
		{"zero", `0`, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := qoderReset(json.RawMessage(testCase.raw))
			if got.IsZero() == testCase.want {
				t.Fatalf("reset %q = %v, want reported=%v", testCase.raw, got, testCase.want)
			}
		})
	}
}
