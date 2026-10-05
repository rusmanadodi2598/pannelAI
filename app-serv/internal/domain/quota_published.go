// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/quota_published.go
// @for       The published-quota vocabulary: a provider's own buckets, the
//
//	scheduling state the poll worker reads, and the answer it stores.
//
// @uses      internal/domain (Decimal, AppError constructors), strings, time.
// @reason    The quota screen shows what a provider says about itself, and that
//
//	vocabulary is the provider's, not the gateway's: a bucket is named by
//	a label ("Claude & GPT (Weekly)"), so it cannot reuse QuotaWindowKind,
//	whose set is closed to the four windows this gateway accounts for
//	(migrations/000007). The rules the cache depends on, a missing ceiling
//	is not a zero ceiling, and one answer cannot carry the same label twice,
//	are domain decisions, so they live here rather than in the statement that
//	happens to store them (AGENTS.md §2.2).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-02
package domain

import (
	"strings"
	"time"
)

// PublishedState is one endpoint's scheduling record in the published-quota
// cache: what the last answer said in envelope terms, and when the poll worker
// should look again. It is the sweep's unit of work, which is why it carries the
// attempt columns the screen never reads.
//
// FetchedAt and LastAttemptAt are pointers because both are legitimately absent:
// an endpoint the worker has tried and never succeeded on has no answer to stamp,
// and a row created by a store has no attempt of its own. A zero time.Time would
// read as "the epoch", which a screen would render as an answer two decades stale.
type PublishedState struct {
	EndpointID          string
	ProviderID          string
	Plan                string
	Message             string
	FetchedAt           *time.Time
	LastAttemptAt       *time.Time
	NextAttemptAt       time.Time
	ConsecutiveFailures int
}

// PublishedWindowRow is one bucket a provider published, keyed by the provider's
// own label rather than by a window kind this gateway chose. Used and Total are
// decimal strings for the reason cost is: SPEC-API-001 §4 forbids a float on the
// wire, and the numeric(20, 6) column rounds a float the same way nobody asked
// for. A percentage is deliberately absent, the panel derives it, so changing
// that display rule is not a backfill.
type PublishedWindowRow struct {
	EndpointID string
	Label      string

	// Used is the consumed amount; the column is NOT NULL, so a provider that
	// names a bucket but reports no consumption stores zero, not nothing.
	Used string

	// Total is the ceiling. nil means the provider stated no ceiling at all
	// (unlimited); a "0" string means a ceiling that is fully spent. The two are
	// different facts and collapsing them is the bug this field exists to prevent.
	Total     *string
	Unlimited bool

	// IsCreditBalance marks a money balance rather than a capped window. It is not
	// the same fact as `Unlimited`: a prepaid wallet has a finite amount of money
	// and no periodic ceiling, and drawing either as a percentage of a total the
	// provider never claimed would render a balance as a spend share.
	IsCreditBalance bool
	Recurring       bool
	Unit            string
	ResetsAt        *time.Time
	FetchedAt       time.Time
}

// Ceiling returns the published ceiling and whether the provider stated one. The
// second value, not the first, distinguishes unlimited from exhausted; a caller
// that reads only the string cannot make that distinction, which is why this
// accessor, not the field, is the documented way to ask.
func (w PublishedWindowRow) Ceiling() (string, bool) {
	if w.Total == nil {
		return "", false
	}
	return *w.Total, true
}

// PublishedQuota is one endpoint's whole cached answer: its state plus the
// buckets that answer published. An endpoint the cache has never heard of is
// absent from the collection rather than present and empty, so the read side can
// tell "no provider answer yet" from "the provider published zero buckets".
type PublishedQuota struct {
	State   PublishedState
	Windows []PublishedWindowRow
}

// PublishedAttempt is one poll's scheduling outcome: the instant it ran, when to run
// again, how the failure run moved, and, the part that is not scheduling, the
// sentence the provider answered with when it answered with no buckets at all.
//
// Plan and Message are pointers rather than strings because "this poll stated nothing
// new" and "this poll stated the empty string" are different claims: a sentence the
// provider never sent must not reach the card wearing the provider's words.
type PublishedAttempt struct {
	EndpointID    string
	ProviderID    string
	AttemptedAt   time.Time
	NextAttemptAt time.Time
	FailureDelta  int

	Plan    *string
	Message *string
}

// Validate refuses an attempt that cannot be scheduled: without an endpoint or a
// provider there is no row to write, and without an instant there is nothing to age.
func (a PublishedAttempt) Validate() error {
	if strings.TrimSpace(a.EndpointID) == "" {
		return NewValidationError("published quota attempt requires endpoint_id")
	}
	if strings.TrimSpace(a.ProviderID) == "" {
		return NewValidationError("published quota attempt requires provider_id")
	}
	if a.AttemptedAt.IsZero() || a.NextAttemptAt.IsZero() {
		return NewValidationError("published quota attempt requires an attempt instant and a next one")
	}
	// A negative run would shorten every backoff interval computed from it, so it is
	// refused rather than clamped; the migration's CHECK is the backstop behind this.
	if a.FailureDelta < 0 {
		return NewValidationError("published quota failure delta must not be negative")
	}
	return nil
}

// PublishedAnswer is what the poll worker stores for one endpoint: the envelope
// the provider answered with and the buckets in it.
//
// It names no attempt columns on purpose. Storing an answer is not a scheduling
// decision, the worker decides the next interval and records it through
// RecordAttempt, so a write DTO that could carry NextAttemptAt would let a
// caller overwrite the schedule with a value it read a poll ago.
type PublishedAnswer struct {
	EndpointID string
	ProviderID string
	Plan       string
	Message    string
	FetchedAt  time.Time
	Windows    []PublishedWindowRow
}

// Labels returns the bucket labels this answer carries, in answer order. The
// prune that deletes a renamed bucket is bounded by exactly this set, so it is
// derived from the answer rather than kept as a second list a caller can forget
// to update.
func (a PublishedAnswer) Labels() []string {
	labels := make([]string, 0, len(a.Windows))
	for _, window := range a.Windows {
		labels = append(labels, window.Label)
	}
	return labels
}

// Validate checks the answer against the rules the cache is built on, before any
// statement runs. A duplicate label would make one INSERT ... ON CONFLICT try to
// update the same row twice, and a blank label would store a bucket nothing can
// refer to later; both are the writer's bug, so they are refused here in English
// rather than surfacing as a database error code (AGENTS.md §1.3).
func (a PublishedAnswer) Validate() error {
	if strings.TrimSpace(a.EndpointID) == "" {
		return NewValidationError("published quota endpoint_id is required")
	}
	if strings.TrimSpace(a.ProviderID) == "" {
		return NewValidationError("published quota provider_id is required")
	}
	if a.FetchedAt.IsZero() {
		return NewValidationError("published quota fetched_at is required")
	}

	seen := make(map[string]struct{}, len(a.Windows))
	for _, window := range a.Windows {
		label := strings.TrimSpace(window.Label)
		if label == "" {
			return NewValidationError("published quota window label is required")
		}
		if window.EndpointID != "" && window.EndpointID != a.EndpointID {
			return NewValidationError("published quota window belongs to another endpoint")
		}
		if _, dup := seen[label]; dup {
			return NewValidationError("published quota window label is duplicated: " + label)
		}
		seen[label] = struct{}{}
		if err := validatePublishedAmount(window.Used, true); err != nil {
			return err
		}
		if err := validatePublishedAmount(derefAmount(window.Total), false); err != nil {
			return err
		}
	}
	return nil
}

// validatePublishedAmount checks one published amount parses as a decimal and is
// not negative. A provider that reports negative remaining is normalised by the
// adapter that read it, never by the cache, so a negative here means the worker
// built the answer wrong. An absent ceiling (empty and not required) is valid.
func validatePublishedAmount(value string, required bool) error {
	if value == "" {
		if required {
			return NewValidationError("published quota window used is required")
		}
		return nil
	}
	amount, err := ParseDecimal(value)
	if err != nil {
		return NewValidationError("published quota window amount is not a decimal: " + value)
	}
	if amount.IsNegative() {
		return NewValidationError("published quota window amount is negative: " + value)
	}
	return nil
}

// derefAmount flattens an optional ceiling for validation.
func derefAmount(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
