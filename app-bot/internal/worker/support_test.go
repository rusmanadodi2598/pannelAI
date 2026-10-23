// Package worker consumes queued work with bounded concurrency.
//
// @file      internal/worker/support_test.go
// @for       The fake stream, fake sender, and item helpers shared by the worker tests.
// @uses      testing, context, encoding/json, io, log/slog, sync, time, logx, redis, schema.
// @reason    Two test files drive the same consumer, so the doubles live in one place and the expectations stay readable in each suite.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-22
package worker_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/logx"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

type dlqRecord struct {
	payload string
	reason  string
}

type fakeStream struct {
	mu        sync.Mutex
	entries   []redisrepo.Entry
	delivered bool
	acked     []string
	dlq       []dlqRecord
	dlqCalls  int
	ensureErr error
	ackErr    error
	dlqErr    error
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

func (f *fakeStream) EnqueueDLQ(_ context.Context, payload []byte, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dlqCalls++
	if f.dlqErr != nil {
		return f.dlqErr
	}
	f.dlq = append(f.dlq, dlqRecord{payload: string(payload), reason: reason})
	return nil
}

func (f *fakeStream) ackedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.acked...)
}

func (f *fakeStream) deadLetters() []dlqRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]dlqRecord(nil), f.dlq...)
}

func (f *fakeStream) deadLetterAttempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dlqCalls
}

// fakeSender fails with errs[i] on call i and succeeds once the list is spent.
type fakeSender struct {
	mu   sync.Mutex
	sent []string
	errs []error
}

func (f *fakeSender) SendMessage(_ context.Context, _ int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := len(f.sent)
	f.sent = append(f.sent, text)
	if index < len(f.errs) {
		return f.errs[index]
	}
	return nil
}

func (f *fakeSender) texts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent...)
}

// permanentErr is a failure the worker must not retry.
type permanentErr struct{}

func (permanentErr) Error() string   { return "blocked by user" }
func (permanentErr) Permanent() bool { return true }

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

func discardLogger() *slog.Logger {
	return logx.New("error", "test", io.Discard)
}
