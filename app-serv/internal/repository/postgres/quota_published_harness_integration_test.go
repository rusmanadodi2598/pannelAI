//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published_harness_integration_test.go
// @for       The shared harness the published-quota integration tests run on: a
//
//	statement-counting pool, an emptied cache, and one seeded account.
//
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgxpool, testing,
//
//	context, strings, sync, time, internal/domain, internal/migrations.
//
// @reason    "The screen reads the cache in one query" is the whole reason this
//
//	table exists, and a stub cannot prove it: a stub counts nothing, and only a
//	real server shows the difference between a batched `= ANY($1)` read and a
//	per-id loop. So the harness is newTestPool plus a pgx QueryTracer — the same
//	DSN guard and migration path every other harness in this package uses
//	(usage_harness_integration_test.go being the precedent for a harness of its
//	own), extended only by the counter.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-02
package postgres

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// statementCounter records the statements pgx sends. pgx routes Query, QueryRow,
// Exec and a transaction's BEGIN/COMMIT through the tracer, so what this counts is
// the real round-trip count — which is exactly what these tests assert, and the
// reason a per-bucket statement could not hide in it. The leading keyword of each
// call is kept so a failed count reports which statements came back, not just how
// many.
type statementCounter struct {
	mu   sync.Mutex
	sqls []string
}

func (c *statementCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sqls = append(c.sqls, firstKeyword(data.SQL))
	return ctx
}

func (c *statementCounter) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {}

func (c *statementCounter) load() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return int64(len(c.sqls))
}

// trace renders the recorded statements in order, e.g. "BEGIN INSERT INSERT DELETE COMMIT".
func (c *statementCounter) trace() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.sqls, " ")
}

func (c *statementCounter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sqls = nil
}

// firstKeyword takes the leading keyword of a statement, which is all a test
// failure needs to show: the rest of a 400-character SQL string only buries it.
func firstKeyword(query string) string {
	fields := strings.Fields(strings.TrimSpace(query))
	if len(fields) == 0 {
		return "(empty)"
	}
	return strings.ToUpper(fields[0])
}

// newPublishedRepo returns a published-quota repository over an emptied cache,
// plus the counter that sees every statement it sends.
//
// upstream_endpoints is truncated with CASCADE, which takes both cache tables with
// it through the 000013 foreign keys — so a run cannot inherit another run's rows,
// and the cascade itself gets exercised on the way.
func newPublishedRepo(t *testing.T) (*PublishedQuotaRepository, *statementCounter) {
	t.Helper()

	dsn := requireTestDSN(t)
	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing the DSN: %v", err)
	}
	counter := &statementCounter{}
	cfg.ConnConfig.Tracer = counter

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE upstream_endpoints CASCADE`); err != nil {
		t.Fatalf("truncating upstream_endpoints: %v", err)
	}
	return NewPublishedQuotaRepository(pool), counter
}

// seedPublishedEndpoint creates the account a cache row belongs to, because the
// foreign key 000013 declares means an answer cannot be stored for an endpoint
// that does not exist.
func seedPublishedEndpoint(t *testing.T, repo *PublishedQuotaRepository, id, providerID string) {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, "seed "+id, domain.UpstreamAuthAPIKey, 1, time.Now())
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint(%s) error = %v", id, err)
	}
	if err := NewEndpointRepository(repo.pool).Create(context.Background(), endpoint); err != nil {
		t.Fatalf("seeding endpoint %s: %v", id, err)
	}
}

// publishedAnswer builds one endpoint's answer with the given labels, each bucket
// reporting used 1 against a ceiling of 1.
func publishedAnswer(endpointID, providerID string, fetchedAt time.Time, labels ...string) domain.PublishedAnswer {
	windows := make([]domain.PublishedWindowRow, 0, len(labels))
	one := "1"
	for _, label := range labels {
		windows = append(windows, domain.PublishedWindowRow{
			EndpointID: endpointID, Label: label, Used: one, Total: &one, FetchedAt: fetchedAt,
		})
	}
	return domain.PublishedAnswer{
		EndpointID: endpointID, ProviderID: providerID, Plan: "pro", Message: "fine",
		FetchedAt: fetchedAt, Windows: windows,
	}
}

// testNow returns one instant for a whole test, truncated to the microsecond
// PostgreSQL's timestamptz keeps, so a round-tripped value compares equal.
func testNow() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
