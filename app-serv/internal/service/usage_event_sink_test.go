// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_sink_test.go
// @for       Tests for the usage event consumer's behaviour when its console
//
//	sink fails.
//
// @uses      internal/domain, context, errors, testing.
// @reason    The ring is a view of the event, not the record: a Redis that is
//
//	down has to leave the consumer consuming, or one failing dependency
//	stops every later event from being observed. That distinction is the
//	whole reason the consumer writes nothing durable, so it is pinned on
//	its own rather than as a case in the seam table.
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

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageEventConsumer_ConsoleFailureDoesNotStopConsumption asserts a failing
// sink is a view failure, not a consumer failure: the next event still arrives.
func TestUsageEventConsumer_ConsoleFailureDoesNotStopConsumption(t *testing.T) {
	first, err := domain.EncodeUsageEvent(publishedEventFixture())
	if err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	second := publishedEventFixture()
	second.RequestID = "req_after_failure"
	secondPayload, err := domain.EncodeUsageEvent(second)
	if err != nil {
		t.Fatalf("encoding the second fixture: %v", err)
	}

	sink := &consoleLineSink{err: errors.New("redis is down")}
	bus := &usageEventBusDouble{
		subscription: &usageEventSubscriptionDouble{payloads: [][]byte{first, secondPayload}},
	}
	consumer := NewUsageEventConsumer(bus, sink, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); consumer.Run(ctx) }()
	// Both payloads are delivered even though neither append succeeds; the
	// subscription records how many it handed over.
	subscription := bus.subscription.(*usageEventSubscriptionDouble)
	waitFor(t, func() bool { return subscription.deliveredCount() == 2 })
	cancel()
	<-done

	if got := len(sink.recorded()); got != 0 {
		t.Fatalf("console lines = %d, want 0 while the sink fails", got)
	}
}
