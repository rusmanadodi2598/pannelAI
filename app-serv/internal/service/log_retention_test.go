// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/log_retention_test.go
// @for       Table-driven tests for the request-log retention worker.
// @uses      testing, context, time, internal/repository.
// @reason    The retention worker owns a long-lived ticker, retry state, and a
//
//	panic boundary. A regression in any of those can leave rows forever,
//	keep a goroutine alive after shutdown, or kill the process, so the
//	worker is driven directly under the race detector and the canonical
//	LogService purge is exercised for its dynamic settings behavior.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewLogRetentionWorker_ValidatesDependenciesAndPolicy(t *testing.T) {
	valid := retentionPolicy()
	cases := []struct {
		name   string
		purger LogPurger
		policy LogRetentionPolicy
		wantOK bool
	}{
		{name: "valid", purger: &retentionPurger{}, policy: valid, wantOK: true},
		{name: "nil purger", policy: valid},
		{name: "zero interval", purger: &retentionPurger{}, policy: LogRetentionPolicy{MaxAttempts: 1, Timeout: time.Second}},
		{name: "zero attempts", purger: &retentionPurger{}, policy: LogRetentionPolicy{Interval: time.Second, Timeout: time.Second}},
		{name: "zero timeout", purger: &retentionPurger{}, policy: LogRetentionPolicy{Interval: time.Second, MaxAttempts: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worker, err := NewLogRetentionWorker(tc.purger, tc.policy, nil)
			if tc.wantOK {
				if err != nil || worker == nil {
					t.Fatalf("NewLogRetentionWorker() = %v, want success", err)
				}
				return
			}
			if err == nil {
				t.Fatal("NewLogRetentionWorker() = nil error, want validation failure")
			}
		})
	}
}

func TestLogRetentionWorker_RunOnceRetriesAndResets(t *testing.T) {
	purger := &retentionPurger{err: errors.New("database unavailable"), deleted: 4}
	policy := retentionPolicy()
	policy.MaxAttempts = 2
	worker := newRetentionWorker(t, purger, policy)

	if deleted, ran := worker.RunOnce(context.Background()); !ran || deleted != 0 {
		t.Fatalf("first RunOnce() = (%d, %t), want (0, true)", deleted, ran)
	}
	if deleted, ran := worker.RunOnce(context.Background()); !ran || deleted != 0 {
		t.Fatalf("second RunOnce() = (%d, %t), want (0, true)", deleted, ran)
	}

	purger.mu.Lock()
	purger.err = nil
	purger.mu.Unlock()
	if deleted, ran := worker.RunOnce(context.Background()); !ran || deleted != 4 {
		t.Fatalf("successful RunOnce() = (%d, %t), want (4, true)", deleted, ran)
	}
	if got := purger.callCount(); got != 3 {
		t.Fatalf("purge calls = %d, want 3", got)
	}
}

func TestLogRetentionWorker_RunOnceRecoversPanic(t *testing.T) {
	purger := &retentionPurger{panicOn: true, deleted: 5}
	worker := newRetentionWorker(t, purger, retentionPolicy())

	if deleted, ran := worker.RunOnce(context.Background()); !ran || deleted != 0 {
		t.Fatalf("panic RunOnce() = (%d, %t), want (0, true)", deleted, ran)
	}
	purger.mu.Lock()
	purger.panicOn = false
	purger.mu.Unlock()
	if deleted, ran := worker.RunOnce(context.Background()); !ran || deleted != 5 {
		t.Fatalf("post-panic RunOnce() = (%d, %t), want (5, true)", deleted, ran)
	}
}

func TestLogRetentionWorker_RunOnceRefusesConcurrentCycle(t *testing.T) {
	purger := &retentionPurger{started: make(chan struct{}), release: make(chan struct{})}
	worker := newRetentionWorker(t, purger, retentionPolicy())
	finished := make(chan bool, 1)
	go func() {
		_, ran := worker.RunOnce(context.Background())
		finished <- ran
	}()
	<-purger.started
	if _, ran := worker.RunOnce(context.Background()); ran {
		t.Fatal("concurrent RunOnce() ran, want it refused")
	}
	close(purger.release)
	if ran := <-finished; !ran {
		t.Fatal("first RunOnce() reported it did not run")
	}
}

func TestLogRetentionWorker_RunStopsOnCancellation(t *testing.T) {
	policy := retentionPolicy()
	policy.Interval = time.Hour
	worker := newRetentionWorker(t, &retentionPurger{}, policy)
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(returned)
	}()
	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

func TestLogRetentionWorker_UsesCurrentRetentionSetting(t *testing.T) {
	const nowDay = 18
	fixed := time.Date(2026, 9, nowDay, 12, 0, 0, 0, time.UTC)
	settings := &retentionSettingsRepo{retentionDays: 7}
	logs := &retentionLogRepo{deleted: 2}
	settingsSvc, err := NewSettingsService(SettingsServiceDeps{Repo: settings})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}
	logSvc, err := NewLogService(LogServiceDeps{Logs: logs, Settings: settingsSvc})
	if err != nil {
		t.Fatalf("NewLogService() error = %v", err)
	}
	logSvc.clock = func() time.Time { return fixed }
	worker := newRetentionWorker(t, logSvc, retentionPolicy())

	worker.RunOnce(context.Background())
	settings.mu.Lock()
	settings.retentionDays = 2
	settings.mu.Unlock()
	worker.RunOnce(context.Background())

	logs.mu.Lock()
	defer logs.mu.Unlock()
	if len(logs.cutoffs) != 2 {
		t.Fatalf("cutoff calls = %d, want 2", len(logs.cutoffs))
	}
	if want := fixed.AddDate(0, 0, -7); !logs.cutoffs[0].Equal(want) {
		t.Fatalf("first cutoff = %s, want %s", logs.cutoffs[0], want)
	}
	if want := fixed.AddDate(0, 0, -2); !logs.cutoffs[1].Equal(want) {
		t.Fatalf("second cutoff = %s, want %s", logs.cutoffs[1], want)
	}
}

func newRetentionWorker(t *testing.T, purger LogPurger, policy LogRetentionPolicy) *LogRetentionWorker {
	t.Helper()
	worker, err := NewLogRetentionWorker(purger, policy, nil)
	if err != nil {
		t.Fatalf("NewLogRetentionWorker() error = %v", err)
	}
	return worker
}
