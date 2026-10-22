// Package service holds the use cases that are not HTTP shaped.
//
// @file      internal/service/intake.go
// @for       Duplicate suppression and enqueue for one Telegram update.
// @uses      context, fmt, time, internal/schema.
// @reason    Telegram retries a slow webhook, so the accept path must mark the update before queueing it and release that mark when the queue write fails, otherwise a retry is dropped as a duplicate.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

// Deduper is the Redis duplicate guard.
type Deduper interface {
	Mark(ctx context.Context, updateID int64, ttl time.Duration) (bool, error)
	Forget(ctx context.Context, updateID int64) error
}

// Queue is the Redis stream producer.
type Queue interface {
	Enqueue(ctx context.Context, item schema.WorkItem) error
}

// Intake accepts one validated work item.
type Intake struct {
	dedupe Deduper
	queue  Queue
	ttl    time.Duration
}

// NewIntake wires the guard and the queue with the duplicate window.
func NewIntake(dedupe Deduper, queue Queue, ttl time.Duration) *Intake {
	return &Intake{dedupe: dedupe, queue: queue, ttl: ttl}
}

// Accept returns true when the item reached the queue, false when the update
// was already seen. A queue failure releases the mark and returns the error.
func (s *Intake) Accept(ctx context.Context, item schema.WorkItem) (bool, error) {
	if err := item.Validate(); err != nil {
		return false, err
	}
	fresh, err := s.dedupe.Mark(ctx, item.UpdateID, s.ttl)
	if err != nil {
		return false, fmt.Errorf("marking update %d: %w", item.UpdateID, err)
	}
	if !fresh {
		return false, nil
	}
	if err := s.queue.Enqueue(ctx, item); err != nil {
		if forgetErr := s.dedupe.Forget(ctx, item.UpdateID); forgetErr != nil {
			return false, fmt.Errorf("enqueueing update %d: %w (releasing the mark: %v)", item.UpdateID, err, forgetErr)
		}
		return false, fmt.Errorf("enqueueing update %d: %w", item.UpdateID, err)
	}
	return true, nil
}
