// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/quota_published_test.go
// @for       Table-driven tests for the published-quota answer rules: the label
//
//	set one answer may hold, the amounts it may carry, and the ceiling accessor
//	that separates "no ceiling" from "a spent one".
//
// @uses      testing, time.
// @reason    The cache is written by a worker that has no view of the schema, so
//
//	the rules that keep a stored answer readable, one label per bucket, no
//	negative amount, an endpoint id that is not blank, have to be enforced
//	where the vocabulary lives, not in the statement that happens to store it
//	(AGENTS.md §2.2). A duplicate label in particular turns one batched upsert
//	into a PostgreSQL error about affecting a row twice, which is a database
//	message a client must never see (§1.3).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-02
package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// publishedTestNow is one instant, so a zero FetchedAt case is the only row in
// the table that fails on time.
var publishedTestNow = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

// TestPublishedAnswerValidate pins the answer shapes the cache refuses. Each row
// is a mistake a poll adapter can plausibly make while reading a vendor payload.
func TestPublishedAnswerValidate(t *testing.T) {
	ceiling := "100"
	cases := []struct {
		name    string
		answer  PublishedAnswer
		wantErr string
	}{
		{
			name:    "a blank endpoint is not an owner for the answer",
			answer:  PublishedAnswer{ProviderID: "alpha", FetchedAt: publishedTestNow},
			wantErr: "endpoint_id",
		},
		{
			name:    "a blank provider cannot be scheduled",
			answer:  PublishedAnswer{EndpointID: "ep_a", FetchedAt: publishedTestNow},
			wantErr: "provider_id",
		},
		{
			name:    "an answer without an instant was never fetched",
			answer:  PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha"},
			wantErr: "fetched_at",
		},
		{
			name: "a blank label cannot be referred to later",
			answer: PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow,
				Windows: []PublishedWindowRow{{EndpointID: "ep_a", Label: "  ", Used: "1"}}},
			wantErr: "label is required",
		},
		{
			name: "one answer cannot carry the same label twice",
			answer: PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow,
				Windows: []PublishedWindowRow{
					{EndpointID: "ep_a", Label: "Weekly", Used: "1"},
					{EndpointID: "ep_a", Label: "Weekly", Used: "2"},
				}},
			wantErr: "duplicated",
		},
		{
			name: "a bucket cannot belong to another endpoint",
			answer: PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow,
				Windows: []PublishedWindowRow{{EndpointID: "ep_other", Label: "Weekly", Used: "1"}}},
			wantErr: "another endpoint",
		},
		{
			name: "a used amount is required",
			answer: PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow,
				Windows: []PublishedWindowRow{{EndpointID: "ep_a", Label: "Weekly"}}},
			wantErr: "used is required",
		},
		{
			name: "a non-decimal amount is refused",
			answer: PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow,
				Windows: []PublishedWindowRow{{EndpointID: "ep_a", Label: "Weekly", Used: "lots"}}},
			wantErr: "not a decimal",
		},
		{
			name: "a negative ceiling is the worker's bug, not data",
			answer: PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow,
				Windows: []PublishedWindowRow{{EndpointID: "ep_a", Label: "Weekly", Used: "1", Total: strPtr("-1")}}},
			wantErr: "negative",
		},
		{
			name:   "an answer with no buckets is valid",
			answer: PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow},
		},
		{
			name: "a capped and an uncapped bucket in one answer is valid",
			answer: PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow,
				Windows: []PublishedWindowRow{
					{EndpointID: "ep_a", Label: "Weekly", Used: "1", Total: &ceiling},
					{EndpointID: "ep_a", Label: "Daily", Used: "1", Total: nil},
				}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.answer.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want this answer accepted", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want it refused for %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %q, want it to mention %q", err, tc.wantErr)
			}
			// A writer's mistake is a 400-shaped validation error, never a bare
			// fmt error that a handler would have to guess a code for (§1.3).
			var appErr *AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("Validate() error = %T, want a *domain.AppError", err)
			}
			if appErr.Code != "VALIDATION_ERROR" {
				t.Fatalf("Validate() code = %q, want VALIDATION_ERROR", appErr.Code)
			}
		})
	}
}

// TestPublishedWindowRowCeilingKeepsNullApartFromZero is the accessor-level form
// of the rule the migration spells out in comments: an unstated ceiling and a
// ceiling of zero are different facts, and only the pointer says which.
func TestPublishedWindowRowCeilingKeepsNullApartFromZero(t *testing.T) {
	unlimited := PublishedWindowRow{Label: "unlimited", Used: "7", Total: nil}
	if value, ok := unlimited.Ceiling(); ok || value != "" {
		t.Fatalf("Ceiling() = %q/%v, want empty and false for an unstated ceiling", value, ok)
	}

	spent := PublishedWindowRow{Label: "spent", Used: "500", Total: strPtr("0")}
	if value, ok := spent.Ceiling(); !ok || value != "0" {
		t.Fatalf("Ceiling() = %q/%v, want the stored zero", value, ok)
	}
}

// TestPublishedAnswerLabelsAreThePruneSet checks the set the store's delete is
// bounded by: it is exactly the labels the answer carries, in order, and an
// answer with no buckets yields an empty list rather than a nil that a caller
// could read as "prune nothing".
func TestPublishedAnswerLabelsAreThePruneSet(t *testing.T) {
	answer := PublishedAnswer{
		EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: publishedTestNow,
		Windows: []PublishedWindowRow{
			{EndpointID: "ep_a", Label: "Weekly"},
			{EndpointID: "ep_a", Label: "Daily"},
		},
	}
	if got := answer.Labels(); len(got) != 2 || got[0] != "Weekly" || got[1] != "Daily" {
		t.Fatalf("Labels() = %v, want the answer's own labels in order", got)
	}
	if got := (PublishedAnswer{}).Labels(); got == nil || len(got) != 0 {
		t.Fatalf("Labels() of an empty answer = %v, want an empty non-nil set", got)
	}
}

// strPtr is the one-line pointer helper the table rows need for an optional
// ceiling; a field cannot take a literal address inline.
func strPtr(value string) *string { return &value }
