//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/usage_event_policy_test.go
// @for       Integration tests for the transport's stated policies against a
//
//	real server: a publish nobody hears, an empty payload, and a
//	double close.
//
// @uses      github.com/redis/go-redis/v9, internal/domain, context, testing.
// @reason    Each of these is a policy rather than a mechanism, and a policy is
//
//	exactly what an in-memory double would restate rather than verify:
//	that the real client reports success for a publish with zero
//	subscribers, and that closing a subscription twice is quiet. The
//	build tag matches the file these share a harness with.
//
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration ./internal/repository/redis/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package redisrepo

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageEventBus_PublishWithNoSubscriberIsNotAnError pins the documented
// policy: Pub/Sub is best effort, the durable record is the usage row, and a
// publish nobody is listening to is not a failure.
func TestUsageEventBus_PublishWithNoSubscriberIsNotAnError(t *testing.T) {
	bus, _ := newTestBus(t)
	payload, err := domain.EncodeUsageEvent(usageEventFixture())
	if err != nil {
		t.Fatalf("EncodeUsageEvent() = %v, want nil", err)
	}
	if err := bus.Publish(context.Background(), payload); err != nil {
		t.Fatalf("Publish() with no subscriber = %v, want nil", err)
	}
}

// TestUsageEventBus_RefusesAnEmptyPayload asserts the transport does not forward
// a payload with nothing in it, which no consumer could act on.
func TestUsageEventBus_RefusesAnEmptyPayload(t *testing.T) {
	bus, _ := newTestBus(t)
	if err := bus.Publish(context.Background(), nil); err == nil {
		t.Fatal("Publish(nil) = nil, want a refusal")
	}
}

// TestUsageEventBus_CloseIsIdempotent asserts the consumer can close a
// subscription twice on a shutdown path without reporting a failure.
func TestUsageEventBus_CloseIsIdempotent(t *testing.T) {
	bus, _ := newTestBus(t)
	subscription, err := bus.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe() = %v, want nil", err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatalf("first Close() = %v, want nil", err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatalf("second Close() = %v, want nil", err)
	}
}
