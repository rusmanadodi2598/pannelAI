//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/usage_event_live_test.go
// @for       F4 live evidence: one recorded request whose domain event crosses a
//
//	real Redis and lands in the console ring a real consumer reads.
//
// @uses      internal/domain, internal/repository/redis, internal/service,
//
//	context, strings, testing, time.
//
// @reason    F4 of docs/DRAFT/010-USAGE-ENDPOINT-READINESS.md is closed by "the
//
//	event has at least one publisher and one subscriber in production".
//	Unit tests can show each half works against a double; only a live
//	pass shows the two halves meet on the same channel with the real
//	client, which is the part that was missing for the whole life of
//	this event. The evidence is a console line the consumer wrote from
//	an event the recorder emitted, with the request id an operator can
//	join back to the usage row.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration -run TestUsageEventLive ./cmd/app-serv/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TestUsageEventLive_RecordedRequestReachesTheConsole is the F4 live pass.
func TestUsageEventLive_RecordedRequestReachesTheConsole(t *testing.T) {
	stack := newLiveStack(t, newLiveUpstream(t))
	ctx := context.Background()

	// The publisher and the consumer the composition root builds, over the
	// stack's real Redis client and the real log service.
	bus := redisrepo.NewUsageEventBus(stack.redis)
	publisher := service.NewUsageEventPublisher(bus, nil)
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{
		Repo: newLiveSettingsRepo(stack),
	})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	logSvc, err := service.NewLogService(service.LogServiceDeps{
		Logs: newLiveLogRepo(stack), Settings: settings, Console: redisrepo.NewConsoleBuffer(stack.redis),
	})
	if err != nil {
		t.Fatalf("log service: %v", err)
	}
	consumer := service.NewUsageEventConsumer(bus, logSvc, nil)
	usage, err := service.NewUsageService(service.UsageServiceDeps{
		Usage: newLiveUsageRepo(stack), Settings: settings, Events: publisher,
	})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go publisher.Run(runCtx)
	go consumer.Run(runCtx)

	// The console ring is cleared so the line this pass asserts on belongs to
	// this run alone.
	if err := logSvc.ClearConsole(ctx); err != nil {
		t.Fatalf("clearing the console: %v", err)
	}
	// Pub/Sub does not replay, so the subscription has to be live before the
	// event is published. A short wait is the honest way to observe that
	// against a real broker.
	time.Sleep(250 * time.Millisecond)

	requestID := "req_f4_live_" + domain.NewULID(time.Now())
	if _, err := usage.Record(ctx, domain.UsageRecordInput{
		RequestID: requestID, ProviderID: "openai", EndpointID: "ep_f4_live",
		Model: "gpt-4o", TokensIn: 4, TokensOut: 3, CostUSD: "0.01000000",
		Status: domain.UsageStatusSuccess,
	}); err != nil {
		t.Fatalf("recording the request: %v", err)
	}

	// Read the ring through the same service the console route reads.
	lines, maxRecords, err := waitForConsoleLine(t, logSvc, requestID)
	if err != nil {
		t.Fatalf("reading the console ring: %v", err)
	}
	t.Logf("live evidence: request_id=%s console_lines=%d max_records=%d", requestID, len(lines), maxRecords)

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, requestID) {
		t.Fatalf("the console ring does not carry the request id; lines = %q", lines)
	}
	if !strings.Contains(joined, "status=success") || !strings.Contains(joined, "tokens=7") {
		t.Fatalf("the console line does not describe the event; lines = %q", lines)
	}
	if !strings.Contains(joined, "cost=0.01000000") {
		t.Fatalf("the console line does not carry the recorded cost; lines = %q", lines)
	}
}

// TestUsageEventLive_PublishFailureDoesNotFailTheRequest asserts the policy the
// live stack can prove: a broker that refuses a publish leaves the request
// served and the accounting row written.
func TestUsageEventLive_PublishFailureDoesNotFailTheRequest(t *testing.T) {
	stack := newLiveStack(t, newLiveUpstream(t))
	ctx := context.Background()

	// A client pointed at a port nothing listens on is the cheapest honest way
	// to make the broker fail while every other dependency stays real.
	broken := brokenRedisClient(t)
	publisher := service.NewUsageEventPublisher(redisrepo.NewUsageEventBus(broken), nil)
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: newLiveSettingsRepo(stack)})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	usage, err := service.NewUsageService(service.UsageServiceDeps{
		Usage: newLiveUsageRepo(stack), Settings: settings, Events: publisher,
	})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go publisher.Run(runCtx)

	requestID := "req_f4_broker_down_" + domain.NewULID(time.Now())
	record, err := usage.Record(ctx, domain.UsageRecordInput{
		RequestID: requestID, ProviderID: "openai", Model: "gpt-4o",
		TokensIn: 1, TokensOut: 1, CostUSD: "0.01", Status: domain.UsageStatusSuccess,
	})
	if err != nil {
		t.Fatalf("Record() with a broken broker = %v, want nil", err)
	}
	if record.RequestID() != requestID {
		t.Fatalf("recorded request id = %q, want %q", record.RequestID(), requestID)
	}
	// The row is the durable record and has to be there regardless.
	var rows int
	if err := stack.pool.QueryRow(ctx, "SELECT count(*) FROM usage_records WHERE request_id = $1", requestID).Scan(&rows); err != nil {
		t.Fatalf("reading usage rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("usage rows = %d, want 1 with a broken broker", rows)
	}

	// Give the drain time to attempt the publish and count the failure.
	waitForFailures(t, publisher, 1)
	t.Logf("live evidence: request_id=%s usage_rows=%d publish_failures=%d", requestID, rows, publisher.Failed())
}

// waitForConsoleLine polls the console ring until the request id appears, so the
// evidence is read after the consumer wrote it rather than racing the stream.
func waitForConsoleLine(t *testing.T, logSvc *service.LogService, requestID string) ([]string, int, error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastLines []string
	var lastMax int
	for time.Now().Before(deadline) {
		lines, maxRecords, err := logSvc.Console(context.Background())
		if err != nil {
			return nil, 0, err
		}
		lastLines, lastMax = lines, maxRecords
		if strings.Contains(strings.Join(lines, "\n"), requestID) {
			return lines, maxRecords, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return lastLines, lastMax, nil
}

// waitForFailures polls until the publisher has counted want failures.
func waitForFailures(t *testing.T, publisher *service.UsageEventPublisher, want int64) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if publisher.Failed() >= want {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("publish failures = %d, want at least %d", publisher.Failed(), want)
}
