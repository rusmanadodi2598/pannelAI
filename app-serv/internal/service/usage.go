// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage.go
// @for       The usage recorder and the summary, timeseries, and records reads.
// @uses      internal/domain, internal/repository, context, time.
// @reason    SPEC-API-001 §7.12 makes one aggregate the source for a summary, a
//
//	chart, a paged list, and a single-request detail that joins the
//	captured log. Orchestrating that join, and deciding the default read
//	window, belongs here where no net/http import is needed
//	(AGENTS.md §1.5), so the same reads serve a worker or a future
//	export job.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// UsageService implements SPEC-API-001 §7.12.
const maxUsagePageSize = 100

type UsageService struct {
	usage    repository.UsageRecordRepository
	logs     repository.RequestLogRepository
	settings *SettingsService
	clock    func() time.Time
	events   *UsageEventPublisher
}

// UsageServiceDeps holds the collaborators the service needs.
//
// Settings is required: the detail read reports the capture policy in force so
// the viewer can tell "capture is off" from "this request was never logged".
// Reading it here rather than taking it as a call argument keeps that policy a
// single source of truth, the one the log service already writes by.
//
// Events is optional: without one the recorder still writes every row and
// simply emits no domain event, which is the documented behaviour for a
// deployment that wired no broker.
type UsageServiceDeps struct {
	Usage    repository.UsageRecordRepository
	Logs     repository.RequestLogRepository
	Settings *SettingsService
	Events   *UsageEventPublisher
}

// NewUsageService validates deps and returns a ready service.
func NewUsageService(deps UsageServiceDeps) (*UsageService, error) {
	if deps.Usage == nil {
		return nil, domain.NewValidationError("usage repository is required")
	}
	if deps.Settings == nil {
		return nil, domain.NewValidationError("settings service is required")
	}
	return &UsageService{
		usage: deps.Usage, logs: deps.Logs, settings: deps.Settings,
		clock: time.Now, events: deps.Events,
	}, nil
}

// Record validates and stores one request's accounting row, then emits the
// domain event for it (AGENTS.md §2.3).
//
// The record is validated by the domain constructor, so a reporter that passes
// a negative token count or an unparseable cost is rejected here rather than
// writing a row that would skew every aggregate built on it.
//
// The event is emitted only after the row is stored, and emitting it cannot
// fail the call: the caller already has its answer, and the usage row is the
// durable record a lost event cannot replace. That ordering is what makes the
// event mean "this request is recorded" rather than "this request was
// attempted".
func (s *UsageService) Record(ctx context.Context, in domain.UsageRecordInput) (domain.UsageRecord, error) {
	record, err := domain.NewUsageRecord(in, "", s.clock())
	if err != nil {
		return domain.UsageRecord{}, err
	}
	if err := s.usage.Record(ctx, record); err != nil {
		return domain.UsageRecord{}, fmt.Errorf("recording usage: %w", err)
	}
	s.events.PublishEvent(record.NewUsageEvent())
	return record, nil
}

// Summary returns the filter's totals and, when asked, its group breakdown.
func (s *UsageService) Summary(ctx context.Context, filter domain.UsageFilter, groupBy domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	if err := filter.Validate(); err != nil {
		return domain.UsageTotals{}, nil, err
	}
	if groupBy != "" && !groupBy.IsValid() {
		return domain.UsageTotals{}, nil, domain.NewValidationError("invalid group_by: " + string(groupBy))
	}
	return s.usage.Summary(ctx, filter, groupBy)
}

// Timeseries returns the filter's totals bucketed by the granularity.
func (s *UsageService) Timeseries(ctx context.Context, filter domain.UsageFilter, granularity domain.UsageGranularity) ([]domain.RateBucket, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	if !granularity.IsValid() {
		return nil, domain.NewValidationError("invalid granularity: " + string(granularity))
	}
	return s.usage.Timeseries(ctx, filter, granularity)
}

// Records returns one page of raw records with the total the meta block needs.
func (s *UsageService) Records(ctx context.Context, filter domain.UsageFilter, page, perPage int) ([]domain.UsageRecord, int64, error) {
	if err := filter.Validate(); err != nil {
		return nil, 0, err
	}
	if page < 1 || perPage < 1 || perPage > maxUsagePageSize {
		return nil, 0, domain.NewValidationError("page must be at least 1 and per_page must be between 1 and 100")
	}
	return s.usage.List(ctx, filter, repository.PageQuery{Page: page, PerPage: perPage})
}

// RecordDetail is one request's usage plus its captured log when capture is on.
// CaptureEnabled is reported even when the log row is missing, so the panel can
// tell "capture is off" from "this request was never logged". CaptureBody is
// the effective cap, which the viewer needs to explain a truncation.
type RecordDetail struct {
	Usage          domain.UsageRecord
	Log            *domain.RequestLog
	CaptureEnabled bool
	CaptureBody    int
}

// Detail returns one request's detail, joining the captured log when one exists.
//
// A missing log row is not an error: capture may have been off when the request
// ran, and the usage row is the record of the request regardless. A missing
// usage row is the 404.
func (s *UsageService) Detail(ctx context.Context, requestID string) (RecordDetail, error) {
	record, err := s.usage.GetByRequestID(ctx, requestID)
	if err != nil {
		return RecordDetail{}, err
	}
	settings, err := s.settings.Settings(ctx)
	if err != nil {
		return RecordDetail{}, err
	}
	detail := RecordDetail{
		Usage:          record,
		CaptureEnabled: settings.Logging.RequestCaptureEnabled,
		CaptureBody:    settings.Logging.CaptureBodyMaxBytes,
	}
	if s.logs == nil {
		return detail, nil
	}
	entry, err := s.logs.GetByRequestID(ctx, requestID)
	if err != nil {
		if domain.AsAppError(err).Code == "NOT_FOUND" {
			return detail, nil
		}
		return RecordDetail{}, fmt.Errorf("loading request log: %w", err)
	}
	detail.Log = &entry
	return detail, nil
}
