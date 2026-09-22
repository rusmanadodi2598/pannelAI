// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_live_doubles_test.go
// @for       The doubles and fixtures the live Usage service tests drive.
//
// @uses      internal/domain, internal/repository, context, sync, testing, time.
// @reason    Two seams and one record builder, kept apart from the tests that
//
//	use them so each test file stays inside the AGENTS.md §1.1 budget and
//	so a case reads as its own rule rather than as its own scaffolding.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// liveActiveDouble is an in-memory ActiveRequestStore. It is not a Redis
// double: the store's own Redis behaviour is pinned against a real server in
// internal/repository/redis, and what this test needs is to see exactly which
// markers the service asked to write and remove.
type liveActiveDouble struct {
	mu      sync.Mutex
	started []domain.ActiveRequest
	removed []domain.ActiveRequest
	live    []domain.ActiveRequest
	err     error
}

func (d *liveActiveDouble) Start(_ context.Context, marker domain.ActiveRequest) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// The attempt is recorded before the failure is reported, so a test can see
	// that the tracker tried: a store that fails still received the call.
	d.started = append(d.started, marker)
	if d.err != nil {
		return d.err
	}
	d.live = append(d.live, marker)
	return nil
}

func (d *liveActiveDouble) Finish(_ context.Context, marker domain.ActiveRequest) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.removed = append(d.removed, marker)
	kept := make([]domain.ActiveRequest, 0, len(d.live))
	for _, existing := range d.live {
		if existing.MarkerID != marker.MarkerID {
			kept = append(kept, existing)
		}
	}
	d.live = kept
	return nil
}

func (d *liveActiveDouble) Active(_ context.Context, _ time.Time, limit int) ([]domain.ActiveRequest, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return nil, d.err
	}
	if limit < 1 || limit > len(d.live) {
		limit = len(d.live)
	}
	return append([]domain.ActiveRequest(nil), d.live[:limit]...), nil
}

func (d *liveActiveDouble) startedCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.started)
}

func (d *liveActiveDouble) removedCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.removed)
}

// liveUsageDouble records the filter and page a live read asked for, so the
// bound is asserted rather than assumed, and answers with a fixed page.
type liveUsageDouble struct {
	mu        sync.Mutex
	gotFilter domain.UsageFilter
	gotPage   repository.PageQuery
	records   []domain.UsageRecord
	err       error
	callCount int
}

func (d *liveUsageDouble) Record(context.Context, domain.UsageRecord) error { return nil }

func (d *liveUsageDouble) Summary(context.Context, domain.UsageFilter, domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	return domain.UsageTotals{}, nil, nil
}

func (d *liveUsageDouble) Timeseries(context.Context, domain.UsageFilter, domain.UsageGranularity) ([]domain.RateBucket, error) {
	return nil, nil
}

func (d *liveUsageDouble) List(_ context.Context, filter domain.UsageFilter, q repository.PageQuery) ([]domain.UsageRecord, int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callCount++
	d.gotFilter, d.gotPage = filter, q
	if d.err != nil {
		return nil, 0, d.err
	}
	return append([]domain.UsageRecord(nil), d.records...), int64(len(d.records)), nil
}

func (d *liveUsageDouble) GetByRequestID(context.Context, string) (domain.UsageRecord, error) {
	return domain.UsageRecord{}, domain.ErrUsageRecordNotFound
}

func (d *liveUsageDouble) MonthlyUsage(context.Context, string, time.Time) (domain.UsageTotals, error) {
	return domain.UsageTotals{}, nil
}

// liveRecord builds one aggregate row through the domain constructor.
func liveRecord(t *testing.T, requestID, providerID, model string, status domain.UsageStatus, ts time.Time) domain.UsageRecord {
	t.Helper()
	record, err := domain.NewUsageRecord(domain.UsageRecordInput{
		RequestID: requestID, TS: ts, ProviderID: providerID, Model: model,
		TokensIn: 3, TokensOut: 4, CostUSD: "0.00050000", Status: status,
	}, "", ts)
	if err != nil {
		t.Fatalf("building the record fixture: %v", err)
	}
	return record
}

// liveServiceFixture builds the service over the two doubles with a pinned
// clock, so every window assertion is arithmetic rather than a wait.
func liveServiceFixture(t *testing.T, now time.Time) (*UsageLiveService, *liveActiveDouble, *liveUsageDouble) {
	t.Helper()
	active := &liveActiveDouble{}
	usage := &liveUsageDouble{}
	svc, err := NewUsageLiveService(UsageLiveServiceDeps{Active: active, Usage: usage})
	if err != nil {
		t.Fatalf("NewUsageLiveService() = %v, want nil", err)
	}
	svc.clock = func() time.Time { return now }
	return svc, active, usage
}
