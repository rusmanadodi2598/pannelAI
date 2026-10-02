// Tests for the published-quota cache reads.
//
// @file      internal/service/quota_published_cache_test.go
// @for       Proves the collection's provider numbers cost one query, and that a stored answer lowers intact.
// @uses      context, internal/domain, internal/service, testing, time.
// @reason    This screen's standing rule is that reading it never fans out to one
//
//	provider call per account. The rule holds only if the batched read stays
//	batched, so the count of queries is asserted rather than assumed, and the
//	distinction between "no ceiling" and "no answer yet" is asserted with it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// countingPublishedCache is the storage port with a query counter on it. The counter is
// the point of the fake: a change that turned the batched read into a per-endpoint read
// would still render correct cards, and only a count catches it.
type countingPublishedCache struct {
	reads    int
	lastIDs  []string
	byID     map[string]domain.PublishedQuota
	failRead bool
}

func (c *countingPublishedCache) ListPublishedByEndpointIDs(_ context.Context, ids []string) (map[string]domain.PublishedQuota, error) {
	c.reads++
	c.lastIDs = append([]string(nil), ids...)
	if c.failRead {
		return nil, domain.NewInternalError("cache unavailable")
	}
	out := make(map[string]domain.PublishedQuota, len(ids))
	for _, id := range ids {
		if quota, ok := c.byID[id]; ok {
			out[id] = quota
		}
	}
	return out, nil
}

func (c *countingPublishedCache) DueForRefresh(_ context.Context, _ time.Time, _ int) ([]domain.PublishedState, error) {
	return nil, nil
}

func (c *countingPublishedCache) RecordAttempt(_ context.Context, _ domain.PublishedAttempt) error {
	return nil
}

func (c *countingPublishedCache) StorePublished(_ context.Context, _ domain.PublishedAnswer) error {
	return nil
}

// accountsFor names the accounts a page carries. Repetition is deliberate in the
// call sites below: the batched read must dedupe, the way a provider's many windows
// collapse to one account.
func accountsFor(ids ...string) []domain.QuotaAccount {
	out := make([]domain.QuotaAccount, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.QuotaAccount{EndpointID: id, ProviderID: "glm"})
	}
	return out
}

// newPublishedTestService builds the service with only the cache set. The batched read
// touches nothing else, and a test that had to satisfy the whole repository interface to
// reach it would be testing its own fixtures.
func newPublishedTestService(cache repository.PublishedQuotaRepository) *QuotaService {
	return &QuotaService{publishedCache: cache, clock: time.Now}
}

func TestPagePublishedReadsTheWholePageInOneQuery(t *testing.T) {
	cache := &countingPublishedCache{byID: map[string]domain.PublishedQuota{
		"ep_a": {State: domain.PublishedState{ProviderID: "glm"}},
	}}

	svc := newPublishedTestService(cache)
	// Four rows, two endpoints: the page must ask for the distinct ids once, not
	// once per row, or a card of ten windows would cost ten provider reads.
	answers, err := svc.PagePublished(context.Background(), accountsFor("ep_a", "ep_a", "ep_b", "ep_a"))
	if err != nil {
		t.Fatalf("PagePublished() error = %v", err)
	}
	if cache.reads != 1 {
		t.Fatalf("PagePublished() issued %d cache reads, want the one batched read", cache.reads)
	}
	if len(cache.lastIDs) != 2 {
		t.Fatalf("PagePublished() asked for %v, want the two distinct endpoint ids", cache.lastIDs)
	}
	// Every account on the page is answered: one from the cache, one flagged as never
	// polled. Dropping the second is what hid a provider whose keys had not routed yet.
	if len(answers) != 2 || answers[0].EndpointID != "ep_a" || answers[1].EndpointID != "ep_b" {
		t.Fatalf("PagePublished() answered %+v, want one entry per distinct account in page order", answers)
	}
	if !answers[0].Cached || answers[0].NeverPolled {
		t.Fatalf("the cached account lowered as %+v, want cached and not never-polled", answers[0])
	}
	if !answers[1].NeverPolled || answers[1].Cached {
		t.Fatalf("the unanswered account lowered as %+v, want never-polled", answers[1])
	}
}

func TestPagePublishedStaysQuietWithoutACache(t *testing.T) {
	svc := &QuotaService{clock: time.Now}
	answers, err := svc.PagePublished(context.Background(), accountsFor("ep_a"))
	if err != nil {
		t.Fatalf("PagePublished() error = %v", err)
	}
	if answers != nil {
		t.Fatalf("PagePublished() answered %d entries without a cache wired, want nil", len(answers))
	}
}

func TestPagePublishedCarriesTheReadFailureUp(t *testing.T) {
	cache := &countingPublishedCache{failRead: true}
	svc := newPublishedTestService(cache)

	if _, err := svc.PagePublished(context.Background(), accountsFor("ep_a")); err == nil {
		t.Fatal("PagePublished() swallowed a cache read failure; the handler needs it to say provider numbers are missing")
	}
}

func TestPublishedFromCacheKeepsNoCeilingDistinctFromZero(t *testing.T) {
	zero := "0.000000"
	later := time.Unix(1_800_000_000, 0).UTC()
	soon := time.Unix(1_700_000_000, 0).UTC()
	cache := &countingPublishedCache{byID: map[string]domain.PublishedQuota{"ep_a": {
		State: domain.PublishedState{ProviderID: "claude", Plan: "Pro", FetchedAt: &later},
		Windows: []domain.PublishedWindowRow{
			{Label: "weekly (7d)", Used: "50.000000"},
			{Label: "session (5h)", Used: "30.000000", Total: &zero, ResetsAt: &soon},
			{Label: "Balance (USD)", Used: "0.000000", Total: strPtr("12.500000"), Unit: "USD", IsCreditBalance: true},
		},
	}}}

	usage := publishedFromCache("ep_a", cache.byID["ep_a"])

	if len(usage.Windows) != 3 {
		t.Fatalf("lowered %d windows, want 3", len(usage.Windows))
	}
	if usage.Plan != "Pro" || usage.ProviderID != "claude" || !usage.FetchedAt.Equal(later) {
		t.Fatalf("envelope lowered as %+v, want the plan, provider and fetch instant kept", usage)
	}

	// The bucket that closes first leads, and one with no reset goes last: a card that
	// reshuffled between two identical polls would teach the operator nothing.
	if usage.Windows[0].Label != "session (5h)" {
		t.Fatalf("first row = %q, want the bucket with a reset instant to lead", usage.Windows[0].Label)
	}
	if usage.Windows[2].Label != "Balance (USD)" {
		t.Fatalf("last row = %q, want the bucket with no reset to trail", usage.Windows[2].Label)
	}

	var unlimited, spent, credit PublishedWindow
	for _, window := range usage.Windows {
		switch window.Label {
		case "weekly (7d)":
			unlimited = window
		case "session (5h)":
			spent = window
		case "Balance (USD)":
			credit = window
		}
	}
	if unlimited.HasTotal {
		t.Fatal("a stored NULL total lowered as a ceiling; unlimited and exhausted would render the same")
	}
	if !spent.HasTotal || spent.Total != 0 {
		t.Fatalf("a stored zero ceiling lowered as HasTotal=%v Total=%v, want a real ceiling of 0", spent.HasTotal, spent.Total)
	}
	if !credit.IsCreditBalance || credit.Unit != "USD" || credit.Total != 12.5 {
		t.Fatalf("credit balance lowered as %+v, want 12.5 USD flagged as money", credit)
	}
}

// TestPagePublishedAnswersWindowlessAccount is the reason the page enumerates accounts
// rather than counted windows: an endpoint that has served no traffic has no window row,
// and if it is dropped from the answer the operator sees no card for it at all — the
// provider is configured, the worker has polled it, and the screen still shows nothing.
func TestPagePublishedAnswersWindowlessAccount(t *testing.T) {
	cached := time.Unix(1_759_000_000, 0).UTC()
	cache := &countingPublishedCache{byID: map[string]domain.PublishedQuota{
		"ep_answered": {State: domain.PublishedState{ProviderID: "glm", Plan: "Pro", FetchedAt: &cached}},
	}}
	svc := newPublishedTestService(cache)

	answers, err := svc.PagePublished(context.Background(), accountsFor("ep_answered", "ep_windowless"))
	if err != nil {
		t.Fatalf("PagePublished() error = %v", err)
	}
	if cache.reads != 1 {
		t.Fatalf("PagePublished() issued %d reads, want the one batched read", cache.reads)
	}
	if len(answers) != 2 {
		t.Fatalf("PagePublished() answered %d accounts, want one entry per account on the page", len(answers))
	}

	first, second := answers[0], answers[1]
	if first.NeverPolled || !first.Cached || first.EndpointID != "ep_answered" {
		t.Fatalf("answered account = %+v, want the cached answer", first)
	}
	if !second.NeverPolled || second.EndpointID != "ep_windowless" || second.ProviderID != "glm" {
		t.Fatalf("windowless account = %+v, want it named and flagged never-polled", second)
	}
	if len(second.Windows) != 0 || second.Message != "" {
		t.Fatalf("a never-polled account must carry no numbers and no provider sentence: %+v", second)
	}
	if second.Cached {
		t.Fatal("a never-polled account must not claim to be a cached answer")
	}
}
