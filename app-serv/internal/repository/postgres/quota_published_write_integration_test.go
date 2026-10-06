//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published_write_integration_test.go
// @for       The published-quota store against a real server: the label prune and the batched write's statement count.
// @uses      testing, context, time, internal/domain.
// @reason    The worker is the only thing that keeps this cache honest, and its two guarantees are constraint-level: a renamed bucket must vanish rather than linger beside its replacement, and one poll must cost one transaction whatever the bucket count. Neither is visible to a stub, and the cascade the prune and the upsert depend on only proves itself against a real server (AGENTS.md §1.7, §2.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-02
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestPublishedQuotaRepository_StorePrunesALabelTheNewAnswerDrops proves the
// prune: a provider renaming a bucket must not leave the old total on screen.
func TestPublishedQuotaRepository_StorePrunesALabelTheNewAnswerDrops(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()

	seedPublishedEndpoint(t, repo, "ep_a", "alpha")
	if err := repo.StorePublished(ctx, publishedAnswer("ep_a", "alpha", now, "Weekly limit", "Daily limit")); err != nil {
		t.Fatalf("first StorePublished() error = %v", err)
	}
	renamed := publishedAnswer("ep_a", "alpha", now.Add(time.Minute), "Claude & GPT (Weekly)")
	if err := repo.StorePublished(ctx, renamed); err != nil {
		t.Fatalf("second StorePublished() error = %v", err)
	}

	stored, err := repo.ListPublishedByEndpointIDs(ctx, []string{"ep_a"})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs() error = %v", err)
	}
	windows := stored["ep_a"].Windows
	if len(windows) != 1 {
		labels := make([]string, 0, len(windows))
		for _, window := range windows {
			labels = append(labels, window.Label)
		}
		t.Fatalf("after the rename the cache holds %v, want only the renamed bucket", labels)
	}
	if windows[0].Label != "Claude & GPT (Weekly)" {
		t.Fatalf("surviving bucket = %q, want the label the newest answer carried", windows[0].Label)
	}
	if !windows[0].FetchedAt.After(now) {
		t.Fatalf("surviving bucket fetched_at = %v, want the newest answer's instant", windows[0].FetchedAt)
	}

	// An answer that publishes nothing clears the buckets but keeps the envelope:
	// the endpoint still has a plan and a fetched_at to show.
	if err := repo.StorePublished(ctx, publishedAnswer("ep_a", "alpha", now.Add(2*time.Minute))); err != nil {
		t.Fatalf("StorePublished(no buckets) error = %v", err)
	}
	emptied, err := repo.ListPublishedByEndpointIDs(ctx, []string{"ep_a"})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs() after the empty answer error = %v", err)
	}
	if len(emptied["ep_a"].Windows) != 0 {
		t.Fatalf("the empty answer left %d buckets, want 0", len(emptied["ep_a"].Windows))
	}
	if emptied["ep_a"].State.Message != "fine" {
		t.Fatalf("the empty answer lost the envelope: state = %+v", emptied["ep_a"].State)
	}
}

// TestPublishedQuotaRepository_StoreCostsOneStatementPerTableNotPerBucket pins the
// §1.7 guarantee for the write side: a poll of one bucket and a poll of eight are
// the same number of round trips.
func TestPublishedQuotaRepository_StoreCostsOneStatementPerTableNotPerBucket(t *testing.T) {
	repo, counter := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()
	seedPublishedEndpoint(t, repo, "ep_a", "alpha")

	labels := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		labels = append(labels, "bucket-"+string(rune('a'+i)))
	}

	counter.reset()
	if err := repo.StorePublished(ctx, publishedAnswer("ep_a", "alpha", now, labels[0])); err != nil {
		t.Fatalf("StorePublished(1 bucket) error = %v", err)
	}
	single, singleTrace := counter.load(), counter.trace()

	counter.reset()
	if err := repo.StorePublished(ctx, publishedAnswer("ep_a", "alpha", now, labels...)); err != nil {
		t.Fatalf("StorePublished(8 buckets) error = %v", err)
	}
	batch, batchTrace := counter.load(), counter.trace()

	// pgx traces the transaction's BEGIN and COMMIT alongside its three
	// statements: state upsert, one set-based window upsert, and the prune. The
	// count cannot grow with the bucket count, which is the point of this test,
	// eight buckets cost exactly what one costs.
	const wantStatements = 5
	if single != wantStatements || batch != wantStatements {
		t.Fatalf("statements sent = %d (%s) for one bucket and %d (%s) for eight, want %d each",
			single, singleTrace, batch, batchTrace, wantStatements)
	}

	stored, err := repo.ListPublishedByEndpointIDs(ctx, []string{"ep_a"})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs() error = %v", err)
	}
	if len(stored["ep_a"].Windows) != 8 {
		t.Fatalf("the batched store left %d buckets, want 8", len(stored["ep_a"].Windows))
	}

	// One statement fewer with nothing to write: the batched window upsert is
	// skipped rather than run over an empty set, which is also what makes the five
	// above BEGIN + state upsert + window upsert + prune + COMMIT.
	counter.reset()
	if err := repo.StorePublished(ctx, publishedAnswer("ep_a", "alpha", now)); err != nil {
		t.Fatalf("StorePublished(no buckets) error = %v", err)
	}
	if empty := counter.load(); empty != wantStatements-1 {
		t.Fatalf("an answer with no buckets sent %d statements (%s), want %d",
			empty, counter.trace(), wantStatements-1)
	}
}

// TestPublishedQuotaRepository_StoreRefusesAnUnknownEndpoint proves the foreign
// key the migration declares: an answer for an account that does not exist is a
// not-found the caller can act on, not a row that lingers orphaned.
func TestPublishedQuotaRepository_StoreRefusesAnUnknownEndpoint(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	err := repo.StorePublished(context.Background(), publishedAnswer("ep_ghost", "alpha", testNow()))
	if err == nil {
		t.Fatal("an answer for a missing endpoint was stored, so the foreign key is not enforcing")
	}
	if !errors.Is(err, domain.ErrEndpointNotFound) {
		t.Fatalf("StorePublished(unknown endpoint) error = %v, want domain.ErrEndpointNotFound", err)
	}
}

// TestPublishedQuotaRepository_GoodAnswerClearsAStoredSentence pins the two halves of the
// soft-answer path together. A provider that refuses an account has its sentence stored by
// RecordAttempt, the card's only honest content, and when a later poll brings real
// buckets, that sentence must go with it. A stale "credential invalid" printed above fresh
// numbers would tell the operator the account is broken when the provider has just said it
// is not.
func TestPublishedQuotaRepository_GoodAnswerClearsAStoredSentence(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()
	seedPublishedEndpoint(t, repo, "ep_a", "alpha")

	refused := "Kimi connected, but this account has no permission to view usage."
	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: "ep_a", ProviderID: "alpha",
		AttemptedAt: now, NextAttemptAt: now.Add(2 * time.Minute),
		Message: &refused,
	}); err != nil {
		t.Fatalf("RecordAttempt() error = %v", err)
	}

	stored, err := repo.ListPublishedByEndpointIDs(ctx, []string{"ep_a"})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs() error = %v", err)
	}
	if got := stored["ep_a"].State.Message; got != refused {
		t.Fatalf("stored message = %q, want the provider's own refusal", got)
	}
	if len(stored["ep_a"].Windows) != 0 {
		t.Fatalf("stored windows = %+v, want none: a refusal publishes no buckets", stored["ep_a"].Windows)
	}

	later := now.Add(2 * time.Minute)
	recovered := domain.PublishedAnswer{
		EndpointID: "ep_a", ProviderID: "alpha", Plan: "K2", FetchedAt: later,
		Windows: []domain.PublishedWindowRow{{
			EndpointID: "ep_a", Label: "Weekly", Used: "3", FetchedAt: later,
		}},
	}
	if err := repo.StorePublished(ctx, recovered); err != nil {
		t.Fatalf("StorePublished() error = %v", err)
	}

	after, err := repo.ListPublishedByEndpointIDs(ctx, []string{"ep_a"})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs() after recovery error = %v", err)
	}
	state := after["ep_a"].State
	if state.Message != "" {
		t.Fatalf("message after a good answer = %q, want it cleared, the refusal is history now", state.Message)
	}
	if state.Plan != "K2" {
		t.Fatalf("plan = %q, want the recovered answer's plan", state.Plan)
	}
	if len(after["ep_a"].Windows) != 1 || state.FetchedAt == nil || !state.FetchedAt.Equal(later) {
		t.Fatalf("recovered answer = %+v, want the one bucket and its real fetch instant", after["ep_a"])
	}
}
