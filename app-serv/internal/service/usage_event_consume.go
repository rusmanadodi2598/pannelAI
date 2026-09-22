// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_consume.go
// @for       The subscriber half of the usage domain event: the production
//
//	consumer that mirrors every recorded request into the console ring.
//
// @uses      internal/domain, internal/repository, context, log/slog,
//
//	runtime/debug, time.
//
// @reason    AGENTS.md §2.3 makes an event real only when something consumes it,
//
//	and the console ring is the one surface in this process that is
//	already a stream of lines an operator reads: the gateway's own record
//	of what it served now reaches it through the event instead of a
//	second write at the accounting site. That is what makes the seam
//	observable rather than decorative — if the publisher stops, the
//	console stops growing, and the panel shows it.
//
//	The consumer deliberately writes nothing durable. The usage row is
//	the durable record; a subscriber that wrote a second copy would be a
//	second source of truth for the same request.
//
//	RETRY POLICY (AGENTS.md §1.6)
//
//	A receive that times out is idleness, not failure, and costs nothing.
//	A transport failure backs off exponentially from 250ms to 30s, with
//	the attempt count reset by the first successful receive; there is no
//	dead letter, because the subscription is the only state and losing it
//	means the next attempt rebuilds it.
//
//	TERMINATION
//
//	Run returns when its context is cancelled, closing its subscription
//	on the way out. It owns exactly one goroutine and no others.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// The consumer's stated retry policy: a short first wait, doubling to a ceiling
// that keeps a long outage cheap to wait through.
const (
	usageEventBackoffBase    = 250 * time.Millisecond
	usageEventBackoffCeiling = 30 * time.Second
	// usageEventIdleWait is how long one receive blocks before reporting
	// idleness. It bounds how quickly a cancelled context is noticed, so
	// shutdown does not wait on a quiet channel.
	usageEventIdleWait = time.Second
)

// ConsoleLineWriter is the sink the consumer writes rendered lines to.
// LogService implements it, so the ring's bound and the settings read stay in
// one place and this consumer only decides what one line says.
type ConsoleLineWriter interface {
	AppendConsole(ctx context.Context, line string) error
}

// UsageEventConsumer drains the usage domain event channel and mirrors each
// event into the console ring. A nil bus makes Run return immediately, which is
// the documented behaviour for a deployment that wired no broker.
type UsageEventConsumer struct {
	bus    repository.UsageEventBus
	writer ConsoleLineWriter
	logger *slog.Logger
}

// NewUsageEventConsumer builds the consumer over a bus and a line sink.
func NewUsageEventConsumer(bus repository.UsageEventBus, writer ConsoleLineWriter, logger *slog.Logger) *UsageEventConsumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &UsageEventConsumer{bus: bus, writer: writer, logger: logger}
}

// Run consumes until ctx is cancelled, which is the consumer's explicit
// termination condition (AGENTS.md §1.6).
//
// A subscription is opened per attempt rather than once outside the loop: a
// transport failure leaves the old subscription unusable, and rebuilding it is
// the recovery. The backoff is what keeps a broker that is down from turning
// into a hot loop.
func (c *UsageEventConsumer) Run(ctx context.Context) {
	if c == nil || c.bus == nil {
		return
	}
	attempt := 0
	for ctx.Err() == nil {
		subscription, err := c.bus.Subscribe(ctx)
		if err != nil {
			attempt++
			c.logger.Warn("subscribing to usage events failed",
				"attempt", attempt, "backoff_ms", c.backoffDelay(attempt).Milliseconds(), "error", err)
			if !sleepCtx(ctx, c.backoffDelay(attempt)) {
				return
			}
			continue
		}
		attempt = 0
		c.consume(ctx, subscription)
	}
}

// consume drains one subscription until it fails or the context is cancelled,
// then closes it. It is separate from Run so the reconnect policy and the read
// loop are each readable on their own.
func (c *UsageEventConsumer) consume(ctx context.Context, subscription repository.UsageEventSubscription) {
	defer func() {
		if recovered := recover(); recovered != nil {
			c.logger.Error("panic while consuming a usage event",
				"panic", recovered, "stack", string(debug.Stack()))
		}
		if err := subscription.Close(); err != nil {
			c.logger.Warn("closing the usage event subscription failed", "error", err)
		}
	}()

	for ctx.Err() == nil {
		payload, err := subscription.Receive(ctx, usageEventIdleWait)
		if err != nil {
			if errors.Is(err, repository.ErrUsageEventTimeout) || errors.Is(err, context.DeadlineExceeded) {
				// Idleness. Nothing to report and nothing to recover from.
				continue
			}
			if errors.Is(err, context.Canceled) {
				return
			}
			c.logger.Warn("receiving a usage event failed", "error", err)
			return
		}
		c.handle(ctx, payload)
	}
}

// handle decodes one payload and mirrors it into the console ring.
//
// A payload the codec refuses is logged and skipped, never fatal: one malformed
// message must not take the consumer down for every later event, and the
// refusal is the codec's guarantee that no consumer acts on a payload the
// aggregate would have rejected (OWASP A08).
func (c *UsageEventConsumer) handle(ctx context.Context, payload []byte) {
	event, err := domain.DecodeUsageEvent(payload)
	if err != nil {
		c.logger.Warn("dropping a usage event that failed to decode", "error", err)
		return
	}
	if c.writer == nil {
		return
	}
	if err := c.writer.AppendConsole(ctx, event.SummaryLine()); err != nil {
		// The ring is a view, not the record: a failed append is logged and the
		// next event is still consumed.
		c.logger.Warn("mirroring a usage event into the console failed",
			"request_id", event.RequestID, "error", err)
	}
}

// backoffDelay returns the wait before the next attempt: the base doubled once
// per attempt already spent, capped. There is no jitter because there is one
// consumer per process and the wait is bounded by the ceiling.
func (c *UsageEventConsumer) backoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := usageEventBackoffBase
	for step := 1; step < attempt; step++ {
		if delay >= usageEventBackoffCeiling/2 {
			return usageEventBackoffCeiling
		}
		delay *= 2
	}
	if delay > usageEventBackoffCeiling {
		return usageEventBackoffCeiling
	}
	return delay
}

// sleepCtx waits for the delay or until ctx is cancelled, reporting whether the
// wait completed. It is what keeps a shutdown prompt while a backoff is in
// progress.
func sleepCtx(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
