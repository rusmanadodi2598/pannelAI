// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/quota_test.go
// @for       Table-driven tests for quota windows, rollover, and budget caps.
// @uses      testing, time.
// @reason    AGENTS.md §2.1 and §2.4 require the rules the router reads to be
//
//	pinned: whether a window is exhausted decides if an endpoint is
//	skipped, and whether a rollover resets the counter decides whether a
//	provider's quota is counted twice.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"testing"
	"time"
)

// TestQuotaWindowKind_Set covers the closed window set and its durations.
func TestQuotaWindowKind_Set(t *testing.T) {
	cases := []struct {
		name         string
		in           string
		wantValid    bool
		wantDuration time.Duration
	}{
		{"five hour", "5h", true, 5 * time.Hour},
		{"daily", "daily", true, 24 * time.Hour},
		{"weekly", "weekly", true, 7 * 24 * time.Hour},
		{"monthly", "monthly", true, 30 * 24 * time.Hour},
		{"hourly is not a stored window", "hourly", false, 0},
		{"empty", "", false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind := QuotaWindowKind(tc.in)
			if got := kind.IsValid(); got != tc.wantValid {
				t.Fatalf("IsValid(%q) = %v, want %v", tc.in, got, tc.wantValid)
			}
			if got := kind.Duration(); got != tc.wantDuration {
				t.Fatalf("Duration(%q) = %s, want %s", tc.in, got, tc.wantDuration)
			}
		})
	}
}

// TestQuotaWindow_Add covers the counter arithmetic at the rollover boundary,
// which is where a provider's forgiven usage would otherwise be re-counted.
func TestQuotaWindow_Add(t *testing.T) {
	resetAt := time.Date(2026, 9, 18, 13, 0, 0, 0, time.UTC)
	beforeReset := resetAt.Add(-time.Minute)
	afterReset := resetAt.Add(time.Minute)
	five := int64(5)

	cases := []struct {
		name      string
		window    QuotaWindow
		units     int64
		now       time.Time
		wantUsed  int64
		wantReset *time.Time
	}{
		{
			name:      "usage accumulates before the reset",
			window:    mustWindow(t, "ep_1", QuotaWindowDaily, &five, &resetAt),
			units:     3,
			now:       beforeReset,
			wantUsed:  3,
			wantReset: &resetAt,
		},
		{
			name:      "a rolled window restarts at the amount just consumed",
			window:    withUsage(t, "ep_1", QuotaWindowDaily, &five, &resetAt, 5),
			units:     2,
			now:       afterReset,
			wantUsed:  2,
			wantReset: ptrTime(afterReset.Add(24 * time.Hour)),
		},
		{
			name:      "zero units change nothing",
			window:    withUsage(t, "ep_1", QuotaWindowDaily, &five, &resetAt, 4),
			units:     0,
			now:       beforeReset,
			wantUsed:  4,
			wantReset: &resetAt,
		},
		{
			name:      "negative units change nothing",
			window:    withUsage(t, "ep_1", QuotaWindowDaily, &five, &resetAt, 4),
			units:     -3,
			now:       beforeReset,
			wantUsed:  4,
			wantReset: &resetAt,
		},
		{
			name:      "a window with no reset instant never rolls",
			window:    mustWindow(t, "ep_1", QuotaWindowFiveHour, nil, nil),
			units:     7,
			now:       afterReset,
			wantUsed:  7,
			wantReset: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			window := tc.window
			window.Add(tc.units, tc.now)
			if window.Used() != tc.wantUsed {
				t.Fatalf("Used = %d, want %d", window.Used(), tc.wantUsed)
			}
			got := window.ResetsAt()
			switch {
			case tc.wantReset == nil && got != nil:
				t.Fatalf("ResetsAt = %s, want nil", got)
			case tc.wantReset != nil && got == nil:
				t.Fatalf("ResetsAt = nil, want %s", tc.wantReset)
			case tc.wantReset != nil && !got.Equal(*tc.wantReset):
				t.Fatalf("ResetsAt = %s, want %s", got, tc.wantReset)
			}
		})
	}
}

// TestQuotaWindow_Exhausted covers the rule the router skips an endpoint on,
// including the uncapped case and the exact-ceiling boundary.
func TestQuotaWindow_Exhausted(t *testing.T) {
	limit := int64(10)
	zero := int64(0)

	cases := []struct {
		name            string
		window          QuotaWindow
		wantExhausted   bool
		wantRemaining   int64
		wantRemainingOK bool
	}{
		{"uncapped is never exhausted", mustWindow(t, "ep_1", QuotaWindowDaily, nil, nil), false, 0, false},
		{"below the ceiling", withUsage(t, "ep_1", QuotaWindowDaily, &limit, nil, 4), false, 6, true},
		{"exactly at the ceiling", withUsage(t, "ep_1", QuotaWindowDaily, &limit, nil, 10), true, 0, true},
		{"over the ceiling", withUsage(t, "ep_1", QuotaWindowDaily, &limit, nil, 11), true, 0, true},
		{"a zero ceiling is exhausted at zero", withUsage(t, "ep_1", QuotaWindowDaily, &zero, nil, 0), true, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.window.Exhausted(); got != tc.wantExhausted {
				t.Fatalf("Exhausted = %v, want %v", got, tc.wantExhausted)
			}
			remaining, ok := tc.window.Remaining()
			if ok != tc.wantRemainingOK || remaining != tc.wantRemaining {
				t.Fatalf("Remaining = (%d, %v), want (%d, %v)", remaining, ok, tc.wantRemaining, tc.wantRemainingOK)
			}
		})
	}
}

// TestNewQuotaWindow covers window construction validation.
func TestNewQuotaWindow(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	limit := int64(10)
	negative := int64(-1)

	cases := []struct {
		name       string
		endpointID string
		kind       QuotaWindowKind
		limit      *int64
		wantErr    bool
	}{
		{"valid window", "ep_1", QuotaWindowDaily, &limit, false},
		{"uncapped window", "ep_1", QuotaWindowDaily, nil, false},
		{"zero limit is allowed", "ep_1", QuotaWindowFiveHour, ptrInt64(0), false},
		{"missing endpoint", "", QuotaWindowDaily, nil, true},
		{"unknown window kind", "ep_1", "hourly", nil, true},
		{"negative limit", "ep_1", QuotaWindowDaily, &negative, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			window, err := NewQuotaWindow(tc.endpointID, "provider", tc.kind, tc.limit, nil, now)
			if tc.wantErr {
				if err == nil {
					t.Fatal("NewQuotaWindow = nil error, want a rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewQuotaWindow error = %v", err)
			}
			if window.Source() != QuotaSourceComputed {
				t.Fatalf("Source = %q, want computed", window.Source())
			}
			if window.Used() != 0 {
				t.Fatalf("Used = %d, want 0 for a fresh window", window.Used())
			}
		})
	}
}

// mustWindow builds a window for a table case, failing the test on a rejected
// construction.
func mustWindow(t *testing.T, endpointID string, kind QuotaWindowKind, limit *int64, resetsAt *time.Time) QuotaWindow {
	t.Helper()
	window, err := NewQuotaWindow(endpointID, "provider", kind, limit, resetsAt, time.Now())
	if err != nil {
		t.Fatalf("NewQuotaWindow(%s) error = %v", endpointID, err)
	}
	return window
}

// withUsage builds a window already carrying usage, which the table cases need
// to reach the exhausted and rollover boundaries.
func withUsage(t *testing.T, endpointID string, kind QuotaWindowKind, limit *int64, resetsAt *time.Time, used int64) QuotaWindow {
	t.Helper()
	window := mustWindow(t, endpointID, kind, limit, resetsAt)
	window.used = used
	return window
}

// mustCap builds a cap for a table case.
func mustCap(t *testing.T, endpointID string, cost *Decimal, tokens *int64) QuotaCap {
	t.Helper()
	cap, err := NewQuotaCap(endpointID, cost, tokens, time.Now())
	if err != nil {
		t.Fatalf("NewQuotaCap(%s) error = %v", endpointID, err)
	}
	return cap
}

// decimalPtr parses a decimal for a table case.
func decimalPtr(t *testing.T, value string) *Decimal {
	t.Helper()
	parsed, err := ParseDecimal(value)
	if err != nil {
		t.Fatalf("ParseDecimal(%q) error = %v", value, err)
	}
	return &parsed
}

// ptrInt64 returns a pointer to an int64 for a table case.
func ptrInt64(value int64) *int64 { return &value }

// ptrTime returns a pointer to an instant for a table case.
func ptrTime(value time.Time) *time.Time { return &value }
