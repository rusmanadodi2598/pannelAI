// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_retention_test.go
// @for       The usage retention worker's cutoff, one-shot cycle, and cancellation.
// @uses      context, io, log/slog, testing, time.
// @reason    The purge cutoff is the only thing that decides what billing history survives, so it is pinned against a fixed clock rather than inferred from a real run.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-22
package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type recordingUsagePurger struct {
	cutoff  time.Time
	deleted int64
	err     error
}

func (p *recordingUsagePurger) DeleteUsageOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	p.cutoff = cutoff
	return p.deleted, p.err
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestUsageRetentionWorkerUsesTheConfiguredWindow(t *testing.T) {
	fixed := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	purger := &recordingUsagePurger{deleted: 7}
	worker, err := NewUsageRetentionWorker(purger,
		DefaultUsageRetentionPolicy(90), silentLogger())
	if err != nil {
		t.Fatalf("NewUsageRetentionWorker() error = %v", err)
	}
	worker.now = func() time.Time { return fixed }

	deleted, ran := worker.RunOnce(context.Background())
	if !ran || deleted != 7 {
		t.Fatalf("RunOnce() = (%d, %v), want (7, true)", deleted, ran)
	}
	if want := fixed.Add(-90 * 24 * time.Hour); !purger.cutoff.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", purger.cutoff, want)
	}
}

func TestUsageRetentionWorkerStopsOnCancellation(t *testing.T) {
	worker, err := NewUsageRetentionWorker(&recordingUsagePurger{},
		DefaultUsageRetentionPolicy(90), silentLogger())
	if err != nil {
		t.Fatalf("NewUsageRetentionWorker() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}
