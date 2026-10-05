//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published_integration_test.go
// @for       The published-quota read path against a real server: the batched
//
//	query the quota screen runs, and the NULL ceiling it must not flatten.
//
// @uses      testing, context, time, internal/domain.
// @reason    Two guarantees of this read are invisible to a stub. That N
//
//	endpoints cost one statement is only measurable against a server that can
//	count them, and that a NULL ceiling survives as "no ceiling" is a property
//	of the numeric column and the LEFT JOIN, not of the Go struct, the
//	joining NULLs are exactly what makes an endpoint with no buckets decodable
//	instead of an error (AGENTS.md §1.7, §2.1).
//
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

// TestPublishedQuotaRepository_ListIsOneQueryForTheWholeBatch pins the property
// the screen's read path is built on: N endpoints cost one statement, not N, and
// an empty page costs nothing at all (AGENTS.md §1.7).
func TestPublishedQuotaRepository_ListIsOneQueryForTheWholeBatch(t *testing.T) {
	repo, counter := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()

	ids := []string{"ep_a", "ep_b", "ep_c"}
	for i, id := range ids {
		providerID := "provider_" + string(rune('a'+i))
		seedPublishedEndpoint(t, repo, id, providerID)
		if err := repo.StorePublished(ctx, publishedAnswer(id, providerID, now, "Weekly", "Daily")); err != nil {
			t.Fatalf("StorePublished(%s) error = %v", id, err)
		}
	}

	counter.reset()
	answers, err := repo.ListPublishedByEndpointIDs(ctx, ids)
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs() error = %v", err)
	}
	if got := counter.load(); got != 1 {
		t.Fatalf("the batched read sent %d statements (%s) for %d endpoints, want exactly 1",
			got, counter.trace(), len(ids))
	}
	if len(answers) != len(ids) {
		t.Fatalf("ListPublishedByEndpointIDs() returned %d endpoints, want %d", len(answers), len(ids))
	}
	for _, id := range ids {
		if len(answers[id].Windows) != 2 {
			t.Fatalf("%s has %d buckets, want 2", id, len(answers[id].Windows))
		}
		if answers[id].State.Plan != "pro" || answers[id].State.FetchedAt == nil {
			t.Fatalf("%s state = %+v, want the plan and fetched_at the answer carried", id, answers[id].State)
		}
	}

	// A page with no ids must not reach the server: a scan over an empty array is
	// still a round trip, and one per poll tick is a query for nothing.
	counter.reset()
	empty, err := repo.ListPublishedByEndpointIDs(ctx, nil)
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs(nil) error = %v", err)
	}
	if got := counter.load(); got != 0 {
		t.Fatalf("an empty id list sent %d statements, want 0", got)
	}
	if len(empty) != 0 {
		t.Fatalf("ListPublishedByEndpointIDs(nil) returned %d endpoints, want 0", len(empty))
	}

	// An endpoint the cache has never answered for is absent, not present-and-empty.
	missing, err := repo.ListPublishedByEndpointIDs(ctx, []string{"ep_never"})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs(ep_never) error = %v", err)
	}
	if _, ok := missing["ep_never"]; ok {
		t.Fatal("an endpoint with no cached row was returned as if it had one")
	}
}

// TestPublishedQuotaRepository_NullTotalSurvivesAsNoCeiling drives the one column
// a reader will be tempted to simplify: NULL means the provider stated no ceiling,
// 0 means a ceiling that is spent, and the cache must not merge them.
func TestPublishedQuotaRepository_NullTotalSurvivesAsNoCeiling(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()
	zero, hundred := "0", "100"

	seedPublishedEndpoint(t, repo, "ep_a", "alpha")
	answer := domain.PublishedAnswer{
		EndpointID: "ep_a", ProviderID: "alpha", Plan: "free", FetchedAt: now,
		Windows: []domain.PublishedWindowRow{
			{EndpointID: "ep_a", Label: "unlimited bucket", Used: "7", Total: nil, Unlimited: true, FetchedAt: now},
			{EndpointID: "ep_a", Label: "spent bucket", Used: "500", Total: &zero, FetchedAt: now},
			{EndpointID: "ep_a", Label: "capped bucket", Used: "500", Total: &hundred, FetchedAt: now},
		},
	}
	if err := repo.StorePublished(ctx, answer); err != nil {
		t.Fatalf("StorePublished() error = %v", err)
	}

	stored, err := repo.ListPublishedByEndpointIDs(ctx, []string{"ep_a"})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs() error = %v", err)
	}
	windows := stored["ep_a"].Windows
	if len(windows) != 3 {
		t.Fatalf("read back %d buckets, want 3", len(windows))
	}
	byLabel := make(map[string]domain.PublishedWindowRow, len(windows))
	for _, window := range windows {
		byLabel[window.Label] = window
	}

	unlimited := byLabel["unlimited bucket"]
	if value, ok := unlimited.Ceiling(); ok {
		t.Fatalf("the uncapped bucket read back a ceiling %q, want none: NULL and 0 are different facts", value)
	}
	if unlimited.Total != nil {
		t.Fatalf("the uncapped bucket decoded Total to %q, want a nil pointer", *unlimited.Total)
	}
	if unlimited.Used != "7.000000" {
		t.Fatalf("the uncapped bucket Used = %q, want the column's decimal text", unlimited.Used)
	}
	if !unlimited.Unlimited {
		t.Fatal("the uncapped bucket lost the unlimited flag the provider set")
	}

	spent := byLabel["spent bucket"]
	if value, ok := spent.Ceiling(); !ok {
		t.Fatal("the spent bucket lost its ceiling: a stored 0 must not read back as unlimited")
	} else if value != "0.000000" {
		t.Fatalf("the spent bucket ceiling = %q, want zero rendered at the column scale", value)
	}
	if _, ok := byLabel["capped bucket"].Ceiling(); !ok {
		t.Fatal("the capped bucket lost its ceiling")
	}
}

// TestPublishedQuotaRepository_AnswerWithNoBucketsStillReads proves the LEFT JOIN
// direction: an endpoint whose provider answered with buckets removed still has a
// plan and a fetched_at to show, and decoding it must not fail on the join's NULLs.
func TestPublishedQuotaRepository_AnswerWithNoBucketsStillReads(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()

	seedPublishedEndpoint(t, repo, "ep_a", "alpha")
	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: "ep_a", ProviderID: "alpha",
		AttemptedAt: now, NextAttemptAt: now.Add(time.Minute), FailureDelta: 0,
	}); err != nil {
		t.Fatalf("RecordAttempt() error = %v", err)
	}

	stored, err := repo.ListPublishedByEndpointIDs(ctx, []string{"ep_a"})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs() error = %v", err)
	}
	quota, ok := stored["ep_a"]
	if !ok {
		t.Fatal("an endpoint with a state row and no buckets was left out of the read")
	}
	if len(quota.Windows) != 0 {
		t.Fatalf("read %d buckets for an endpoint that has none, want 0", len(quota.Windows))
	}
	if quota.State.FetchedAt != nil {
		t.Fatalf("FetchedAt = %v, want nil until an answer is stored", *quota.State.FetchedAt)
	}
}
