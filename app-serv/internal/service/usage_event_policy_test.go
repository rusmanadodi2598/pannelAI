// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_policy_test.go
// @for       Tests for the retry policy the usage event consumer states.
//
// @uses      testing, time.
// @reason    AGENTS.md §1.6 forbids an unstated retry policy, so the consumer
//
//	declares its backoff bounds as constants. A constant is easy to
//	change by accident and impossible to notice in review, so the shape
//	the policy promises (grows, never shrinks, never crosses its
//	ceiling, starts at its base) is pinned here rather than described in
//	a comment.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-22
package service

import (
	"testing"
	"time"
)

// TestUsageEventConsumer_BackoffIsBoundedAndMonotonic pins the stated retry
// policy: the delay grows, never shrinks, and never crosses its ceiling.
func TestUsageEventConsumer_BackoffIsBoundedAndMonotonic(t *testing.T) {
	consumer := NewUsageEventConsumer(&usageEventBusDouble{}, &consoleLineSink{}, nil)
	previous := time.Duration(0)
	for attempt := 1; attempt <= 20; attempt++ {
		delay := consumer.backoffDelay(attempt)
		if delay < previous {
			t.Fatalf("backoffDelay(%d) = %s, below the previous %s", attempt, delay, previous)
		}
		if delay > usageEventBackoffCeiling {
			t.Fatalf("backoffDelay(%d) = %s, above the ceiling %s", attempt, delay, usageEventBackoffCeiling)
		}
		previous = delay
	}
	if got := consumer.backoffDelay(1); got != usageEventBackoffBase {
		t.Fatalf("backoffDelay(1) = %s, want the base %s", got, usageEventBackoffBase)
	}
	if got := consumer.backoffDelay(0); got != usageEventBackoffBase {
		t.Fatalf("backoffDelay(0) = %s, want the base %s", got, usageEventBackoffBase)
	}
}
