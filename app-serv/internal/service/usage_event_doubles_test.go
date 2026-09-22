// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_doubles_test.go
// @for       The in-memory broker, subscription, and event fixture every usage
//
//	event test drives.
//
// @uses      internal/domain, internal/repository, context, sync, time.
// @reason    The publisher and the consumer are two halves of one seam, so their
//
//	tests share one double: a broker that records what was published and
//	hands it to whoever subscribed. Defining it twice would let the two
//	halves' tests disagree about what the seam does, which is the failure
//	this file exists to prevent. The subscription is mutex-guarded
//	because the consumer reads it from its own goroutine while a test
//	reads the delivered count.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// usageEventBusDouble is an in-memory broker: it records what was published and
// can be told to fail or to block.
type usageEventBusDouble struct {
	mu           sync.Mutex
	published    [][]byte
	publishFn    func(ctx context.Context, payload []byte) error
	subErr       error
	subscription repository.UsageEventSubscription
}

func (b *usageEventBusDouble) Publish(ctx context.Context, payload []byte) error {
	if b.publishFn != nil {
		return b.publishFn(ctx, payload)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.published = append(b.published, append([]byte(nil), payload...))
	return nil
}

func (b *usageEventBusDouble) Subscribe(context.Context) (repository.UsageEventSubscription, error) {
	if b.subErr != nil {
		return nil, b.subErr
	}
	if b.subscription != nil {
		return b.subscription, nil
	}
	return &usageEventSubscriptionDouble{}, nil
}

func (b *usageEventBusDouble) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.published)
}

func (b *usageEventBusDouble) payloads() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, len(b.published))
	copy(out, b.published)
	return out
}

// usageEventSubscriptionDouble delivers a fixed sequence and then reports
// idleness forever. It is mutex-guarded because the consumer reads it from its
// own goroutine while a test reads the delivered count: an unguarded counter is
// a data race the race detector reports, and the count is the evidence these
// tests assert on.
type usageEventSubscriptionDouble struct {
	mu        sync.Mutex
	delivered int
	payloads  [][]byte
	recvErr   error
	closed    bool
}

func (s *usageEventSubscriptionDouble) Receive(_ context.Context, _ time.Duration) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.delivered < len(s.payloads) {
		payload := s.payloads[s.delivered]
		s.delivered++
		return payload, nil
	}
	return nil, repository.ErrUsageEventTimeout
}

func (s *usageEventSubscriptionDouble) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// deliveredCount reports how many payloads the subscription handed over.
func (s *usageEventSubscriptionDouble) deliveredCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.delivered
}

// wasClosed reports whether Close ran.
func (s *usageEventSubscriptionDouble) wasClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// publishedEventFixture is the benign event the tables below vary.
func publishedEventFixture() domain.UsageEvent {
	return domain.UsageEvent{
		RequestID:   "req_1",
		ProviderID:  "openai",
		EndpointID:  "ep_1",
		Model:       "gpt-4o",
		TotalTokens: 7,
		CostUSD:     "0.01000000",
		Status:      domain.UsageStatusSuccess,
		OccurredAt:  time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

// consoleLineSink records the lines the consumer mirrors.
type consoleLineSink struct {
	mu    sync.Mutex
	lines []string
	err   error
}

func (s *consoleLineSink) AppendConsole(_ context.Context, line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.lines = append(s.lines, line)
	return nil
}

func (s *consoleLineSink) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.lines))
	copy(out, s.lines)
	return out
}

// failingSubscribeBus reports a subscription error a fixed number of times and
// then serves a subscription that delivers nothing.
type failingSubscribeBus struct {
	mu           sync.Mutex
	failures     int
	subscribeCh  chan struct{}
	subscription repository.UsageEventSubscription
}

func (b *failingSubscribeBus) Publish(context.Context, []byte) error { return nil }

func (b *failingSubscribeBus) Subscribe(context.Context) (repository.UsageEventSubscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subscribeCh != nil {
		select {
		case b.subscribeCh <- struct{}{}:
		default:
		}
	}
	if b.failures > 0 {
		b.failures--
		return nil, errors.New("broker is unreachable")
	}
	if b.subscription == nil {
		b.subscription = &usageEventSubscriptionDouble{}
	}
	return b.subscription, nil
}
