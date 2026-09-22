// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_degenerate_test.go
// @for       Tests for the usage event consumer's degenerate wiring: no broker,
//
//	and no console ring.
//
// @uses      internal/domain, context, testing, time.
// @reason    Both collaborators are optional in the composition root, and both
//
//	are wired in every other test in this package. A consumer that
//	panicked or spun on a nil bus, or that stopped consuming because the
//	ring was absent, would only show up in a deployment that wired one
//	half, which is the deployment these tests describe.
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

// TestUsageEventConsumer_NilBusIsInert asserts a deployment without a broker
// keeps working: Run returns immediately.
func TestUsageEventConsumer_NilBusIsInert(t *testing.T) {
	consumer := NewUsageEventConsumer(nil, &consoleLineSink{}, nil)
	done := make(chan struct{})
	go func() { defer close(done); consumer.Run(context.Background()) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run with a nil bus did not return")
	}
}

// TestUsageEventConsumer_NilWriterStillConsumes asserts a deployment with no
// console ring consumes the channel rather than failing on the first event.
func TestUsageEventConsumer_NilWriterStillConsumes(t *testing.T) {
	payload, err := domain.EncodeUsageEvent(publishedEventFixture())
	if err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	bus := &usageEventBusDouble{
		subscription: &usageEventSubscriptionDouble{payloads: [][]byte{payload}},
	}
	consumer := NewUsageEventConsumer(bus, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); consumer.Run(ctx) }()
	subscription := bus.subscription.(*usageEventSubscriptionDouble)
	waitFor(t, func() bool { return subscription.deliveredCount() == 1 })
	cancel()
	<-done
}
