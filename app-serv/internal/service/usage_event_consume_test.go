// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_consume_test.go
// @for       Table-driven tests for the usage event consumer: what one event
//
//	renders, what a malformed payload does, and how a failing
//	subscription recovers.
//
// @uses      internal/domain, internal/repository, context, errors, sync,
//
//	testing, time.
//
// @reason    The consumer is the half that makes the event real (F4's completion
//
//	criterion): without it the publisher writes to a channel nobody
//	reads. Its rules are the ones that fail quietly — a malformed payload
//	that takes the consumer down stops every later event, and a receive
//	timeout mistaken for a fault turns an idle channel into a reconnect
//	loop — so the double counts receives and the table asserts on lines
//	the sink actually received.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageEventConsumer_MirrorsEachEventIntoTheConsole is the completion
// evidence for F4: a real consumer turns a real published payload into a line
// an operator can read. The table varies the event's own fields, so the line is
// built from the event rather than from anything the test hardcoded.
func TestUsageEventConsumer_MirrorsEachEventIntoTheConsole(t *testing.T) {
	cases := []struct {
		name        string
		event       domain.UsageEvent
		wantContain []string
	}{
		{
			name:        "a successful call renders its identity and counters",
			event:       publishedEventFixture(),
			wantContain: []string{"req_1", "openai", "ep_1", "gpt-4o", "tokens=7", "cost=0.01000000", "status=success"},
		},
		{
			name: "a failed call renders as an error",
			event: func() domain.UsageEvent {
				event := publishedEventFixture()
				event.RequestID = "req_2"
				event.Status = domain.UsageStatusError
				event.TotalTokens = 0
				event.CostUSD = "0.00000000"
				return event
			}(),
			wantContain: []string{"req_2", "tokens=0", "status=error"},
		},
		{
			name: "an endpoint-less call renders an empty endpoint",
			event: func() domain.UsageEvent {
				event := publishedEventFixture()
				event.RequestID = "req_3"
				event.EndpointID = ""
				return event
			}(),
			wantContain: []string{"req_3", "endpoint=", "status=success"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := domain.EncodeUsageEvent(tc.event)
			if err != nil {
				t.Fatalf("encoding the fixture: %v", err)
			}
			sink := &consoleLineSink{}
			bus := &usageEventBusDouble{
				subscription: &usageEventSubscriptionDouble{payloads: [][]byte{payload}},
			}
			consumer := NewUsageEventConsumer(bus, sink, nil)

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { defer close(done); consumer.Run(ctx) }()
			waitForLines(t, sink, 1)
			cancel()
			<-done

			lines := sink.recorded()
			if len(lines) != 1 {
				t.Fatalf("console lines = %d, want 1: %q", len(lines), lines)
			}
			for _, want := range tc.wantContain {
				if !contains(lines[0], want) {
					t.Fatalf("line %q does not contain %q", lines[0], want)
				}
			}
		})
	}
}

// TestUsageEventConsumer_SurvivesAMalformedPayload asserts one bad message does
// not take the consumer down: the valid event that follows it still arrives.
func TestUsageEventConsumer_SurvivesAMalformedPayload(t *testing.T) {
	valid, err := domain.EncodeUsageEvent(publishedEventFixture())
	if err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	cases := []struct {
		name    string
		payload []byte
	}{
		{name: "a truncated payload", payload: valid[:len(valid)/2]},
		{name: "a non-JSON payload", payload: []byte("not json at all")},
		{name: "an empty payload", payload: nil},
		{name: "a misdelivered event name", payload: []byte(`{"event":"usage.deleted","request_id":"req_x"}`)},
		{name: "a payload with an unknown field", payload: []byte(`{"event":"usage.recorded","request_id":"req_x","role":"admin"}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &consoleLineSink{}
			bus := &usageEventBusDouble{
				subscription: &usageEventSubscriptionDouble{payloads: [][]byte{tc.payload, valid}},
			}
			consumer := NewUsageEventConsumer(bus, sink, nil)

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { defer close(done); consumer.Run(ctx) }()
			waitForLines(t, sink, 1)
			cancel()
			<-done

			lines := sink.recorded()
			if len(lines) != 1 {
				t.Fatalf("console lines = %d, want 1 (only the valid event): %q", len(lines), lines)
			}
			if !contains(lines[0], "req_1") {
				t.Fatalf("line %q is not the valid event", lines[0])
			}
		})
	}
}

// TestUsageEventConsumer_StopsOnCancellation asserts the termination condition:
// a consumer waiting on a quiet channel returns promptly when its context is
// cancelled, and closes its subscription on the way out.
func TestUsageEventConsumer_StopsOnCancellation(t *testing.T) {
	subscription := &usageEventSubscriptionDouble{}
	bus := &usageEventBusDouble{subscription: subscription}
	consumer := NewUsageEventConsumer(bus, &consoleLineSink{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); consumer.Run(ctx) }()
	// Let the consumer reach its receive loop before cancelling, so the test
	// proves the wait is interruptible rather than that Run never started.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	if !subscription.wasClosed() {
		t.Fatal("subscription was not closed on the way out")
	}
}

// waitForLines blocks until the sink has recorded at least want lines.
func waitForLines(t *testing.T, sink *consoleLineSink, want int) {
	t.Helper()
	waitFor(t, func() bool { return len(sink.recorded()) >= want })
}

// waitFor polls until the condition holds, failing the test on a timeout so a
// regression reports as a failure rather than a hang.
func waitFor(t *testing.T, condition func() bool) {
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
