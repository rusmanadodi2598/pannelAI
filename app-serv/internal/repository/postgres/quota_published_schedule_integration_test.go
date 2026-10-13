//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published_schedule_integration_test.go
// @for       The published-quota scheduling columns against a real server: the attempt stamp the poll worker writes and the due queue it reads.
// @uses      testing, context, time, internal/domain.
// @reason    The sweep is the only thing that decides how often a provider gets queried, so its two failure modes are expensive in opposite directions: a brand-new endpoint that never becomes due leaves the screen blank forever, and a due set read without its ordering key sorts the whole table every tick. Both are properties of the row the database holds, not of the Go code, and neither is reachable without a server (AGENTS.md §1.7).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-02
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestPublishedQuotaRepository_RecordAttemptSchedulesAndCountsFailures covers the
// sweep's bookkeeping: a first attempt creates the row a brand-new endpoint needs
// to be polled at all, the failure run accumulates, and a stored answer resets it
// without touching the schedule the worker owns.
func TestPublishedQuotaRepository_RecordAttemptSchedulesAndCountsFailures(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()
	seedPublishedEndpoint(t, repo, "ep_a", "alpha")

	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: "ep_a", ProviderID: "alpha",
		AttemptedAt: now, NextAttemptAt: now.Add(5 * time.Minute), FailureDelta: 1,
	}); err != nil {
		t.Fatalf("RecordAttempt() error = %v", err)
	}
	due, err := repo.DueForRefresh(ctx, now, 10)
	if err != nil {
		t.Fatalf("DueForRefresh(now) error = %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("an endpoint scheduled 5 minutes out is already due: %+v", due)
	}

	due, err = repo.DueForRefresh(ctx, now.Add(6*time.Minute), 10)
	if err != nil {
		t.Fatalf("DueForRefresh(later) error = %v", err)
	}
	if len(due) != 1 || due[0].ProviderID != "alpha" || due[0].ConsecutiveFailures != 1 {
		t.Fatalf("DueForRefresh() = %+v, want ep_a with its provider id and a failure run of 1", due)
	}
	if due[0].LastAttemptAt == nil || !due[0].LastAttemptAt.Equal(now) {
		t.Fatalf("LastAttemptAt = %v, want the attempt instant", due[0].LastAttemptAt)
	}
	if due[0].FetchedAt != nil {
		t.Fatalf("FetchedAt = %v, want nil for an endpoint that has never answered", *due[0].FetchedAt)
	}

	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: "ep_a", ProviderID: "alpha",
		AttemptedAt: now.Add(6 * time.Minute), NextAttemptAt: now.Add(16 * time.Minute), FailureDelta: 1,
	}); err != nil {
		t.Fatalf("RecordAttempt() again error = %v", err)
	}
	due, err = repo.DueForRefresh(ctx, now.Add(17*time.Minute), 10)
	if err != nil {
		t.Fatalf("DueForRefresh() error = %v", err)
	}
	if len(due) != 1 || due[0].ConsecutiveFailures != 2 {
		t.Fatalf("failure run after a second failed poll = %+v, want 2", due)
	}

	// A good answer clears the run and leaves the next interval alone: rescheduling
	// is the worker's decision, made through RecordAttempt.
	nextBefore := due[0].NextAttemptAt
	if err := repo.StorePublished(ctx, publishedAnswer("ep_a", "alpha", now.Add(18*time.Minute), "Weekly")); err != nil {
		t.Fatalf("StorePublished() error = %v", err)
	}
	stored, err := repo.DueForRefresh(ctx, now.Add(24*time.Hour), 10)
	if err != nil {
		t.Fatalf("DueForRefresh() error = %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("DueForRefresh() = %+v, want the endpoint back after a success", stored)
	}
	if stored[0].ConsecutiveFailures != 0 {
		t.Fatalf("failure run after a stored answer = %d, want 0", stored[0].ConsecutiveFailures)
	}
	if !stored[0].NextAttemptAt.Equal(nextBefore) {
		t.Fatalf("StorePublished moved the schedule from %v to %v, which is the worker's column",
			nextBefore, stored[0].NextAttemptAt)
	}
	if stored[0].Plan != "pro" || stored[0].FetchedAt == nil {
		t.Fatalf("state after the store = %+v, want the answer's plan and fetched_at", stored[0])
	}

	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: "ep_a", ProviderID: "alpha",
		AttemptedAt: now, NextAttemptAt: now.Add(time.Minute), FailureDelta: -1,
	}); err == nil {
		t.Fatal("a negative failure delta was accepted, which would shorten every later backoff")
	}
}

// TestPublishedQuotaRepository_DueForRefreshIsBoundedAndOldestFirst proves the
// sweep takes the longest-waiting endpoints first and never reads more than it was
// asked to.
func TestPublishedQuotaRepository_DueForRefreshIsBoundedAndOldestFirst(t *testing.T) {
	repo, counter := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()

	// Deliberately inserted out of due order, so the assertion below cannot pass on
	// insertion order: the oldest due date has to win.
	seeds := []struct {
		id       string
		offsetMS int
	}{
		{id: "ep_c", offsetMS: 3},
		{id: "ep_a", offsetMS: 1},
		{id: "ep_b", offsetMS: 2},
	}
	for _, seed := range seeds {
		seedPublishedEndpoint(t, repo, seed.id, "alpha")
		if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
			EndpointID: seed.id, ProviderID: "alpha",
			AttemptedAt: now, NextAttemptAt: now.Add(time.Duration(seed.offsetMS) * time.Minute), FailureDelta: 0,
		}); err != nil {
			t.Fatalf("RecordAttempt(%s) error = %v", seed.id, err)
		}
	}

	counter.reset()
	queue, err := repo.DueForRefresh(ctx, now.Add(5*time.Minute), 2)
	if err != nil {
		t.Fatalf("DueForRefresh() error = %v", err)
	}
	if counter.load() != 1 {
		t.Fatalf("the sweep sent %d statements (%s), want 1", counter.load(), counter.trace())
	}
	if len(queue) != 2 {
		t.Fatalf("DueForRefresh(limit 2) returned %d, want 2", len(queue))
	}
	if queue[0].EndpointID != "ep_a" || queue[1].EndpointID != "ep_b" {
		t.Fatalf("queue = %s,%s, want the two oldest-due endpoints", queue[0].EndpointID, queue[1].EndpointID)
	}

	// A limit below one asks for nothing, and a sweep tick that asks for nothing
	// must not reach the server.
	counter.reset()
	if none, err := repo.DueForRefresh(ctx, now, 0); err != nil || len(none) != 0 {
		t.Fatalf("DueForRefresh(limit 0) = %d rows/%v, want no rows and no error", len(none), err)
	}
	if counter.load() != 0 {
		t.Fatalf("limit 0 sent %d statements, want 0", counter.load())
	}
}
