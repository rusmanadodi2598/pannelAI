// Command app-serv wires the usage domain event between its publisher and its
// subscriber.
//
// @file      cmd/app-serv/usage_event_wiring_doubles_test.go
// @for       The in-memory broker, subscription, and stores the wiring test
//
//	drives.
//
// @uses      internal/domain, internal/repository, context, sync, time.
// @reason    The wiring test follows one recorded request to the console line it
//
//	produced, which needs a broker shared by both halves. Keeping the
//	doubles here leaves the test itself readable as the scenario it
//	describes: record a request, then assert the line.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"context"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// wiringBus is an in-memory broker shared by the publisher and the consumer
// under test, so a recorded request can be followed to the console line it
// produced without a live Redis.
type wiringBus struct {
	mu          sync.Mutex
	published   [][]byte
	subscribers []*wiringSubscription
	deliver     chan []byte
}

func newWiringBus() *wiringBus {
	return &wiringBus{deliver: make(chan []byte, 64)}
}

func (b *wiringBus) Publish(_ context.Context, payload []byte) error {
	b.mu.Lock()
	b.published = append(b.published, append([]byte(nil), payload...))
	subscribers := append([]*wiringSubscription(nil), b.subscribers...)
	b.mu.Unlock()
	for _, subscription := range subscribers {
		subscription.push(payload)
	}
	return nil
}

func (b *wiringBus) Subscribe(context.Context) (repository.UsageEventSubscription, error) {
	subscription := &wiringSubscription{deliver: make(chan []byte, 64)}
	b.mu.Lock()
	b.subscribers = append(b.subscribers, subscription)
	b.mu.Unlock()
	return subscription, nil
}

func (b *wiringBus) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.published)
}

// subscriberCount reports how many subscriptions have been opened, so a test can
// wait for the consumer to be listening before it publishes: Pub/Sub does not
// replay, so a publish that beats the subscription is simply not delivered.
func (b *wiringBus) subscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subscribers)
}

// wiringSubscription is one consumer's inbox.
type wiringSubscription struct {
	mu      sync.Mutex
	deliver chan []byte
	closed  bool
}

func (s *wiringSubscription) push(payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.deliver <- append([]byte(nil), payload...):
	default:
	}
}

func (s *wiringSubscription) Receive(ctx context.Context, wait time.Duration) ([]byte, error) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case payload := <-s.deliver:
		return payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, repository.ErrUsageEventTimeout
	}
}

func (s *wiringSubscription) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
