// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_usage_routes_test.go
// @for       Route-table tests for the four §7.12 usage reads: the session gate,
//
//	the verb the mux refuses, and the request id every answer echoes.
//
// @uses      context, net/http, net/http/httptest, strings, testing, time,
//
//	internal/domain, internal/handler, internal/repository, internal/service.
//
// @reason    Draft 010 F1: the handler tests drive the methods directly, so the
//
//	gate and the verb table those methods sit behind had no test that
//	could fail if a registration lost its `gateway()` wrapper. These
//	cases drive the real mux with the production auth fixture, which is
//	what makes the 401 and the 405 the mux's own answers rather than
//	the handler's.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-22
package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// routerUsageRepo is the in-memory UsageRecordRepository for the route tests:
// one seeded row answers every read, so the route is driven end to end without
// a database.
type routerUsageRepo struct {
	record domain.UsageRecord
}

func (r *routerUsageRepo) Record(context.Context, domain.UsageRecord) error { return nil }
func (r *routerUsageRepo) Summary(context.Context, domain.UsageFilter, domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	return r.record.Totals(), nil, nil
}
func (r *routerUsageRepo) Timeseries(context.Context, domain.UsageFilter, domain.UsageGranularity) ([]domain.RateBucket, error) {
	return []domain.RateBucket{{Bucket: r.record.TS(), Totals: r.record.Totals()}}, nil
}
func (r *routerUsageRepo) List(context.Context, domain.UsageFilter, repository.PageQuery) ([]domain.UsageRecord, int64, error) {
	return []domain.UsageRecord{r.record}, 1, nil
}
func (r *routerUsageRepo) GetByRequestID(_ context.Context, requestID string) (domain.UsageRecord, error) {
	if requestID != r.record.RequestID() {
		return domain.UsageRecord{}, domain.ErrUsageRecordNotFound
	}
	return r.record, nil
}
func (r *routerUsageRepo) MonthlyUsage(context.Context, string, time.Time) (domain.UsageTotals, error) {
	return domain.UsageTotals{}, nil
}

// usageRoutePaths are the four reads, each of which the mux must gate and
// refuse a write verb on. The detail path carries a concrete id because the
// pattern is what is under test, not the lookup.
var usageRoutePaths = []string{
	"/api/v1/usage/summary",
	"/api/v1/usage/timeseries",
	"/api/v1/usage/records",
	"/api/v1/usage/records/req_010",
}

// TestUsageRoutes_RequireSession pins the gate per route, so a registration
// that lost its `gateway()` wrapper fails here and not only in the whole-table
// sweep.
func TestUsageRoutes_RequireSession(t *testing.T) {
	mux, _ := newUsageRouter(t)
	for _, path := range usageRoutePaths {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s without a session = %d, want 401 (body: %s)", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"UNAUTHORIZED"`) {
			t.Fatalf("GET %s body = %s, want the UNAUTHORIZED code", path, rec.Body.String())
		}
	}
}

// TestUsageRoutes_ThroughMux drives the four reads with a session and the verb
// none of them registers, so the served shape and the §8 verb envelope are
// pinned through the real mux.
func TestUsageRoutes_ThroughMux(t *testing.T) {
	mux, _ := newUsageRouter(t)
	cookie := loginCookie(t, mux)

	read := func(path string) string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookie)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200 (body: %s)", path, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	if body := read("/api/v1/usage/summary"); !strings.Contains(body, `"requests":1`) {
		t.Fatalf("summary body = %s, want the seeded totals", body)
	}
	if body := read("/api/v1/usage/timeseries"); !strings.Contains(body, `"granularity":"hour"`) {
		t.Fatalf("timeseries body = %s, want the default granularity", body)
	}
	if body := read("/api/v1/usage/records"); !strings.Contains(body, `"request_id":"req_010"`) {
		t.Fatalf("records body = %s, want the seeded row", body)
	}
	if body := read("/api/v1/usage/records/req_010"); !strings.Contains(body, `"capture_enabled":false`) {
		t.Fatalf("detail body = %s, want the capture flag", body)
	}

	for _, path := range usageRoutePaths {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.AddCookie(cookie)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s = %d, want 405 (body: %s)", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"METHOD_NOT_ALLOWED"`) {
			t.Fatalf("POST %s body = %s, want the METHOD_NOT_ALLOWED code", path, rec.Body.String())
		}
	}
}

// TestUsageRoutes_EchoRequestID pins that a caller-supplied trace id survives
// the middleware chain on a usage read, so the panel can correlate its own
// request with the access log line.
func TestUsageRoutes_EchoRequestID(t *testing.T) {
	mux, _ := newUsageRouter(t)
	cookie := loginCookie(t, mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/summary", nil)
	req.Header.Set(RequestIDHeader, "trace-usage-010")
	req.AddCookie(cookie)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get(RequestIDHeader); got != "trace-usage-010" {
		t.Fatalf("response %s = %q, want the caller's trace id", RequestIDHeader, got)
	}
}

// newUsageRouter builds the real mux with the §7.12 handler wired over the
// in-memory repositories, so the gate and the verb table under test are the
// production ones.
func newUsageRouter(t *testing.T) (*Mux, *routerUsageRepo) {
	t.Helper()
	_, authHandler := newAuthenticatedRouter(t)
	now := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	repo := &routerUsageRepo{record: domain.RehydrateUsageRecord(
		"usg_010", "req_010", now, "ep_010", "openai", "gky_010", "gpt-4o", "", 4, 5, 1, 0, "0.25", 42, "success", "")}
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: newMemSettingsRepo()})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	usage, err := service.NewUsageService(service.UsageServiceDeps{Usage: repo, Settings: settings})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Auth:  authHandler,
		Usage: handler.NewUsageHandler(usage),
	}), repo
}
