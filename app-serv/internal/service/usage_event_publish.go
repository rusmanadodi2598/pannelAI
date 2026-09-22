// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_publish.go
// @for       The publisher half of the usage domain event: a bounded queue and
//
//	one drain goroutine, fed by the recorder's choke point.
//
// @uses      internal/domain, internal/repository, context, log/slog,
//
//	runtime/debug, sync/atomic, time.
//
// @reason    AGENTS.md §2.3 makes a mutation on an aggregate root emit a domain
//
//	event, and the event must not put a broker round trip on the request
//	path. The queue is what separates the two: Record enqueues and
//	returns, one goroutine publishes.
//
//	PUBLISH POLICY (AGENTS.md §1.6)
//
//	The queue is bounded. A full queue drops the newest event and counts
//	it, because the alternative is a request path that blocks on a
//	broker, and the durable record of the request is the usage row the
//	recorder already wrote. A publish failure is logged and counted, not
//	retried on a timer: Pub/Sub has no durable destination to retry
//	into, and a retry loop over a channel whose consumer is gone would
//	spin forever.
//
//	TERMINATION
//
//	Run returns when its context is cancelled, after draining what is
//	already queued. It is the only goroutine the publisher owns, and it
//	spawns nothing that outlives it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// usageEventQueueDepth bounds how many events may be waiting to publish. It is
// sized for a burst rather than a backlog: a broker that is slower than the
// data plane for longer than this many events is a broker that is down, and the
// drop policy is what answers that.
const usageEventQueueDepth = 256

// usageEventPublishTimeout bounds one broker call, so a hung broker degrades the
// publisher instead of stalling its goroutine forever (AGENTS.md §1.6).
const usageEventPublishTimeout = 3 * time.Second

// usageEventDrainTimeout bounds the whole drain on shutdown, so a broker that
// went away while events were queued cannot hold up process termination.
const usageEventDrainTimeout = 5 * time.Second

// UsageEventPublisher publishes the usage domain event on the recorder's
// behalf. A nil bus makes PublishEvent a no-op, which is the documented
// behaviour for a deployment that wired no broker: the data plane keeps
// serving and no event is emitted, rather than the process refusing to boot.
type UsageEventPublisher struct {
	bus    repository.UsageEventBus
	logger *slog.Logger

	queue chan []byte

	// dropped counts events the bounded queue refused, and failed counts
	// publishes the broker refused. Both are read by a test and by the log
	// line the policy writes, so a silent drop is not invisible.
	dropped atomic.Int64
	failed  atomic.Int64
}

// NewUsageEventPublisher builds the publisher over a bus. The bus is optional.
func NewUsageEventPublisher(bus repository.UsageEventBus, logger *slog.Logger) *UsageEventPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &UsageEventPublisher{
		bus:    bus,
		logger: logger,
		queue:  make(chan []byte, usageEventQueueDepth),
	}
}

// PublishEvent encodes the event and enqueues it. It never blocks and never
// returns an error: a caller on the request path has already answered the
// client, and a broker failure is accounting, not the client's problem (the
// rule every write in this service follows).
//
// An event the codec refuses is dropped with a log line rather than queued:
// the codec's rules are the aggregate's own, so a refusal here is a bug in the
// caller, and publishing an invalid payload would push that bug onto every
// consumer.
func (p *UsageEventPublisher) PublishEvent(event domain.UsageEvent) {
	if p == nil || p.bus == nil {
		return
	}
	payload, err := domain.EncodeUsageEvent(event)
	if err != nil {
		p.logger.Error("usage event refused by its own codec",
			"request_id", event.RequestID, "error", err)
		return
	}
	select {
	case p.queue <- payload:
	default:
		// The queue is full. Dropping the newest event rather than blocking is
		// the stated policy: the usage row is the durable record, and a request
		// must not wait on a broker.
		p.dropped.Add(1)
		p.logger.Warn("usage event queue is full; event dropped",
			"request_id", event.RequestID, "depth", usageEventQueueDepth)
	}
}

// Run publishes queued events until ctx is cancelled, then drains what is
// already queued and returns.
//
// The drain is bounded by usageEventDrainTimeout and runs on a context that is
// independent of the cancelled one, so shutdown empties the queue without a
// broker that went away being able to hold up termination. A publish runs under
// its own timeout and its own panic boundary, so neither a hung broker nor a
// panic in the transport ends the process (AGENTS.md §1.6).
func (p *UsageEventPublisher) Run(ctx context.Context) {
	if p == nil || p.bus == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			p.drain(ctx)
			return
		case payload := <-p.queue:
			p.publishOne(ctx, payload)
		}
	}
}

// drain publishes the events already queued at shutdown, under one bounded
// deadline. Events that do not fit in the window are counted as dropped rather
// than silently lost, and the loop never waits for a new arrival: the queue is
// closed for business the moment Run is leaving.
//
// The deadline is derived from context.WithoutCancel(ctx) rather than from ctx:
// Run only reaches here because ctx is already done, so a plain derivation would
// abort the drain immediately and lose the very events this function exists to
// flush. WithoutCancel keeps the context's values while dropping its
// cancellation, which is exactly the intent and is why the fresh root is not
// needed.
func (p *UsageEventPublisher) drain(ctx context.Context) {
	if ctx.Err() == nil {
		// Defensive: drain is the shutdown path, and a caller that reached it
		// without a cancelled context would be asking to flush mid-flight.
		return
	}
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), usageEventDrainTimeout)
	defer cancel()
	for {
		select {
		case payload := <-p.queue:
			p.publishOne(drainCtx, payload)
		default:
			return
		}
	}
}

// publishOne publishes one payload under a timeout and a panic boundary.
func (p *UsageEventPublisher) publishOne(ctx context.Context, payload []byte) {
	defer func() {
		if recovered := recover(); recovered != nil {
			p.failed.Add(1)
			p.logger.Error("panic while publishing a usage event",
				"panic", recovered, "stack", string(debug.Stack()))
		}
	}()
	callCtx, cancel := context.WithTimeout(ctx, usageEventPublishTimeout)
	defer cancel()
	if err := p.bus.Publish(callCtx, payload); err != nil {
		p.failed.Add(1)
		p.logger.Warn("publishing a usage event failed", "error", err)
	}
}

// Dropped reports how many events the bounded queue refused.
func (p *UsageEventPublisher) Dropped() int64 { return p.dropped.Load() }

// Failed reports how many publishes the broker refused.
func (p *UsageEventPublisher) Failed() int64 { return p.failed.Load() }
