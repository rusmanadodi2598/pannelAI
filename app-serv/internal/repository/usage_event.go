// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/usage_event.go
// @for       The broker seam a domain event crosses: publish, and a
//
//	subscription a consumer drains.
//
// @uses      context, errors, time.
// @reason    AGENTS.md §2.3 makes cross-domain stateful communication an
//
//	asynchronous event, and the transport is a broker the service layer
//	must not name (AGENTS.md §1.5 keeps Redis out of `service`). The
//	port is deliberately two methods rather than a general message bus:
//	app-serv publishes one event on one channel, and a seam that could
//	carry anything would let a second event type arrive without anyone
//	deciding where its consumer lives.
//
//	Payloads are `[]byte` rather than a domain type on purpose: the codec
//	that turns an event into its wire form belongs beside the aggregate
//	that defines it (internal/domain), so the broker stays a transport
//	and the contract stays in the domain.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package repository

import (
	"context"
	"errors"
	"time"
)

// ErrUsageEventTimeout reports that a subscription had nothing to deliver
// within the wait it was given. It is a sentinel rather than a nil payload so a
// consumer can tell an idle channel from a delivery, and a transport that
// signals idleness some other way can still answer in this vocabulary.
var ErrUsageEventTimeout = errors.New("no usage event arrived within the wait")

// UsageEventBus is the publish/subscribe transport for the usage domain event
// (AGENTS.md §2.3). The channel is bound at construction, so neither method
// takes a topic and a caller cannot publish to a channel no consumer reads.
type UsageEventBus interface {
	// Publish hands one encoded event to the broker. A publish with no
	// subscriber is not an error: pub/sub delivery is best effort by design,
	// and the durable record of a request is the usage row, not the event.
	Publish(ctx context.Context, payload []byte) error

	// Subscribe opens one subscription. The caller owns it and must Close it;
	// a subscription that is never closed holds a broker connection open.
	Subscribe(ctx context.Context) (UsageEventSubscription, error)
}

// UsageEventSubscription is one consumer's view of the channel.
type UsageEventSubscription interface {
	// Receive returns the next payload, waiting at most wait. A wait that
	// elapses with nothing to deliver reports ErrUsageEventTimeout, so a
	// consumer can poll for its own shutdown without treating silence as a
	// failure. Any other error is a transport fault.
	Receive(ctx context.Context, wait time.Duration) ([]byte, error)

	// Close releases the subscription. It is safe to call more than once.
	Close() error
}
