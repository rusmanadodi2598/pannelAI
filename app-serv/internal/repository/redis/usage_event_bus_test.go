//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/usage_event_bus_test.go
// @for       Integration tests for the Pub/Sub transport the usage event
//
//	crosses, against a real server.
//
// @uses      github.com/redis/go-redis/v9, internal/domain, internal/repository,
//
//	context, errors, os, testing, time.
//
// @reason    The bus's whole contract is broker behaviour: a publish that
//
//	reaches a subscriber, an idle channel that reports idleness rather
//	than a fault, and a subscription that survives many idle waits. None
//	of that can be shown against an in-memory double, and the idle case
//	in particular is the one a double cannot fail: the real question is
//	whether the connection survives a read deadline, which only a real
//	socket answers.
//
//	The file carries an `integration` build tag, so the default
//	`go test ./...` stays hermetic (AGENTS.md §2.1 forbids t.Skip as a way
//	to sidestep a test). With the tag active the address is required, not
//	optional: a missing value fails the test rather than passing
//	silently.
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
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// newTestBus connects to the configured server and returns a bus over it.
func newTestBus(t *testing.T) (*UsageEventBus, redis.UniversalClient) {
	t.Helper()

	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the usage event bus tests", testRedisEnv)
	}
	client := redis.NewClient(redisOptions(t, raw))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("closing redis client: %v", err)
		}
	})
	return NewUsageEventBus(client), client
}

// usageEventFixture is the benign event the tables below publish.
func usageEventFixture() domain.UsageEvent {
	return domain.UsageEvent{
		RequestID:   "req_live_1",
		ProviderID:  "openai",
		EndpointID:  "ep_live_1",
		Model:       "gpt-4o",
		TotalTokens: 7,
		CostUSD:     "0.01000000",
		Status:      domain.UsageStatusSuccess,
		OccurredAt:  time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

// TestUsageEventBus_PublishReachesASubscriber is the transport's core claim: an
// encoded event published by one caller is the event a subscriber reads back.
func TestUsageEventBus_PublishReachesASubscriber(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.UsageEvent)
	}{
		{name: "a complete event arrives intact"},
		{
			name: "an endpoint-less error event arrives intact",
			mutate: func(e *domain.UsageEvent) {
				e.RequestID = "req_live_2"
				e.EndpointID = ""
				e.Status = domain.UsageStatusError
				e.TotalTokens = 0
				e.CostUSD = "0.00000000"
			},
		},
		{
			name:   "a unicode model name arrives intact",
			mutate: func(e *domain.UsageEvent) { e.RequestID = "req_live_3"; e.Model = "モデル/φ-4o" },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus, _ := newTestBus(t)
			ctx := context.Background()
			subscription, err := bus.Subscribe(ctx)
			if err != nil {
				t.Fatalf("Subscribe() = %v, want nil", err)
			}
			defer func() {
				if err := subscription.Close(); err != nil {
					t.Errorf("closing the subscription: %v", err)
				}
			}()

			want := usageEventFixture()
			if tc.mutate != nil {
				tc.mutate(&want)
			}
			payload, err := domain.EncodeUsageEvent(want)
			if err != nil {
				t.Fatalf("EncodeUsageEvent() = %v, want nil", err)
			}
			if err := bus.Publish(ctx, payload); err != nil {
				t.Fatalf("Publish() = %v, want nil", err)
			}

			received := receiveOne(t, subscription)
			got, err := domain.DecodeUsageEvent(received)
			if err != nil {
				t.Fatalf("the received payload does not decode: %v", err)
			}
			if got != want {
				t.Fatalf("received event = %+v, want %+v", got, want)
			}
		})
	}
}

// TestUsageEventBus_IdleChannelReportsIdlenessNotAFault asserts the distinction
// the consumer's loop depends on: a quiet second is ErrUsageEventTimeout, and
// the subscription still delivers the next publish. Without this the consumer
// would treat every idle wait as a broken broker and reconnect once a second.
func TestUsageEventBus_IdleChannelReportsIdlenessNotAFault(t *testing.T) {
	bus, _ := newTestBus(t)
	ctx := context.Background()
	subscription, err := bus.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() = %v, want nil", err)
	}
	defer func() {
		if err := subscription.Close(); err != nil {
			t.Errorf("closing the subscription: %v", err)
		}
	}()

	// Several idle waits in a row, then a real delivery on the same
	// subscription: this is the exact sequence the consumer's loop runs.
	for attempt := 1; attempt <= 3; attempt++ {
		_, err := subscription.Receive(ctx, 100*time.Millisecond)
		if !errors.Is(err, repository.ErrUsageEventTimeout) {
			t.Fatalf("idle receive %d = %v, want ErrUsageEventTimeout", attempt, err)
		}
	}

	want := usageEventFixture()
	payload, err := domain.EncodeUsageEvent(want)
	if err != nil {
		t.Fatalf("EncodeUsageEvent() = %v, want nil", err)
	}
	if err := bus.Publish(ctx, payload); err != nil {
		t.Fatalf("Publish() = %v, want nil", err)
	}
	got, err := domain.DecodeUsageEvent(receiveOne(t, subscription))
	if err != nil {
		t.Fatalf("the received payload does not decode: %v", err)
	}
	if got != want {
		t.Fatalf("received event after idle waits = %+v, want %+v", got, want)
	}
}

// receiveOne reads one payload, failing the test rather than hanging.
func receiveOne(t *testing.T, subscription repository.UsageEventSubscription) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		payload, err := subscription.Receive(context.Background(), 200*time.Millisecond)
		if err == nil {
			return payload
		}
		if !errors.Is(err, repository.ErrUsageEventTimeout) {
			t.Fatalf("Receive() = %v, want a payload", err)
		}
	}
	t.Fatal("no usage event arrived within 5s")
	return nil
}
