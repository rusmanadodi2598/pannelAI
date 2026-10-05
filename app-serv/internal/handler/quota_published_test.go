// Tests for the quota collection's published block at the route.
//
// @file      internal/handler/quota_published_test.go
// @for       Proves GET /api/v1/quotas always answers published as an array, and names the gap when the cache cannot be read.
// @uses      context, encoding/json, internal/domain, internal/repository, internal/service, net/http/httptest, testing, time.
// @reason    The provider number now leads the card, so the array's shape is what the panel
//
//	parses strictly. An empty page has to read as "nothing answered yet", and a
//	cache outage has to read as that too, a bare null would let a database fault
//	look like accounts whose providers published nothing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-02
package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// publishedCacheStub is the screen's read seam with a switch for the failing case, so
// both branches of the degraded path are exercised without a database.
type publishedCacheStub struct {
	answers map[string]domain.PublishedQuota
	fail    bool

	// reads and lastIDs are the point of the stub: the route's no-N+1 rule is a claim
	// about how many statements one page costs, and only a count can fail it.
	reads   int
	lastIDs []string
}

func (c *publishedCacheStub) ListPublishedByEndpointIDs(ctx context.Context, ids []string) (map[string]domain.PublishedQuota, error) {
	c.reads++
	c.lastIDs = append([]string(nil), ids...)
	if c.fail {
		return nil, domain.NewInternalError("cache unreachable")
	}
	if len(ids) == 0 {
		return map[string]domain.PublishedQuota{}, nil
	}
	out := make(map[string]domain.PublishedQuota, len(ids))
	for _, id := range ids {
		if answer, ok := c.answers[id]; ok {
			_ = ctx
			out[id] = answer
		}
	}
	return out, nil
}

func (c *publishedCacheStub) DueForRefresh(_ context.Context, _ time.Time, _ int) ([]domain.PublishedState, error) {
	return nil, nil
}

func (c *publishedCacheStub) RecordAttempt(_ context.Context, _ domain.PublishedAttempt) error {
	return nil
}

func (c *publishedCacheStub) StorePublished(_ context.Context, _ domain.PublishedAnswer) error {
	return nil
}

var _ repository.PublishedQuotaRepository = (*publishedCacheStub)(nil)

func newPublishedQuotaFixture(t *testing.T, cache *publishedCacheStub) *QuotaHandler {
	t.Helper()

	// Three accounts share the page's provider group: two this gateway has counted, and
	// one that has never routed anything. The windowless one is the case the account page
	// exists for, and the three together are what make "one read per page" a claim with
	// content rather than a tautology about a single id.
	repo := newStubQuotaRepo()
	counted := []string{"ep_g12", "ep_g13"}
	for _, id := range counted {
		window, err := domain.NewQuotaWindow(id, "glm", domain.QuotaWindowFiveHour, nil, nil, time.Now())
		if err != nil {
			t.Fatalf("seeding a counted window for %s: %v", id, err)
		}
		repo.windows = append(repo.windows, window)
	}
	repo.accounts = []domain.QuotaAccount{
		{EndpointID: "ep_g12", ProviderID: "glm"},
		{EndpointID: "ep_g13", ProviderID: "glm"},
		{EndpointID: "ep_g14", ProviderID: "glm"},
	}

	quotas, err := service.NewQuotaService(service.QuotaServiceDeps{
		Quotas: repo, Endpoints: &stubEndpointRepo{known: map[string]bool{
			"ep_g12": true, "ep_g13": true, "ep_g14": true,
		}},
		PublishedCache: cache,
	})
	if err != nil {
		t.Fatalf("NewQuotaService() error = %v", err)
	}
	return NewQuotaHandler(quotas)
}

func TestQuotaHandler_ListPublishedBlock(t *testing.T) {
	cases := []struct {
		name      string
		cache     *publishedCacheStub
		wantSub   []string
		forbidSub []string
	}{
		{
			name:      "an account the worker has not answered still gets an entry",
			cache:     &publishedCacheStub{answers: map[string]domain.PublishedQuota{}},
			wantSub:   []string{`"published":[{`, `"never_polled":true`, `"endpoint_id":"ep_g12"`},
			forbidSub: []string{`"published":null`, `"published_note"`, `"cached":true`},
		},
		{
			name: "a cached answer is carried beside the counted windows",
			cache: &publishedCacheStub{answers: map[string]domain.PublishedQuota{
				"ep_g12": {
					State: domain.PublishedState{
						EndpointID: "ep_g12", ProviderID: "glm", Plan: "Pro",
						FetchedAt: ptrTime(time.Unix(1_759_000_000, 0).UTC()),
					},
					Windows: []domain.PublishedWindowRow{
						{Label: "Session (5h)", Used: "7", Unlimited: true, Unit: "%"},
					},
				},
			}},
			wantSub:   []string{`"published":[{`, `"provider_id":"glm"`, `"plan":"Pro"`, `"unlimited":true`, `"cached":true`},
			forbidSub: []string{`"published_note"`, `"failures"`, `"last_attempt_at"`},
		},
		{
			// The reference lets some families fail loudly; under a cached read the
			// honest equivalent is the surviving figures plus the fact that the last
			// attempt did not produce them. Without both instants the card cannot tell
			// those two states apart.
			name: "a failing poll keeps its figures and says it failed",
			cache: &publishedCacheStub{answers: map[string]domain.PublishedQuota{
				"ep_g12": {
					State: domain.PublishedState{
						EndpointID: "ep_g12", ProviderID: "glm",
						FetchedAt:           ptrTime(time.Unix(1_759_000_000, 0).UTC()),
						LastAttemptAt:       ptrTime(time.Unix(1_759_000_600, 0).UTC()),
						ConsecutiveFailures: 3,
					},
					Windows: []domain.PublishedWindowRow{{Label: "Weekly", Used: "4", Total: ptrString("10")}},
				},
			}},
			wantSub: []string{
				`"failures":3`, `"last_attempt_at":"2025-09-27T19:16:40Z"`,
				`"fetched_at":"2025-09-27T19:06:40Z"`, `"cached":true`,
			},
			forbidSub: []string{`"published_note"`},
		},
		{
			name:      "a cache that cannot be read says so and keeps the counts",
			cache:     &publishedCacheStub{fail: true},
			wantSub:   []string{`"published_note":`},
			forbidSub: []string{},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			handler := newPublishedQuotaFixture(t, testCase.cache)
			rec := httptest.NewRecorder()
			handler.List(rec, httptest.NewRequest("GET", "/api/v1/quotas", nil))

			if rec.Code != 200 {
				t.Fatalf("status = %d, want 200 even when the provider cache failed: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			for _, want := range testCase.wantSub {
				if !strings.Contains(body, want) {
					t.Fatalf("body = %s, want it to contain %q", body, want)
				}
			}
			for _, forbidden := range testCase.forbidSub {
				if strings.Contains(body, forbidden) {
					t.Fatalf("body = %s, must not contain %q", body, forbidden)
				}
			}
		})
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

func ptrString(value string) *string { return &value }

// TestQuotaHandler_ListProviderReadIsOneQueryPerRoute pins the rule the whole design rests
// on at the place the operator actually hits: one page of the collection read costs ONE
// cache query, whatever accounts it carries. A future change that moved the batched read
// into a per-endpoint loop would still render correct cards, and only this count notices.
func TestQuotaHandler_ListProviderReadIsOneQueryPerRoute(t *testing.T) {
	cache := &publishedCacheStub{answers: map[string]domain.PublishedQuota{
		"ep_g12": {State: domain.PublishedState{ProviderID: "glm", Plan: "Pro"}},
	}}
	handler := newPublishedQuotaFixture(t, cache)

	// Three accounts share the page's provider group: two counted, one never routed.
	rec := httptest.NewRecorder()
	handler.List(rec, httptest.NewRequest("GET", "/api/v1/quotas", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if cache.reads != 1 {
		t.Fatalf("the route issued %d provider-cache reads for one page, want exactly 1", cache.reads)
	}
	if len(cache.lastIDs) != 3 {
		t.Fatalf("the batched read asked for %v, want the page's accounts in one call", cache.lastIDs)
	}

	body := rec.Body.String()
	for _, account := range cache.lastIDs {
		if !strings.Contains(body, `"`+account+`"`) {
			t.Fatalf("body = %s, want every account of the page listed (%s missing)", body, account)
		}
	}
}
