// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_recorder_doubles_test.go
// @for       The recording usage repository and service builder the event seam
//
//	tests drive.
//
// @uses      internal/domain, internal/repository, context, sync, testing, time.
// @reason    The recorder's seam test needs a store that remembers what was
//
//	written, because the claim under test is "one stored row, one
//	event". It is separate from the broker doubles because the two answer
//	different questions: the broker double proves what travelled, this
//	one proves what was written before it did.
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

// recordingUsageRepo stores what the recorder wrote and can be told to fail.
type recordingUsageRepo struct {
	mu      sync.Mutex
	records []domain.UsageRecord
	err     error
}

func (r *recordingUsageRepo) Record(_ context.Context, record domain.UsageRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.records = append(r.records, record)
	return nil
}

func (r *recordingUsageRepo) Summary(context.Context, domain.UsageFilter, domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	return domain.UsageTotals{}, nil, nil
}

func (r *recordingUsageRepo) Timeseries(context.Context, domain.UsageFilter, domain.UsageGranularity) ([]domain.RateBucket, error) {
	return nil, nil
}

func (r *recordingUsageRepo) List(context.Context, domain.UsageFilter, repository.PageQuery) ([]domain.UsageRecord, int64, error) {
	return nil, 0, nil
}

func (r *recordingUsageRepo) GetByRequestID(context.Context, string) (domain.UsageRecord, error) {
	return domain.UsageRecord{}, domain.ErrUsageRecordNotFound
}

func (r *recordingUsageRepo) MonthlyUsage(context.Context, string, time.Time) (domain.UsageTotals, error) {
	return domain.UsageTotals{}, nil
}

func (r *recordingUsageRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

// newRecordingUsageService builds the recorder over the in-memory repository and
// the given broker, returning both so a test can assert on each.
func newRecordingUsageService(t *testing.T, bus repository.UsageEventBus) (*UsageService, *recordingUsageRepo) {
	t.Helper()
	repo := &recordingUsageRepo{}
	settings, err := NewSettingsService(SettingsServiceDeps{Repo: newStubSettingsStore()})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	svc, err := NewUsageService(UsageServiceDeps{
		Usage: repo, Settings: settings, Events: NewUsageEventPublisher(bus, nil),
	})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}
	return svc, repo
}

// usageRecordInputFixture is the benign recorder payload the table varies.
func usageRecordInputFixture() domain.UsageRecordInput {
	return domain.UsageRecordInput{
		RequestID: "req_1", EndpointID: "ep_1", ProviderID: "openai", Model: "gpt-4o",
		TokensIn: 4, TokensOut: 3, CostUSD: "0.01", LatencyMS: 42,
		Status: domain.UsageStatusSuccess,
	}
}

// drainEvents runs the publisher's drain to completion and returns what reached
// the broker. Every assertion in this file goes through it: the publisher
// queues, so reading the broker without draining would pass whether or not the
// event was ever enqueued, which is exactly the regression these tests exist to
// catch.
func drainEvents(t *testing.T, svc *UsageService, bus *usageEventBusDouble) [][]byte {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); svc.events.Run(ctx) }()
	cancel()
	<-done
	return bus.payloads()
}
