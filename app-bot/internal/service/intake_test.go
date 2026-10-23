// Package service holds the use cases that are not HTTP shaped.
//
// @file      internal/service/intake_test.go
// @for       Duplicate suppression and enqueue behavior of the intake use case.
// @uses      testing, context, errors, time, schema.
// @reason    A dropped enqueue after a successful dedupe mark would lose a paid update forever, so the release path is tested, not assumed.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/service"
)

type fakeDeduper struct {
	newUpdate     bool
	markErr       error
	forgot        []int64
	forgetLiveCtx []bool
	forgetErr     error
}

func (f *fakeDeduper) Mark(context.Context, int64, time.Duration) (bool, error) {
	return f.newUpdate, f.markErr
}

func (f *fakeDeduper) Forget(ctx context.Context, updateID int64) error {
	f.forgot = append(f.forgot, updateID)
	f.forgetLiveCtx = append(f.forgetLiveCtx, ctx.Err() == nil)
	return f.forgetErr
}

type fakeQueue struct {
	enqueued []schema.WorkItem
	err      error
}

func (f *fakeQueue) Enqueue(_ context.Context, item schema.WorkItem) error {
	f.enqueued = append(f.enqueued, item)
	return f.err
}

func validItem() schema.WorkItem {
	return schema.WorkItem{
		UpdateID: 10,
		ChatID:   42,
		UserID:   42,
		Kind:     schema.KindMessage,
		Text:     "halo",
	}
}

func TestAccept_GivenDedupeAndQueueStates_ThenEnqueueHappensExactlyOnce(t *testing.T) {
	cases := []struct {
		name       string
		dedupe     *fakeDeduper
		queue      *fakeQueue
		wantQueued int
		wantForgot int
		wantErr    bool
	}{
		{name: "new update is enqueued", dedupe: &fakeDeduper{newUpdate: true}, queue: &fakeQueue{}, wantQueued: 1},
		{name: "duplicate is skipped", dedupe: &fakeDeduper{newUpdate: false}, queue: &fakeQueue{}, wantQueued: 0},
		{name: "dedupe failure is propagated", dedupe: &fakeDeduper{markErr: errors.New("redis down")}, queue: &fakeQueue{}, wantErr: true},
		{name: "enqueue failure releases the mark", dedupe: &fakeDeduper{newUpdate: true}, queue: &fakeQueue{err: errors.New("stream down")}, wantQueued: 1, wantForgot: 1, wantErr: true},
		{name: "enqueue and release failure still reports the enqueue error", dedupe: &fakeDeduper{newUpdate: true, forgetErr: errors.New("del down")}, queue: &fakeQueue{err: errors.New("stream down")}, wantQueued: 1, wantForgot: 1, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			intake := service.NewIntake(tc.dedupe, tc.queue, time.Hour)

			_, err := intake.Accept(context.Background(), validItem())

			if tc.wantErr && err == nil {
				t.Fatalf("Accept() error = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Accept() error = %v, want nil", err)
			}
			if len(tc.queue.enqueued) != tc.wantQueued {
				t.Fatalf("enqueued = %d, want %d", len(tc.queue.enqueued), tc.wantQueued)
			}
			if len(tc.dedupe.forgot) != tc.wantForgot {
				t.Fatalf("forgot = %d, want %d", len(tc.dedupe.forgot), tc.wantForgot)
			}
		})
	}
}

func TestAccept_GivenCancelledRequest_ThenTheMarkIsStillReleased(t *testing.T) {
	dedupe := &fakeDeduper{newUpdate: true}
	queue := &fakeQueue{err: errors.New("stream down")}
	intake := service.NewIntake(dedupe, queue, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := intake.Accept(ctx, validItem()); err == nil {
		t.Fatalf("Accept() error = nil, want the enqueue failure")
	}
	if len(dedupe.forgot) != 1 {
		t.Fatalf("forgot = %d, want 1", len(dedupe.forgot))
	}
	if !dedupe.forgetLiveCtx[0] {
		t.Fatal("Forget ran on a cancelled context, so the dedupe mark would outlive the failed enqueue and swallow the Telegram retry")
	}
}

func TestAccept_GivenInvalidItem_ThenNothingIsMarkedOrQueued(t *testing.T) {
	dedupe := &fakeDeduper{newUpdate: true}
	queue := &fakeQueue{}
	intake := service.NewIntake(dedupe, queue, time.Hour)

	item := validItem()
	item.ChatID = 0

	if _, err := intake.Accept(context.Background(), item); err == nil {
		t.Fatalf("Accept() error = nil, want a validation error")
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}
