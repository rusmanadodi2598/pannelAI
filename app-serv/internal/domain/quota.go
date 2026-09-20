// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/quota.go
// @for       The quota vocabulary: window kinds, a usage window, and the budget
//
//	cap that makes the router skip an exhausted endpoint.
//
// @uses      internal/domain (Decimal, AppError constructors), time.
// @reason    SPEC-API-001 §7.12 exposes windows and caps and §5 makes a window
//
//	"usage per endpoint with a reset instant"; the exhausted rule is a
//	domain decision the router depends on, so it lives here rather than
//	in the query that happens to read the counters.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import "time"

// QuotaWindowKind is one accounting window. The set is closed and mirrors the
// CHECK constraint on quota_windows.window (SPEC-API-001 §6).
type QuotaWindowKind string

const (
	// QuotaWindowFiveHour is the provider-side rolling five-hour window.
	QuotaWindowFiveHour QuotaWindowKind = "5h"
	// QuotaWindowDaily is the calendar day window.
	QuotaWindowDaily QuotaWindowKind = "daily"

	// QuotaWindowWeekly is the calendar week window.
	QuotaWindowWeekly QuotaWindowKind = "weekly"
	// QuotaWindowMonthly is the calendar month window, which a token or cost
	// cap is compared against.
	QuotaWindowMonthly QuotaWindowKind = "monthly"
)

// ParseQuotaWindowKind validates a wire value before it reaches an entity.
func ParseQuotaWindowKind(value string) (QuotaWindowKind, error) {
	kind := QuotaWindowKind(value)
	if !kind.IsValid() {
		return "", NewValidationError("invalid quota window: " + value)
	}
	return kind, nil
}

// IsValid reports whether the kind is one the schema stores.
func (k QuotaWindowKind) IsValid() bool {
	switch k {
	case QuotaWindowFiveHour, QuotaWindowDaily, QuotaWindowWeekly, QuotaWindowMonthly:
		return true
	default:
		return false
	}
}

// Duration is the window's length, used to compute the next reset when the
// provider has not reported one.
func (k QuotaWindowKind) Duration() time.Duration {
	switch k {
	case QuotaWindowFiveHour:
		return 5 * time.Hour
	case QuotaWindowDaily:
		return 24 * time.Hour
	case QuotaWindowWeekly:
		return 7 * 24 * time.Hour
	case QuotaWindowMonthly:
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

// NextReset is the instant this window rolls over at, given the moment it was
// opened. Every window the gateway accounts for is rolling: the provider counts
// a five-hour window from its start, and the calendar windows are measured as
// fixed lengths from the instant the first call opened them. Computing it from
// the caller's clock keeps the rule in one place instead of at each Add site.
func (k QuotaWindowKind) NextReset(now time.Time) time.Time {
	length := k.Duration()
	if length <= 0 {
		return now.UTC()
	}
	return now.UTC().Add(length)
}

// AccountingKinds are the windows one served request bills against, in the
// order they are written. A request advances every window it falls inside, so
// the five-hour, daily, weekly, and monthly counters stay comparable: reporting
// only the narrowest would make the panel's monthly total disagree with the
// month-to-date figure the budget cap is compared against (§7.12).
//
// The set is fixed rather than configurable per provider because the spec fixes
// the four kinds (SPEC-API-001 §6 CHECK constraint); a provider that reports its
// own cadence does so through Report, which replaces a counter instead of adding.
func AccountingKinds() []QuotaWindowKind {
	return []QuotaWindowKind{QuotaWindowFiveHour, QuotaWindowDaily, QuotaWindowWeekly, QuotaWindowMonthly}
}

// QuotaSource distinguishes a number this gateway counted from one the provider
// reported. The distinction is functional, not decorative: a computed counter
// can be reconciled from usage_records while a reported one cannot.
type QuotaSource string

const (
	// QuotaSourceComputed is a counter this gateway accumulated.
	QuotaSourceComputed QuotaSource = "computed"
	// QuotaSourceReported is a counter the provider published.
	QuotaSourceReported QuotaSource = "reported"
)

// IsValid reports whether the source is one the schema stores.
func (s QuotaSource) IsValid() bool {
	return s == QuotaSourceComputed || s == QuotaSourceReported
}

// QuotaWindow is one endpoint's usage inside one window. Fields are unexported
// on purpose (AGENTS.md §2.2): a counter moves through Add, not by assignment.
type QuotaWindow struct {
	endpointID string
	providerID string
	kind       QuotaWindowKind
	used       int64
	limit      *int64
	resetsAt   *time.Time
	source     QuotaSource
	updatedAt  time.Time
}

// NewQuotaWindow builds a window with no usage yet.
func NewQuotaWindow(endpointID, providerID string, kind QuotaWindowKind, limit *int64, resetsAt *time.Time, now time.Time) (QuotaWindow, error) {
	if endpointID == "" {
		return QuotaWindow{}, NewValidationError("endpoint_id is required")
	}
	if !kind.IsValid() {
		return QuotaWindow{}, NewValidationError("invalid quota window: " + string(kind))
	}
	if limit != nil && *limit < 0 {
		return QuotaWindow{}, NewValidationError("limit must not be negative")
	}
	return QuotaWindow{
		endpointID: endpointID,
		providerID: providerID,
		kind:       kind,
		limit:      limit,
		resetsAt:   resetsAt,
		source:     QuotaSourceComputed,
		updatedAt:  now.UTC(),
	}, nil
}

// RehydrateQuotaWindow rebuilds a stored row. It is for the repository load
// path only; never use it to open a window.
func RehydrateQuotaWindow(endpointID, providerID string, kind QuotaWindowKind, used int64, limit *int64, resetsAt *time.Time, source QuotaSource, updatedAt time.Time) QuotaWindow {
	return QuotaWindow{
		endpointID: endpointID,
		providerID: providerID,
		kind:       kind,
		used:       used,
		limit:      limit,
		resetsAt:   resetsAt,
		source:     source,
		updatedAt:  updatedAt.UTC(),
	}
}

// Accessors expose the state the read side needs without allowing mutation.
func (w QuotaWindow) EndpointID() string      { return w.endpointID }
func (w QuotaWindow) ProviderID() string      { return w.providerID }
func (w QuotaWindow) Window() QuotaWindowKind { return w.kind }
func (w QuotaWindow) Used() int64             { return w.used }
func (w QuotaWindow) Source() QuotaSource     { return w.source }
func (w QuotaWindow) UpdatedAt() time.Time    { return w.updatedAt }
func (w QuotaWindow) Limit() (int64, bool) {
	if w.limit == nil {
		return 0, false
	}
	return *w.limit, true
}
func (w QuotaWindow) ResetsAt() *time.Time { return w.resetsAt }

// Add records usage against the window. A reset instant in the past means the
// window rolled over, so the counter restarts at the amount just consumed
// rather than being added to a total the provider has already forgiven.
func (w *QuotaWindow) Add(units int64, now time.Time) {
	if units <= 0 {
		return
	}
	if w.resetsAt != nil && !now.Before(*w.resetsAt) {
		w.used = 0
		if next := w.kind.Duration(); next > 0 {
			rolled := now.Add(next).UTC()
			w.resetsAt = &rolled
		}
	}
	w.used += units
	w.updatedAt = now.UTC()
}

// Report replaces the counter with a provider-reported value and marks the
// source, because a reported number is authoritative and additive updates to it
// would double-count.
func (w *QuotaWindow) Report(used int64, resetsAt *time.Time, now time.Time) {
	if used < 0 {
		used = 0
	}
	w.used = used
	w.resetsAt = resetsAt
	w.source = QuotaSourceReported
	w.updatedAt = now.UTC()
}

// Exhausted reports whether the window has reached its ceiling. A window with
// no limit is never exhausted, which is the difference between "uncapped" and
// "used up".
func (w QuotaWindow) Exhausted() bool {
	if w.limit == nil {
		return false
	}
	return w.used >= *w.limit
}

// Remaining is the units left before the ceiling, or the second return being
// false when the window is uncapped.
func (w QuotaWindow) Remaining() (int64, bool) {
	if w.limit == nil {
		return 0, false
	}
	if *w.limit <= w.used {
		return 0, true
	}
	return *w.limit - w.used, true
}

// ErrQuotaWindowNotFound is the sentinel a missing quota window maps to.
var ErrQuotaWindowNotFound = NewNotFoundError("quota window not found")
