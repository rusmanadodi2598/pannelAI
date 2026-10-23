// Package worker consumes queued work with bounded concurrency.
//
// @file      internal/worker/updates_test.go
// @for       The consumer loop contract: read, process, and stop on cancel.
// @uses      testing, context, errors, strings, time, redis, schema, worker.
// @reason    An entry acked before its reply is sent is lost for good, so the ordering of send, ack, and dead letter is pinned by tests rather than trusted to the loop.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-21
package worker_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	redisrepo "github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/worker"
)

// runOne consumes the single queued entry and returns once it reached a
// terminal state (acked or dead lettered), then stops the loop.
func runOne(t *testing.T, stream *fakeStream, sender *fakeSender, cfg worker.Config) {
	t.Helper()
	if cfg.Count == 0 {
		cfg.Count = 1
	}
	if len(cfg.RetryDelays) == 0 {
		cfg.RetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	}
	consumer := worker.NewConsumer(stream, sender, discardLogger(), cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx) }()

	<-stream.ready
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(stream.ackedIDs()) > 0 || len(stream.deadLetters()) > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after the context was cancelled")
	}
}

func TestConsumer_GivenStreamEntries_ThenSendAndAckFollowTheContract(t *testing.T) {
	cases := []struct {
		name         string
		entry        func(t *testing.T) redisrepo.Entry
		wantSend     int
		wantAck      int
		wantContains string
	}{
		{
			name: "start command is replied and acked",
			entry: func(t *testing.T) redisrepo.Entry {
				return redisrepo.Entry{ID: "1-0", Payload: payload(t, workItem("/start"))}
			},
			wantSend:     1,
			wantAck:      1,
			wantContains: "Halo, ini KENTANG TECH Media.",
		},
		{
			name: "plain message is replied and acked",
			entry: func(t *testing.T) redisrepo.Entry {
				return redisrepo.Entry{ID: "2-0", Payload: payload(t, workItem("halo"))}
			},
			wantSend:     1,
			wantAck:      1,
			wantContains: "Pesan diterima.",
		},
		{
			name: "callback is replied and acked",
			entry: func(t *testing.T) redisrepo.Entry {
				return redisrepo.Entry{ID: "3-0", Payload: payload(t, schema.WorkItem{UpdateID: 31, ChatID: 42, UserID: 42, Kind: schema.KindCallback, CallbackData: "draft:abc"})}
			},
			wantSend:     1,
			wantAck:      1,
			wantContains: "Tombol diterima.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := newFakeStream(tc.entry(t))
			sender := &fakeSender{}
			runOne(t, stream, sender, worker.Config{Stream: "s", Group: "g", Consumer: "c"})

			texts := sender.texts()
			if len(texts) != tc.wantSend {
				t.Fatalf("sends = %d, want %d", len(texts), tc.wantSend)
			}
			if !strings.Contains(texts[0], tc.wantContains) {
				t.Fatalf("reply = %q, want it to contain %q", texts[0], tc.wantContains)
			}
			if acked := stream.ackedIDs(); len(acked) != tc.wantAck {
				t.Fatalf("acks = %d, want %d", len(acked), tc.wantAck)
			}
			if letters := stream.deadLetters(); len(letters) != 0 {
				t.Fatalf("dead letters = %d, want 0", len(letters))
			}
		})
	}
}

func TestConsumer_GivenAckFailure_ThenTheLoopKeepsGoing(t *testing.T) {
	stream := newFakeStream(redisrepo.Entry{ID: "7-0", Payload: payload(t, workItem("halo"))})
	stream.ackErr = errors.New("redis down")
	sender := &fakeSender{}

	runOne(t, stream, sender, worker.Config{Stream: "s", Group: "g", Consumer: "c"})

	if got := len(sender.texts()); got != 1 {
		t.Fatalf("sends = %d, want 1", got)
	}
	if got := len(stream.ackedIDs()); got != 1 {
		t.Fatalf("ack attempts = %d, want 1", got)
	}
}

func TestConsumer_GivenBrokenGroup_ThenRunStopsWithTheError(t *testing.T) {
	stream := newFakeStream()
	stream.ensureErr = errors.New("redis down")

	err := worker.NewConsumer(stream, &fakeSender{}, discardLogger(), worker.Config{Stream: "s", Group: "g", Consumer: "c"}).Run(context.Background())
	if !errors.Is(err, stream.ensureErr) {
		t.Fatalf("Run() error = %v, want %v", err, stream.ensureErr)
	}
}
