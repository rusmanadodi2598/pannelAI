// Package worker consumes queued work with bounded concurrency.
//
// @file      internal/worker/updates_test.go
// @for       The acknowledgement rules of the update stream consumer.
// @uses      testing, context, encoding/json, errors, io, log/slog, sync, time, redis, schema, logx.
// @reason    An entry that is acked before its reply is sent is lost for good, so the ordering of send, ack, and drop is pinned by tests rather than trusted to the loop.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-21
package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/logx"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/worker"
)

type fakeStream struct {
	mu        sync.Mutex
	entries   []redisrepo.Entry
	delivered bool
	acked     []string
	ensureErr error
	ackErr    error
	ready     chan struct{}
}

func newFakeStream(entries ...redisrepo.Entry) *fakeStream {
	return &fakeStream{entries: entries, ready: make(chan struct{})}
}

func (f *fakeStream) EnsureGroup(context.Context, string, string) error { return f.ensureErr }

func (f *fakeStream) ReadUpdates(ctx context.Context, _, _, _ string, _ int64, _ time.Duration) ([]redisrepo.Entry, error) {
	f.mu.Lock()
	if !f.delivered {
		f.delivered = true
		entries := f.entries
		f.mu.Unlock()
		close(f.ready)
		return entries, nil
	}
	f.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeStream) AckUpdate(_ context.Context, _, _, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acked = append(f.acked, id)
	return f.ackErr
}

func (f *fakeStream) ackedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.acked...)
}

type fakeSender struct {
	mu   sync.Mutex
	sent []string
	err  error
}

func (f *fakeSender) SendMessage(_ context.Context, _ int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return f.err
}

func (f *fakeSender) texts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent...)
}

func payload(t *testing.T, item schema.WorkItem) []byte {
	t.Helper()
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal work item: %v", err)
	}
	return raw
}

func workItem(text string) schema.WorkItem {
	return schema.WorkItem{UpdateID: 30, ChatID: 42, UserID: 42, Kind: schema.KindMessage, Text: text}
}

func TestConsumer_GivenStreamEntries_ThenSendAndAckFollowTheContract(t *testing.T) {
	cases := []struct {
		name         string
		entry        func(t *testing.T) redisrepo.Entry
		sendErr      error
		ackErr       error
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
		{
			name:     "poison payload is dropped without a send",
			entry:    func(t *testing.T) redisrepo.Entry { return redisrepo.Entry{ID: "4-0", Payload: []byte("{not json")} },
			wantSend: 0,
			wantAck:  0,
		},
		{
			name: "invalid item is dropped without a send",
			entry: func(t *testing.T) redisrepo.Entry {
				return redisrepo.Entry{ID: "5-0", Payload: payload(t, schema.WorkItem{UpdateID: 32, ChatID: 42, UserID: 42, Kind: schema.KindMessage})}
			},
			wantSend: 0,
			wantAck:  0,
		},
		{
			name: "send failure leaves the entry unacked for redelivery",
			entry: func(t *testing.T) redisrepo.Entry {
				return redisrepo.Entry{ID: "6-0", Payload: payload(t, workItem("halo"))}
			},
			sendErr:  errors.New("telegram 500"),
			wantSend: 1,
			wantAck:  0,
		},
		{
			name: "ack failure does not stop the consumer",
			entry: func(t *testing.T) redisrepo.Entry {
				return redisrepo.Entry{ID: "7-0", Payload: payload(t, workItem("halo"))}
			},
			ackErr:   errors.New("redis down"),
			wantSend: 1,
			wantAck:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := newFakeStream(tc.entry(t))
			sender := &fakeSender{err: tc.sendErr}
			if tc.ackErr != nil {
				stream.ackErr = tc.ackErr
			}
			consumer := worker.NewConsumer(stream, sender, discardLogger(), worker.Config{Stream: "s", Group: "g", Consumer: "c", Count: 1})

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- consumer.Run(ctx) }()

			<-stream.ready
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if len(sender.texts()) >= tc.wantSend && len(stream.ackedIDs()) >= tc.wantAck {
					break
				}
				time.Sleep(5 * time.Millisecond)
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

			texts := sender.texts()
			if len(texts) != tc.wantSend {
				t.Fatalf("sends = %d, want %d", len(texts), tc.wantSend)
			}
			if tc.wantContains != "" && !strings.Contains(texts[0], tc.wantContains) {
				t.Fatalf("reply = %q, want it to contain %q", texts[0], tc.wantContains)
			}
			if acked := stream.ackedIDs(); len(acked) != tc.wantAck {
				t.Fatalf("acks = %d, want %d", len(acked), tc.wantAck)
			}
		})
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

func discardLogger() *slog.Logger {
	return logx.New("error", "test", io.Discard)
}
