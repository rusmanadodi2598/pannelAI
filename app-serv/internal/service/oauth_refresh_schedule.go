// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_refresh_schedule.go
// @for       The OAuth refresh worker's per-endpoint next-attempt schedule, kept apart from the sweep it feeds.
// @uses      sync, time (through OAuthRefreshWorker).
// @reason    A failing account must not hold every other provider's refresh while it waits out its backoff. The sweep records when one endpoint may be tried again and moves on; Run wakes at the earliest recorded attempt instead of sleeping inside the loop.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package service

import "time"

// due reports whether the endpoint may be attempted at now.
func (w *OAuthRefreshWorker) due(endpointID string, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	at, scheduled := w.next[endpointID]
	return !scheduled || !now.Before(at)
}

// scheduleRetry records the first instant the endpoint may be attempted again.
func (w *OAuthRefreshWorker) scheduleRetry(endpointID string, at time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.next[endpointID] = at
}

// clearRetry ends the endpoint's failure run: neither its attempt count nor its
// next attempt survives a success or a dead letter.
func (w *OAuthRefreshWorker) clearRetry(endpointID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.attempts, endpointID)
	delete(w.next, endpointID)
}

// nextWake answers the wait before the next sweep: the interval, the earliest
// recorded retry, or zero when that retry is already due.
func (w *OAuthRefreshWorker) nextWake(interval time.Duration) time.Duration {
	now := w.now()
	w.mu.Lock()
	var soonest time.Time
	for _, at := range w.next {
		if soonest.IsZero() || at.Before(soonest) {
			soonest = at
		}
	}
	w.mu.Unlock()
	if soonest.IsZero() {
		return interval
	}
	delay := soonest.Sub(now)
	if delay < 0 {
		return 0
	}
	if delay > interval {
		return interval
	}
	return delay
}
