// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/quota_accounting_test.go
// @for       Table-driven tests for the accounting window set and the reset
//
//	instant one served request records (register G22).
//
// @uses      testing, time.
// @reason    AGENTS.md §2.1 and §2.4 require the rule that decides which windows
//
//	a request bills against to be pinned: if a window were missing from
//	the set, the counter the panel reads for it would stay at zero while
//	the flush worker kept draining the others, and no test elsewhere
//	would notice.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-20
package domain

import (
	"testing"
	"time"
)

// TestAccountingKinds_IsTheStoredWindowSet pins that the windows a request
// advances are exactly the four the schema stores. A kind added to the CHECK
// constraint but not to this set would be silently unaccounted.
func TestAccountingKinds_IsTheStoredWindowSet(t *testing.T) {
	cases := []struct {
		name string
		kind QuotaWindowKind
	}{
		{"five hour", QuotaWindowFiveHour},
		{"daily", QuotaWindowDaily},
		{"weekly", QuotaWindowWeekly},
		{"monthly", QuotaWindowMonthly},
	}
	kinds := AccountingKinds()
	if len(kinds) != len(cases) {
		t.Fatalf("AccountingKinds() = %d windows, want %d", len(kinds), len(cases))
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if kinds[i] != tc.kind {
				t.Fatalf("AccountingKinds()[%d] = %q, want %q", i, kinds[i], tc.kind)
			}
			if !kinds[i].IsValid() {
				t.Fatalf("AccountingKinds()[%d] = %q is not a stored window", i, kinds[i])
			}
		})
	}
}

// TestQuotaWindowKind_NextReset covers the reset instant a request records,
// including the zero case for a kind that has no length. A reset in the past
// would make every Add roll the counter over and lose the usage it just counted.
func TestQuotaWindowKind_NextReset(t *testing.T) {
	now := time.Date(2026, 9, 20, 10, 30, 0, 0, time.UTC)

	cases := []struct {
		name  string
		kind  QuotaWindowKind
		want  time.Time
		equal bool
	}{
		{"five hour", QuotaWindowFiveHour, now.Add(5 * time.Hour), false},
		{"daily", QuotaWindowDaily, now.Add(24 * time.Hour), false},
		{"weekly", QuotaWindowWeekly, now.Add(7 * 24 * time.Hour), false},
		{"monthly", QuotaWindowMonthly, now.Add(30 * 24 * time.Hour), false},
		{"unknown kind keeps the instant it was given", "hourly", now, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.kind.NextReset(now)
			if !got.Equal(tc.want) {
				t.Fatalf("NextReset(%q) = %s, want %s", tc.kind, got, tc.want)
			}
			if got.Before(now) {
				t.Fatalf("NextReset(%q) = %s is before now %s, which would roll the counter on every add",
					tc.kind, got, now)
			}
		})
	}
}

// TestQuotaWindowKind_NextResetNormalizesToUTC pins that the instant is stored
// in UTC whatever zone the caller's clock reported, so a comparison against a
// stored timestamp does not depend on which replica answered.
func TestQuotaWindowKind_NextResetNormalizesToUTC(t *testing.T) {
	jakarta := time.FixedZone("WIB", 7*60*60)
	now := time.Date(2026, 9, 20, 17, 30, 0, 0, jakarta)

	got := QuotaWindowDaily.NextReset(now)
	if got.Location() != time.UTC {
		t.Fatalf("NextReset() location = %s, want UTC", got.Location())
	}
	if want := now.UTC().Add(24 * time.Hour); !got.Equal(want) {
		t.Fatalf("NextReset() = %s, want %s", got, want)
	}
}
