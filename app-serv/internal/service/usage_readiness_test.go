// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_readiness_test.go
// @for       Usage read validation, pagination, aggregation delegation, and detail joining.
// @uses      internal/domain, internal/repository, context, testing, time.
// @reason    SPEC-API-001 §7.12 exposes four reads whose bounded filters and
//
//	captured-log join are service behavior. These doubles keep the
//	service contract tested without a database or HTTP transport.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

type usageReadRepo struct {
	record domain.UsageRecord
	called repository.PageQuery
}

func (r *usageReadRepo) Record(context.Context, domain.UsageRecord) error { return nil }
func (r *usageReadRepo) Summary(context.Context, domain.UsageFilter, domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	return r.record.Totals(), []domain.UsageGroupRow{{Key: r.record.ProviderID(), Totals: r.record.Totals()}}, nil
}
func (r *usageReadRepo) Timeseries(context.Context, domain.UsageFilter, domain.UsageGranularity) ([]domain.RateBucket, error) {
	return []domain.RateBucket{{Bucket: r.record.TS(), Totals: r.record.Totals()}}, nil
}
func (r *usageReadRepo) List(_ context.Context, _ domain.UsageFilter, q repository.PageQuery) ([]domain.UsageRecord, int64, error) {
	r.called = q
	return []domain.UsageRecord{r.record}, 1, nil
}
func (r *usageReadRepo) GetByRequestID(context.Context, string) (domain.UsageRecord, error) {
	return r.record, nil
}
func (r *usageReadRepo) MonthlyUsage(context.Context, string, time.Time) (domain.UsageTotals, error) {
	return r.record.Totals(), nil
}

type usageReadLogs struct{ entry domain.RequestLog }

func (l usageReadLogs) Insert(context.Context, domain.RequestLog) error { return nil }
func (l usageReadLogs) List(context.Context, domain.LogFilter, repository.PageQuery) ([]domain.RequestLog, int64, error) {
	return nil, 0, nil
}
func (l usageReadLogs) GetByRequestID(context.Context, string) (domain.RequestLog, error) {
	return l.entry, nil
}
func (l usageReadLogs) DeleteOlderThan(context.Context, time.Time) (int64, error) { return 0, nil }

type missingUsageReadLogs struct{ usageReadLogs }

func (missingUsageReadLogs) GetByRequestID(context.Context, string) (domain.RequestLog, error) {
	return domain.RequestLog{}, domain.ErrRequestLogNotFound
}

func newUsageReadService(t *testing.T, logs repository.RequestLogRepository) (*UsageService, *usageReadRepo) {
	t.Helper()
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	record := domain.RehydrateUsageRecord("usg_1", "req_1", now, "ep_1", "provider_1", "key_1", "model_1", "combo", 4, 5, 1, 0, "0.25", 42, "success", "")
	repo := &usageReadRepo{record: record}
	settings, err := NewSettingsService(SettingsServiceDeps{Repo: newStubSettingsStore()})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	svc, err := NewUsageService(UsageServiceDeps{Usage: repo, Logs: logs, Settings: settings})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}
	return svc, repo
}

func TestUsageService_ReadsValidateAndDelegate(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	filter := domain.NewUsageFilter(domain.UsageFilterInput{}, now)
	svc, repo := newUsageReadService(t, nil)
	if _, _, err := svc.Summary(context.Background(), filter, domain.UsageGroupBy("unknown")); domain.AsAppError(err).Code != "VALIDATION_ERROR" {
		t.Fatalf("invalid group_by error = %v, want VALIDATION_ERROR", err)
	}
	if _, err := svc.Timeseries(context.Background(), filter, domain.UsageGranularity("unknown")); domain.AsAppError(err).Code != "VALIDATION_ERROR" {
		t.Fatalf("invalid granularity error = %v, want VALIDATION_ERROR", err)
	}
	if _, _, err := svc.Records(context.Background(), filter, 0, 25); domain.AsAppError(err).Code != "VALIDATION_ERROR" {
		t.Fatalf("invalid page error = %v, want VALIDATION_ERROR", err)
	}
	if _, _, err := svc.Records(context.Background(), filter, 2, 25); err != nil {
		t.Fatalf("records: %v", err)
	}
	if repo.called.Page != 2 || repo.called.PerPage != 25 {
		t.Fatalf("page query = %+v", repo.called)
	}
	if _, _, err := svc.Summary(context.Background(), filter, domain.UsageGroupProvider); err != nil {
		t.Fatalf("summary: %v", err)
	}
	if _, err := svc.Timeseries(context.Background(), filter, domain.UsageHourly); err != nil {
		t.Fatalf("timeseries: %v", err)
	}
}

func TestUsageService_DetailJoinsCapturedLogAndToleratesMissingLog(t *testing.T) {
	logNow := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	entry, err := domain.NewRequestLog(domain.RequestLogInput{RequestID: "req_1", TS: logNow, Status: domain.RequestLogSuccess, RequestBody: "in", ResponseBody: "out"}, logNow)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	svc, _ := newUsageReadService(t, usageReadLogs{entry: entry})
	detail, err := svc.Detail(context.Background(), "req_1")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Log == nil || detail.Log.RequestBody() != "in" {
		t.Fatalf("detail log = %+v, want captured body", detail.Log)
	}
	if detail.CaptureEnabled {
		t.Fatal("capture default = true, want false")
	}

	missing, _ := newUsageReadService(t, missingUsageReadLogs{})
	detail, err = missing.Detail(context.Background(), "req_1")
	if err != nil {
		t.Fatalf("missing log detail: %v", err)
	}
	if detail.Log != nil {
		t.Fatal("missing log returned a non-nil entry")
	}
}
