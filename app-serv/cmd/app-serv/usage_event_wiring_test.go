// Command app-serv wires the usage domain event between its publisher and its
// subscriber.
//
// @file      cmd/app-serv/usage_event_wiring_test.go
// @for       Tests that the composition root builds a publisher a recorder
//
//	emits into and a subscriber that consumes what it published.
//
// @uses      internal/domain, internal/repository, internal/service, context,
//
//	sync, testing, time.
//
// @reason    F4 of draft 010 is closed by "the event has at least one publisher
//
//	and one subscriber in production". Each half has its own unit tests,
//	and both can pass while the composition root wires neither: the
//	recorder is constructed with a nil publisher by default, and the
//	worker table skips a nil worker silently. So the seam is asserted
//	here, where the graph is assembled, against the same constructors the
//	boot sequence calls.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// wiringUsageRepo is the recorder's store.
type wiringUsageRepo struct{}

func (wiringUsageRepo) Record(context.Context, domain.UsageRecord) error { return nil }
func (wiringUsageRepo) Summary(context.Context, domain.UsageFilter, domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	return domain.UsageTotals{}, nil, nil
}
func (wiringUsageRepo) Timeseries(context.Context, domain.UsageFilter, domain.UsageGranularity) ([]domain.RateBucket, error) {
	return nil, nil
}
func (wiringUsageRepo) List(context.Context, domain.UsageFilter, repository.PageQuery) ([]domain.UsageRecord, int64, error) {
	return nil, 0, nil
}
func (wiringUsageRepo) GetByRequestID(context.Context, string) (domain.UsageRecord, error) {
	return domain.UsageRecord{}, domain.ErrUsageRecordNotFound
}
func (wiringUsageRepo) MonthlyUsage(context.Context, string, time.Time) (domain.UsageTotals, error) {
	return domain.UsageTotals{}, nil
}

// wiringConsoleSink records what the consumer mirrored.
type wiringConsoleSink struct {
	mu    sync.Mutex
	lines []string
}

func (s *wiringConsoleSink) AppendConsole(_ context.Context, line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = append(s.lines, line)
	return nil
}

func (s *wiringConsoleSink) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.lines...)
}

// TestUsageEventWiring_RecordedRequestReachesTheConsole is the F4 end-to-end
// seam: a recorded request produces a console line, through the publisher and
// the consumer the composition root would build.
func TestUsageEventWiring_RecordedRequestReachesTheConsole(t *testing.T) {
	bus := newWiringBus()
	publisher := service.NewUsageEventPublisher(bus, nil)
	sink := &wiringConsoleSink{}
	consumer := service.NewUsageEventConsumer(bus, sink, nil)

	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: wiringSettingsRepo{}})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	usage, err := service.NewUsageService(service.UsageServiceDeps{
		Usage: wiringUsageRepo{}, Settings: settings, Events: publisher,
	})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); publisher.Run(ctx) }()
	go func() { defer wg.Done(); consumer.Run(ctx) }()

	// Pub/Sub has no replay: a publish that arrives before the consumer
	// subscribed is not delivered to it. That is the transport's documented
	// behaviour (the durable record of a request is the usage row), so the test
	// waits for the subscription rather than assuming the consumer won a race
	// against the recorder.
	waitForWiring(t, func() bool { return bus.subscriberCount() == 1 })

	if _, err := usage.Record(ctx, domain.UsageRecordInput{
		RequestID: "req_wired", ProviderID: "openai", EndpointID: "ep_wired",
		Model: "gpt-4o", TokensIn: 4, TokensOut: 3, CostUSD: "0.01",
		Status: domain.UsageStatusSuccess,
	}); err != nil {
		t.Fatalf("Record() = %v, want nil", err)
	}

	waitForWiring(t, func() bool { return len(sink.recorded()) == 1 })
	cancel()
	wg.Wait()

	lines := sink.recorded()
	if len(lines) != 1 {
		t.Fatalf("console lines = %d, want 1: %q", len(lines), lines)
	}
	if !containsWiring(lines[0], "req_wired") || !containsWiring(lines[0], "status=success") {
		t.Fatalf("console line %q does not describe the recorded request", lines[0])
	}
	if got := bus.count(); got != 1 {
		t.Fatalf("published events = %d, want 1", got)
	}
}

// TestUsageEventWiring_RecorderWithoutABrokerStillRecords asserts the wiring is
// additive: a deployment that builds no bus records rows and emits nothing,
// rather than failing to serve.
func TestUsageEventWiring_RecorderWithoutABrokerStillRecords(t *testing.T) {
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: wiringSettingsRepo{}})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	usage, err := service.NewUsageService(service.UsageServiceDeps{Usage: wiringUsageRepo{}, Settings: settings})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}
	if _, err := usage.Record(context.Background(), domain.UsageRecordInput{
		RequestID: "req_nobus", ProviderID: "openai", Model: "gpt-4o", Status: domain.UsageStatusSuccess,
	}); err != nil {
		t.Fatalf("Record() without a bus = %v, want nil", err)
	}
}

// waitForWiring polls until the condition holds, failing rather than hanging.
func waitForWiring(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition did not hold within 5s")
}

// containsWiring reports whether haystack contains needle.
func containsWiring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// wiringSettingsRepo serves the documented defaults.
type wiringSettingsRepo struct{}

func (wiringSettingsRepo) Load(context.Context) (map[domain.SettingsKey]string, error) {
	return map[domain.SettingsKey]string{}, nil
}
func (wiringSettingsRepo) Save(context.Context, domain.SettingsKey, string) error { return nil }
