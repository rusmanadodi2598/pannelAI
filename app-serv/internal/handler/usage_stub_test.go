// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage_stub_test.go
// @for       The in-memory usage and log repositories behind the §7.12 route tests.
// @uses      context, sync, testing, time, internal/domain, internal/repository.
// @reason    Draft 010 F1: the four Usage routes had service tests and a router
//
//	session sweep but no HTTP test that drives a handler method with
//	a real request, so a mapper regression could only surface in the
//	panel. The stubs and the route tables are separate declarations,
//	which keeps each file inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// errUsageRepo stands in for a driver failure, so a route maps it to
// INTERNAL_ERROR rather than leaking text.
var errUsageRepo = errors.New("stub: usage repository unavailable")

// stubUsageRepo is an in-memory UsageRecordRepository over one seeded record,
// recording the last read so a route test can assert what reached storage.
type stubUsageRepo struct {
	mu        sync.Mutex
	record    domain.UsageRecord
	lastSum   domain.UsageFilter
	lastGrp   domain.UsageGroupBy
	lastQ     repository.PageQuery
	sumErr    error
	seriesErr error
	listErr   error
	getErr    error
}

func (r *stubUsageRepo) Record(context.Context, domain.UsageRecord) error { return nil }

func (r *stubUsageRepo) Summary(_ context.Context, filter domain.UsageFilter, groupBy domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastSum = filter
	r.lastGrp = groupBy
	if r.sumErr != nil {
		return domain.UsageTotals{}, nil, r.sumErr
	}
	return r.record.Totals(), []domain.UsageGroupRow{{Key: r.record.ProviderID(), Totals: r.record.Totals()}}, nil
}

func (r *stubUsageRepo) Timeseries(context.Context, domain.UsageFilter, domain.UsageGranularity) ([]domain.RateBucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seriesErr != nil {
		return nil, r.seriesErr
	}
	return []domain.RateBucket{{Bucket: r.record.TS(), Totals: r.record.Totals()}}, nil
}

func (r *stubUsageRepo) List(_ context.Context, filter domain.UsageFilter, q repository.PageQuery) ([]domain.UsageRecord, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastSum = filter
	r.lastQ = q
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	return []domain.UsageRecord{r.record}, 1, nil
}

func (r *stubUsageRepo) GetByRequestID(_ context.Context, requestID string) (domain.UsageRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return domain.UsageRecord{}, r.getErr
	}
	if requestID != r.record.RequestID() {
		return domain.UsageRecord{}, domain.ErrUsageRecordNotFound
	}
	return r.record, nil
}

func (r *stubUsageRepo) MonthlyUsage(context.Context, string, time.Time) (domain.UsageTotals, error) {
	return domain.UsageTotals{}, nil
}

// stubUsageLogs is an in-memory RequestLogRepository answering one entry, so
// the detail route's captured-log join is exercised without a database.
type stubUsageLogs struct {
	entry domain.RequestLog
}

func (l *stubUsageLogs) Insert(context.Context, domain.RequestLog) error { return nil }
func (l *stubUsageLogs) List(context.Context, domain.LogFilter, repository.PageQuery) ([]domain.RequestLog, int64, error) {
	return nil, 0, nil
}
func (l *stubUsageLogs) GetByRequestID(_ context.Context, requestID string) (domain.RequestLog, error) {
	if requestID != l.entry.RequestID() {
		return domain.RequestLog{}, domain.ErrRequestLogNotFound
	}
	return l.entry, nil
}
func (l *stubUsageLogs) DeleteOlderThan(context.Context, time.Time) (int64, error) { return 0, nil }

// missingUsageLogs is a RequestLogRepository whose every read misses, which is
// the state the detail route must answer with capture on and no log block.
type missingUsageLogs struct{}

func (missingUsageLogs) Insert(context.Context, domain.RequestLog) error { return nil }
func (missingUsageLogs) List(context.Context, domain.LogFilter, repository.PageQuery) ([]domain.RequestLog, int64, error) {
	return nil, 0, nil
}
func (missingUsageLogs) GetByRequestID(context.Context, string) (domain.RequestLog, error) {
	return domain.RequestLog{}, domain.ErrRequestLogNotFound
}
func (missingUsageLogs) DeleteOlderThan(context.Context, time.Time) (int64, error) { return 0, nil }

// newUsageHandlerFixture wires the real handler over the in-memory repositories
// through the real service, so the request-to-response path a route test drives
// is the production one. A nil logs double means "no log repository wired".
func newUsageHandlerFixture(t *testing.T, logs repository.RequestLogRepository) (*UsageHandler, *stubUsageRepo) {
	t.Helper()
	return newUsageHandlerFixtureWith(t, logs, false)
}

// newUsageHandlerFixtureCapturing is the same fixture with request capture on,
// which is the state the detail route's log join is visible in.
func newUsageHandlerFixtureCapturing(t *testing.T, logs repository.RequestLogRepository) (*UsageHandler, *stubUsageRepo) {
	t.Helper()
	return newUsageHandlerFixtureWith(t, logs, true)
}

func newUsageHandlerFixtureWith(t *testing.T, logs repository.RequestLogRepository, capture bool) (*UsageHandler, *stubUsageRepo) {
	t.Helper()
	now := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	record := domain.RehydrateUsageRecord("usg_010", "req_010", now, "ep_010", "openai", "gky_010", "gpt-4o", "", 4, 5, 1, 0, "0.25", 42, "error", "UPSTREAM_ERROR")
	repo := &stubUsageRepo{record: record}
	store := newStubTokenSaverSettings()
	if capture {
		store.rows[domain.SettingsKeyLogging] = `{"request_capture_enabled":true,"retention_days":7,"capture_body_max_bytes":65536,"observability_max_records":1000}`
	}
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: store})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	svc, err := service.NewUsageService(service.UsageServiceDeps{Usage: repo, Logs: logs, Settings: settings})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}
	return NewUsageHandler(svc), repo
}

// usageSeedLog builds the captured log the detail route joins.
func usageSeedLog(t *testing.T) domain.RequestLog {
	t.Helper()
	now := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	entry, err := domain.NewRequestLog(domain.RequestLogInput{
		RequestID: "req_010", TS: now, Status: domain.RequestLogSuccess,
		RequestBody: "captured in", ResponseBody: "captured out",
	}, now)
	if err != nil {
		t.Fatalf("request log: %v", err)
	}
	return entry
}

// usageRequest builds a request whose path value a Go 1.22 ServeMux installs,
// so the detail route reads the same segment it reads in production.
func usageRequest(method, target, requestID string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	if requestID != "" {
		req.SetPathValue("request_id", requestID)
	}
	return req
}
