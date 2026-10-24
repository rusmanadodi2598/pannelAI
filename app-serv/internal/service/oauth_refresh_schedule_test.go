// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_refresh_schedule_test.go
// @for       Test helpers that advance the worker fixture to an endpoint's scheduled retry.
// @uses      testing, time.
// @reason    The worker no longer sleeps inline, so a burst of failed attempts is tested by moving the fixture clock to each recorded next attempt instead of calling Sweep back to back.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package service

import (
	"testing"
	"time"
)

// clockNow reads the fixture clock under the lock the tests already use.
func (f *workerFixture) clockNow() time.Time {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	return f.clockAt
}

// retryDelay answers how long the fixture must advance before the endpoint's
// recorded retry is due.
func (f *workerFixture) retryDelay(t *testing.T, endpointID string) time.Duration {
	t.Helper()
	f.worker.mu.Lock()
	at, scheduled := f.worker.next[endpointID]
	f.worker.mu.Unlock()
	if !scheduled {
		t.Fatalf("no retry scheduled for %s", endpointID)
	}
	return at.Sub(f.clockNow())
}
