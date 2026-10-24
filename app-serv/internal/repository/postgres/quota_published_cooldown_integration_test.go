//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published_cooldown_integration_test.go
// @for       The endpoint's quota cooldown against a real server: the write the data plane makes, the sweep that honours it, and the deadline that only extends.
// @uses      testing, context, time, internal/domain.
// @reason    Draft 050 T1 found the poller filtering on a column no path wrote, and the fix is a second column with different rules from the serving one. Which column a sweep reads, whether a concurrent write can shorten a wait, and whether a served call releases an account early are properties of the row PostgreSQL holds; the in-memory doubles cannot tell a GREATEST from an assignment (AGENTS.md §1.7, §2.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-13
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestPublishedQuotaRepository_DueForRefreshSkipsAThrottledEndpoint proves the whole chain draft
// 050 T1 found missing: a vendor 429 on the data plane's own write path lands in the endpoint's
// quota cooldown, the poll sweep honours it before the LIMIT rather than after it, and the
// deadline releases the account by expiring. A throttled endpoint that still occupied an
// oldest-due slot would spend budget on every tick and starve the accounts behind it.
func TestPublishedQuotaRepository_DueForRefreshSkipsAThrottledEndpoint(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()
	endpoints := NewEndpointRepository(repo.pool)

	throttled := "ep_hot"
	calm := "ep_calm"
	for _, id := range []string{throttled, calm} {
		seedPublishedEndpoint(t, repo, id, "alpha")
		if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
			EndpointID: id, ProviderID: "alpha", AttemptedAt: now, NextAttemptAt: now, FailureDelta: 0,
		}); err != nil {
			t.Fatalf("RecordAttempt(%s) error = %v", id, err)
		}
	}

	// throttle writes the cooldown the way the data plane does: the aggregate carries
	// the deadline and RecordUpstreamOutcome persists it with the rest of the parity.
	throttle := func(id string, at time.Time) {
		t.Helper()
		endpoint, err := domain.NewUpstreamEndpoint(id, "alpha", "seed "+id, domain.UpstreamAuthAPIKey, 1, at)
		if err != nil {
			t.Fatalf("NewUpstreamEndpoint(%s) error = %v", id, err)
		}
		endpoint.MarkQuotaThrottled(at)
		if err := endpoints.RecordUpstreamOutcome(ctx, endpoint); err != nil {
			t.Fatalf("RecordUpstreamOutcome(%s) error = %v", id, err)
		}
	}

	// The deadline is judged against the instant the sweep is asking about: an endpoint still
	// throttled at that moment is not polled, and budget is not spent on it.
	throttle(throttled, now)
	due, err := repo.DueForRefresh(ctx, now.Add(time.Minute), 5)
	if err != nil {
		t.Fatalf("DueForRefresh() error = %v", err)
	}
	if len(due) != 1 || due[0].EndpointID != calm {
		ids := make([]string, 0, len(due))
		for _, state := range due {
			ids = append(ids, state.EndpointID)
		}
		t.Fatalf("queue = %v, want only the endpoint that is not cooling down", ids)
	}

	// The write is a deadline, not a flag: asking past it makes the account pollable
	// again with nobody editing a row and no sweeper clearing it.
	due, err = repo.DueForRefresh(ctx, now.Add(domain.QuotaCooldownWindow+time.Minute), 5)
	if err != nil {
		t.Fatalf("DueForRefresh() after the cooldown error = %v", err)
	}
	if len(due) != 2 {
		t.Fatalf("queue = %d endpoints, want both once the cooldown has passed", len(due))
	}

	// The cooldown reads back on the endpoint's own load, and it is the only thing that
	// moved: a vendor throttle for a usage read must not have touched the serving columns.
	reloaded, err := endpoints.GetByID(ctx, throttled)
	if err != nil {
		t.Fatalf("GetByID(%s) error = %v", throttled, err)
	}
	if until := reloaded.QuotaCooldownUntil(); until == nil {
		t.Fatal("the reloaded endpoint lost its quota cooldown")
	} else if !until.After(now) {
		t.Fatalf("cooldown = %v, want the newest 429 to have extended it past now", until)
	}
	if reloaded.RateLimitedUntil() != nil {
		t.Fatalf("rate_limited_until = %v, want the serving column left alone by a quota cooldown", reloaded.RateLimitedUntil())
	}
	// The account's serving state is what it was: the cooldown is a separate question,
	// and the domain owns the invariant that Available() does not read it.
	if reloaded.Status() != domain.UpstreamEndpointActive {
		t.Fatalf("endpoint status = %q, want a quota cooldown to leave an account active", reloaded.Status())
	}
}

// TestPublishedQuotaRepository_DueForRefreshKeepsALaterCooldown pins the one race
// the write has to survive: two outcome writes for one endpoint, an earlier deadline
// landing last. GREATEST is what keeps the longer wait standing, the same rule
// 050 T7 established for a key's park.
func TestPublishedQuotaRepository_DueForRefreshKeepsALaterCooldown(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()
	endpoints := NewEndpointRepository(repo.pool)

	const id = "ep_race"
	seedPublishedEndpoint(t, repo, id, "alpha")
	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: id, ProviderID: "alpha", AttemptedAt: now, NextAttemptAt: now, FailureDelta: 0,
	}); err != nil {
		t.Fatalf("RecordAttempt error = %v", err)
	}

	write := func(at time.Time) {
		t.Helper()
		endpoint, err := domain.NewUpstreamEndpoint(id, "alpha", "seed "+id, domain.UpstreamAuthAPIKey, 1, at)
		if err != nil {
			t.Fatalf("NewUpstreamEndpoint error = %v", err)
		}
		endpoint.MarkQuotaThrottled(at)
		if err := endpoints.RecordUpstreamOutcome(ctx, endpoint); err != nil {
			t.Fatalf("RecordUpstreamOutcome error = %v", err)
		}
	}

	write(now.Add(2 * time.Minute))
	write(now)

	reloaded, err := endpoints.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	want := now.Add(domain.QuotaCooldownWindow + 2*time.Minute)
	if got := reloaded.QuotaCooldownUntil(); got == nil || !got.Equal(want) {
		t.Fatalf("cooldown = %v, want the later deadline %v to survive the earlier write", got, want)
	}
}

// TestPublishedQuotaRepository_OutcomeWithout429LeavesCooldownAlone is the other
// half: an outcome that says nothing about throttling must not shorten or clear a
// wait the vendor already showed, because the poller's budget is the whole point.
func TestPublishedQuotaRepository_OutcomeWithout429LeavesCooldownAlone(t *testing.T) {
	repo, _ := newPublishedRepo(t)
	ctx := context.Background()
	now := testNow()
	endpoints := NewEndpointRepository(repo.pool)

	const id = "ep_quiet"
	seedPublishedEndpoint(t, repo, id, "alpha")

	throttled, err := domain.NewUpstreamEndpoint(id, "alpha", "seed "+id, domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint error = %v", err)
	}
	throttled.MarkQuotaThrottled(now)
	if err := endpoints.RecordUpstreamOutcome(ctx, throttled); err != nil {
		t.Fatalf("RecordUpstreamOutcome(429) error = %v", err)
	}
	want := *throttled.QuotaCooldownUntil()

	// A served call carries no cooldown: the aggregate was loaded before the
	// deadline was set, and the write must keep what the row already holds.
	served, err := domain.NewUpstreamEndpoint(id, "alpha", "seed "+id, domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint error = %v", err)
	}
	served.RecordUpstreamSuccess(now)
	if err := endpoints.RecordUpstreamOutcome(ctx, served); err != nil {
		t.Fatalf("RecordUpstreamOutcome(success) error = %v", err)
	}

	reloaded, err := endpoints.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	until := reloaded.QuotaCooldownUntil()
	if until == nil || !until.Equal(want) {
		t.Fatalf("cooldown = %v, want a served call to leave the %v deadline standing", until, want)
	}
	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: id, ProviderID: "alpha", AttemptedAt: now, NextAttemptAt: now, FailureDelta: 0,
	}); err != nil {
		t.Fatalf("RecordAttempt error = %v", err)
	}
	due, err := repo.DueForRefresh(ctx, now, 5)
	if err != nil {
		t.Fatalf("DueForRefresh() error = %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("queue = %d endpoints, want the account still cooling down to be skipped", len(due))
	}
}
