// Package worker consumes queued work with bounded concurrency.
//
// @file      internal/worker/delivery_test.go
// @for       The retry, dead letter, and payload rejection rules of one entry.
// @uses      testing, context, errors, time, redis, schema, worker.
// @reason    A reply that fails to send must be retried and then preserved, never dropped, so each branch of that decision is pinned here.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-22
package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	redisrepo "github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/worker"
)

// runUntil consumes the queued entry, waits for settled, then stops the loop.
func runUntil(t *testing.T, stream *fakeStream, sender *fakeSender, cfg worker.Config, settled func() bool) {
	t.Helper()
	if cfg.Count == 0 {
		cfg.Count = 1
	}
	consumer := worker.NewConsumer(stream, sender, discardLogger(), cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx) }()

	<-stream.ready
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !settled() {
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

func TestConsumer_GivenSendFailures_ThenRetryAndDeadLetterAreExact(t *testing.T) {
	transient := errors.New("telegram 500")
	fastRetries := []time.Duration{time.Millisecond, time.Millisecond}

	cases := []struct {
		name        string
		payload     func(t *testing.T) []byte
		errs        []error
		dlqErr      error
		wantSend    int
		wantDLQ     int
		wantAck     int
		wantReason  string
		settledFunc func(*fakeStream, *fakeSender) bool
	}{
		{
			name:        "retry succeeds on the second attempt",
			payload:     func(t *testing.T) []byte { return payload(t, workItem("halo")) },
			errs:        []error{transient},
			wantSend:    2,
			wantDLQ:     0,
			wantAck:     1,
			wantReason:  "",
			settledFunc: nil,
		},
		{
			name:        "attempts exhausted sends the payload to the dead letter stream",
			payload:     func(t *testing.T) []byte { return payload(t, workItem("halo")) },
			errs:        []error{transient, transient, transient},
			wantSend:    3,
			wantDLQ:     1,
			wantAck:     1,
			wantReason:  "send",
			settledFunc: nil,
		},
		{
			name:        "permanent refusal is not retried",
			payload:     func(t *testing.T) []byte { return payload(t, workItem("halo")) },
			errs:        []error{permanentErr{}},
			wantSend:    1,
			wantDLQ:     1,
			wantAck:     1,
			wantReason:  "send",
			settledFunc: nil,
		},
		{
			name:        "poison payload is dead lettered without a send",
			payload:     func(t *testing.T) []byte { return []byte("{not json") },
			wantSend:    0,
			wantDLQ:     1,
			wantAck:     1,
			wantReason:  "decode",
			settledFunc: nil,
		},
		{
			name: "unknown field is rejected by the strict decoder",
			payload: func(t *testing.T) []byte {
				return []byte(`{"update_id":9,"chat_id":42,"user_id":42,"kind":"message","text":"halo","surprise":true}`)
			},
			wantSend:    0,
			wantDLQ:     1,
			wantAck:     1,
			wantReason:  "decode",
			settledFunc: nil,
		},
		{
			name: "invalid item is dead lettered",
			payload: func(t *testing.T) []byte {
				return payload(t, schema.WorkItem{UpdateID: 32, ChatID: 42, UserID: 42, Kind: schema.KindMessage})
			},
			wantSend:    0,
			wantDLQ:     1,
			wantAck:     1,
			wantReason:  "invalid",
			settledFunc: nil,
		},
		{
			name:        "dead letter failure leaves the entry unacked",
			payload:     func(t *testing.T) []byte { return []byte("{not json") },
			dlqErr:      errors.New("redis down"),
			wantSend:    0,
			wantDLQ:     0,
			wantAck:     0,
			wantReason:  "",
			settledFunc: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := newFakeStream(redisrepo.Entry{ID: "8-0", Payload: tc.payload(t)})
			stream.dlqErr = tc.dlqErr
			sender := &fakeSender{errs: tc.errs}
			cfg := worker.Config{Stream: "s", Group: "g", Consumer: "c", RetryDelays: fastRetries}

			settled := func() bool {
				if tc.wantAck > 0 {
					return len(stream.ackedIDs()) >= tc.wantAck
				}
				return len(sender.texts()) >= 1 && len(stream.deadLetters()) == 0
			}
			if tc.dlqErr != nil {
				settled = func() bool { return stream.deadLetterAttempts() >= 1 }
			}
			runUntil(t, stream, sender, cfg, settled)

			if got := len(sender.texts()); got != tc.wantSend {
				t.Fatalf("sends = %d, want %d", got, tc.wantSend)
			}
			letters := stream.deadLetters()
			if len(letters) != tc.wantDLQ {
				t.Fatalf("dead letters = %d, want %d", len(letters), tc.wantDLQ)
			}
			if tc.wantDLQ > 0 && letters[0].reason != tc.wantReason {
				t.Fatalf("dead letter reason = %q, want %q", letters[0].reason, tc.wantReason)
			}
			if got := len(stream.ackedIDs()); got != tc.wantAck {
				t.Fatalf("acks = %d, want %d", got, tc.wantAck)
			}
		})
	}
}
