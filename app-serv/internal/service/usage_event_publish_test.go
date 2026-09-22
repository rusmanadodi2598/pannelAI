// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_publish_test.go
// @for       Table-driven tests for the usage event publisher: the drop policy,
//
//	the publish failure policy, the panic boundary, and the drain.
//
// @uses      internal/domain, internal/repository, context, errors, sync,
//
//	testing, time.
//
// @reason    The publisher owns the one path that must never block a request, a
//
//	bounded queue whose overflow is a deliberate drop, and a goroutine
//	that has to stop. Each of those is a rule that fails silently when
//	it regresses: a blocking queue shows up as latency, a dropped event
//	shows up as nothing at all. So the double counts what it received and
//	the table asserts the counts, not just the absence of a crash.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageEventPublisher_PublishesEveryQueuedEvent drives the happy path and
// the drop policy through one table: the count published is asserted against
// the count enqueued, so a publisher that silently dropped everything fails.
func TestUsageEventPublisher_PublishesEveryQueuedEvent(t *testing.T) {
	cases := []struct {
		name    string
		events  int
		wantPub int
	}{
		{name: "a single event is published", events: 1, wantPub: 1},
		{name: "a burst is published", events: 25, wantPub: 25},
		{name: "no event publishes nothing", events: 0, wantPub: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := &usageEventBusDouble{}
			publisher := NewUsageEventPublisher(bus, nil)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { defer close(done); publisher.Run(ctx) }()

			for i := 0; i < tc.events; i++ {
				publisher.PublishEvent(publishedEventFixture())
			}
			// Cancel and join, so the assertion happens after the drain rather
			// than racing it.
			cancel()
			<-done

			if got := bus.count(); got != tc.wantPub {
				t.Fatalf("published = %d, want %d", got, tc.wantPub)
			}
			if publisher.Dropped() != 0 {
				t.Fatalf("dropped = %d, want 0", publisher.Dropped())
			}
			for _, payload := range bus.payloads() {
				if _, err := domain.DecodeUsageEvent(payload); err != nil {
					t.Fatalf("published payload does not decode: %v", err)
				}
			}
		})
	}
}

// TestUsageEventPublisher_FullQueueDropsAndCounts asserts the overflow policy:
// a full queue drops the newest event, counts it, and never blocks the caller.
func TestUsageEventPublisher_FullQueueDropsAndCounts(t *testing.T) {
	// A blocked publisher is what fills the queue: the drain goroutine is stuck
	// in the broker, so nothing is consumed while the caller enqueues.
	release := make(chan struct{})
	bus := &usageEventBusDouble{publishFn: func(context.Context, []byte) error {
		<-release
		return nil
	}}
	publisher := NewUsageEventPublisher(bus, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); publisher.Run(ctx) }()

	enqueued := usageEventQueueDepth + 16
	blocked := make(chan struct{})
	go func() {
		for i := 0; i < enqueued; i++ {
			publisher.PublishEvent(publishedEventFixture())
		}
		close(blocked)
	}()
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("PublishEvent blocked the caller, want a drop instead")
	}
	if got := publisher.Dropped(); got < 1 {
		t.Fatalf("dropped = %d, want at least 1 over a depth of %d", got, usageEventQueueDepth)
	}
	close(release)
	cancel()
	<-done
}

// TestUsageEventPublisher_PublishFailureIsCountedNotReturned asserts a broker
// failure is bookkeeping: it is counted, it does not panic, and the goroutine
// keeps consuming later events.
func TestUsageEventPublisher_PublishFailureIsCountedNotReturned(t *testing.T) {
	cases := []struct {
		name     string
		publish  func(context.Context, []byte) error
		wantFail int64
	}{
		{name: "a broker error is counted", publish: func(context.Context, []byte) error {
			return errors.New("broker is down")
		}, wantFail: 1},
		{name: "a panic in the broker is contained", publish: func(context.Context, []byte) error {
			panic("broker exploded")
		}, wantFail: 1},
		{name: "a healthy broker counts nothing", publish: func(context.Context, []byte) error { return nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := &usageEventBusDouble{publishFn: tc.publish}
			publisher := NewUsageEventPublisher(bus, nil)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { defer close(done); publisher.Run(ctx) }()

			publisher.PublishEvent(publishedEventFixture())
			cancel()
			<-done

			if got := publisher.Failed(); got != tc.wantFail {
				t.Fatalf("failed = %d, want %d", got, tc.wantFail)
			}
		})
	}
}

// TestUsageEventPublisher_RefusesAnInvalidEvent asserts the publisher never puts
// a payload on the channel that a consumer would have to refuse.
func TestUsageEventPublisher_RefusesAnInvalidEvent(t *testing.T) {
	bus := &usageEventBusDouble{}
	publisher := NewUsageEventPublisher(bus, nil)

	publisher.PublishEvent(domain.UsageEvent{RequestID: "", ProviderID: "openai", Model: "gpt"})

	if got := bus.count(); got != 0 {
		t.Fatalf("published = %d, want 0 for an event with no request id", got)
	}
}

// TestUsageEventPublisher_NilBusIsInert asserts a deployment without a broker
// keeps working: publishing is a no-op and Run returns.
func TestUsageEventPublisher_NilBusIsInert(t *testing.T) {
	publisher := NewUsageEventPublisher(nil, nil)
	publisher.PublishEvent(publishedEventFixture())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	publisher.Run(ctx)
	if publisher.Dropped() != 0 || publisher.Failed() != 0 {
		t.Fatalf("nil bus reported drops=%d failures=%d, want none", publisher.Dropped(), publisher.Failed())
	}
}

// TestUsageEventPublisher_EnqueuesWithoutARunningDrain asserts the queue is what
// separates the request path from the broker: publishing succeeds before Run is
// ever called, and the drain publishes it afterwards.
func TestUsageEventPublisher_EnqueuesWithoutARunningDrain(t *testing.T) {
	bus := &usageEventBusDouble{}
	publisher := NewUsageEventPublisher(bus, nil)
	publisher.PublishEvent(publishedEventFixture())
	if got := bus.count(); got != 0 {
		t.Fatalf("published = %d before Run, want 0", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); publisher.Run(ctx) }()
	cancel()
	<-done

	if got := bus.count(); got != 1 {
		t.Fatalf("published = %d after the drain, want 1", got)
	}
}
