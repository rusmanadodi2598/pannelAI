// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_reconnect_test.go
// @for       Tests that the usage event consumer recovers from a failing
//
//	subscription instead of stopping.
//
// @uses      internal/domain, internal/repository, context, testing, time.
// @reason    The consumer opens a subscription per attempt, and a broker that is
//
//	down at boot is the ordinary case this loop exists for: without it a
//	process started before Redis would consume nothing for its whole
//	lifetime while reporting healthy. The test drives the failure at the
//	Subscribe call, which is where a down broker actually surfaces.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageEventConsumer_ReconnectsAfterASubscriptionFailure asserts the retry
// policy: a failing Subscribe is retried, and the consumer recovers without a
// new process.
func TestUsageEventConsumer_ReconnectsAfterASubscriptionFailure(t *testing.T) {
	cases := []struct {
		name     string
		failures int
	}{
		{name: "one failure then success", failures: 1},
		{name: "three failures then success", failures: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := domain.EncodeUsageEvent(publishedEventFixture())
			if err != nil {
				t.Fatalf("encoding the fixture: %v", err)
			}
			sink := &consoleLineSink{}
			bus := &failingSubscribeBus{
				failures:     tc.failures,
				subscription: &usageEventSubscriptionDouble{payloads: [][]byte{payload}},
			}
			consumer := NewUsageEventConsumer(bus, sink, nil)

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { defer close(done); consumer.Run(ctx) }()
			waitForLines(t, sink, 1)
			cancel()
			<-done

			if got := len(sink.recorded()); got != 1 {
				t.Fatalf("console lines = %d, want 1 after %d failures", got, tc.failures)
			}
		})
	}
}
