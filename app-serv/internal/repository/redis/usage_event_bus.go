// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/usage_event_bus.go
// @for       The Redis Pub/Sub transport the usage domain event crosses.
// @uses      github.com/redis/go-redis/v9, internal/repository, context, errors,
//
//	time.
//
// @reason    AGENTS.md §2.3 names Redis Pub/Sub as the asynchronous transport
//
//	for cross-domain events, and SPEC-API-001 §6 already makes Redis a
//	required dependency of this process. Pub/Sub is the right primitive
//	here rather than a list or stream: the event is a notification that
//	something happened, the durable record of the request is the usage
//	row, and a consumer that was down should not replay history it never
//	subscribed to. A stream would add retention and consumer-group state
//	to solve a problem this event does not have.
//
//	The channel name is a constant, so a publisher and a subscriber cannot
//	disagree about the topic by typo.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package redisrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// usageEventChannel is the Pub/Sub channel the usage domain event travels on.
// It is namespaced like every other key this process owns, so it cannot
// collide with an application that shares the Redis instance.
const usageEventChannel = "pannelai:events:usage.recorded"

// usageEventReceiveTimeout bounds one receive when the caller asks for no wait,
// so a consumer that passes a zero wait still returns rather than blocking a
// shutdown indefinitely.
const usageEventReceiveTimeout = 5 * time.Second

// UsageEventBus is the Redis Pub/Sub implementation of
// repository.UsageEventBus.
type UsageEventBus struct {
	client redis.UniversalClient
}

// NewUsageEventBus constructs the bus over an existing client.
func NewUsageEventBus(client redis.UniversalClient) *UsageEventBus {
	return &UsageEventBus{client: client}
}

// Publish hands one encoded event to the channel.
//
// A publish that reaches no subscriber is not an error and not a silent
// failure: Pub/Sub reports the subscriber count, and zero is the honest answer
// for a deployment that wired a publisher and no consumer. The count is
// deliberately not returned, because a caller that could branch on it would be
// tempted to treat "nobody is listening" as a reason not to record a request,
// and the usage row is written regardless.
func (b *UsageEventBus) Publish(ctx context.Context, payload []byte) error {
	if len(payload) == 0 {
		return fmt.Errorf("publishing %s: empty payload", usageEventChannel)
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	if err := b.client.Publish(callCtx, usageEventChannel, payload).Err(); err != nil {
		return fmt.Errorf("publishing %s: %w", usageEventChannel, err)
	}
	return nil
}

// Subscribe opens one subscription to the channel. go-redis resubscribes on a
// dropped connection internally, so a consumer only has to handle the errors
// Receive reports.
func (b *UsageEventBus) Subscribe(ctx context.Context) (repository.UsageEventSubscription, error) {
	subscription := b.client.Subscribe(ctx, usageEventChannel)
	// Subscribe is lazy: without this the first Receive is where a broker that
	// is down would surface, and the caller would get a working-looking
	// subscription that never delivers. Failing here instead keeps the failure
	// at the call that asked for the subscription.
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	if _, err := subscription.Receive(callCtx); err != nil {
		_ = subscription.Close()
		return nil, fmt.Errorf("subscribing to %s: %w", usageEventChannel, err)
	}
	return &usageEventSubscription{subscription: subscription}, nil
}

// usageEventSubscription is one consumer's subscription.
type usageEventSubscription struct {
	subscription *redis.PubSub
}

// Receive returns the next payload, waiting at most wait.
//
// The wait is a read deadline rather than a context timeout, which is what
// keeps an idle channel from looking like a broken connection: the connection
// survives a read that timed out, so a consumer that polls once per second
// keeps one subscription instead of reconnecting every second. Only a
// transport-level error is reported as one.
func (s *usageEventSubscription) Receive(ctx context.Context, wait time.Duration) ([]byte, error) {
	if wait <= 0 {
		wait = usageEventReceiveTimeout
	}
	message, err := s.subscription.ReceiveTimeout(ctx, wait)
	if err != nil {
		if isReceiveTimeout(err) {
			return nil, repository.ErrUsageEventTimeout
		}
		return nil, fmt.Errorf("receiving %s: %w", usageEventChannel, err)
	}
	switch value := message.(type) {
	case *redis.Message:
		return []byte(value.Payload), nil
	case *redis.Subscription, *redis.Pong:
		// Subscription confirmations and health-check pongs are not events.
		// Reporting them as an idle wait is the honest answer: nothing to
		// deliver arrived.
		return nil, repository.ErrUsageEventTimeout
	default:
		// reason: go-redis returns one of the three shapes above; anything else
		// is a library change this consumer has to be told about rather than
		// silently treat as an event.
		return nil, fmt.Errorf("receiving %s: unexpected message type %T", usageEventChannel, message)
	}
}

// Close releases the subscription. Closing an already-closed subscription is
// reported by go-redis as ErrClosed, which is the state the caller asked for,
// so it is not surfaced as a failure.
func (s *usageEventSubscription) Close() error {
	err := s.subscription.Close()
	if err == nil || errors.Is(err, redis.ErrClosed) {
		return nil
	}
	return err
}

// isReceiveTimeout reports whether the error is a read deadline rather than a
// transport fault. The check is on the interface rather than the concrete
// net.Error because go-redis wraps the socket error on its way out, and a
// wrapped timeout must still be recognised as idleness: treating it as a fault
// would make every quiet second look like a broken broker.
func isReceiveTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) {
		return timeout.Timeout()
	}
	return false
}
