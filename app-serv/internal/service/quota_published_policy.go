// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_published_policy.go
// @for       The published-quota poll worker's stated decisions: the family interval table, the failure backoff, the jitter, and who gets scheduled.
// @uses      context, math/rand, strings, sync, time, internal/domain, internal/repository.
// @reason    AGENTS.md §1.6 forbids "just retry forever" and "no retry" as unstated defaults, and §1.1 caps the file that would otherwise carry both these numbers and the sweep that spends them.
//
// RETRY POLICY — every endpoint is re-asked at its family's interval
// (publishedPollInterval), never at a global one: claude 429s an account asked more often
// than every ten minutes and the cloudcode families meter a project, so a faster default
// would spend the allowance the screen exists to show. A poll returning a Go error — row
// gone, credential will not open, cache refused the write — advances that endpoint's
// consecutive-failure run and waits the larger of the family floor and the doubled backoff.
// There is no in-process retry: the next sweep is the retry, so an outage costs a delayed
// re-read rather than a hot loop against an endpoint already refusing us.
//
// DEAD-LETTER POLICY — a repeatedly failing endpoint is not dropped from the queue. It stays
// scheduled at the ceiling interval and keeps its last good windows, because the ordinary
// cause is a credential the operator must re-authenticate: a row deleted after N failures
// needs a manual database edit to be polled again, while one left at the ceiling recovers on
// the first sweep after the fix. The run travels to the card, so "asked twelve times, no
// answer" is visible instead of silent. An endpoint stops being polled only by leaving the
// fleet, which cascades its cache row away.
//
// TERMINATION — Run returns when its context is cancelled; Sweep waits every poll it spawned
// through a wait-group, each in a goroutine that recovers its own panic.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-10-02
package service

import (
	"context"
	mathrand "math/rand"
	"strings"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

const (
	publishedPollDefaultInterval   = 2 * time.Minute
	publishedPollClaudeInterval    = 10 * time.Minute
	publishedPollCloudCodeInterval = 5 * time.Minute

	publishedPollBackoffBase    = 30 * time.Second
	publishedPollBackoffCeiling = 30 * time.Minute
)

// publishedPollIntervals is keyed by the registry provider id the endpoint row names — the same
// key quotafetch dispatches on, so a slower cadence is stated once, where the family is already
// known. claude is 10 minutes because its usage endpoint 429s a heavier cadence, as the
// reference throttles it. gemini-cli and antigravity are the cloudcode-pa families, which meter
// a project rather than a request, and grok-cli reads a credits proxy: all three get 5 minutes.
var publishedPollIntervals = map[string]time.Duration{
	"claude":      publishedPollClaudeInterval,
	"gemini-cli":  publishedPollCloudCodeInterval,
	"antigravity": publishedPollCloudCodeInterval,
	"grok-cli":    publishedPollCloudCodeInterval,
}

// publishedPollInterval is one family's floor. The floor is this code's decision, not the
// row's: a schedule stamped under an older, faster table is re-armed, not honoured.
func publishedPollInterval(providerID string) time.Duration {
	if interval, ok := publishedPollIntervals[providerID]; ok {
		return interval
	}
	return publishedPollDefaultInterval
}

// publishedNextDelay is the wait before one endpoint's next poll: the family floor with no
// failure run, otherwise the larger of floor and the base doubled once per failure spent. The
// max keeps a claude endpoint that failed twice — two minutes of backoff — from being polled
// five times faster than claude allows, and the doubling stops at half the ceiling.
func publishedNextDelay(providerID string, failures int) time.Duration {
	floor := publishedPollInterval(providerID)
	if failures < 1 {
		return floor
	}
	delay := publishedPollBackoffBase
	for step := 1; step < failures; step++ {
		if delay >= publishedPollBackoffCeiling/2 {
			return publishedPollBackoffCeiling
		}
		delay *= 2
	}
	return max(delay, floor)
}

// publishedJitter is the spread added to every delay, so a thousand accounts of one family do
// not wake on the same tick. Concurrent polls draw from one source, hence the lock.
type publishedJitter struct {
	mu  sync.Mutex
	rng *mathrand.Rand
}

func newPublishedJitter(seed int64) *publishedJitter {
	return &publishedJitter{rng: mathrand.New(mathrand.NewSource(seed))}
}

// delay returns d plus up to a quarter of it. Nothing is added at the backoff ceiling, so the
// ceiling stays a ceiling, nor to a zero delay — the seed stamp is due now.
func (j *publishedJitter) delay(d time.Duration) time.Duration {
	span := d / 4
	if span <= 0 || d >= publishedPollBackoffCeiling {
		return d
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return d + time.Duration(j.rng.Int63n(int64(span)+1))
}

// publishedPollProvider prefers the family the endpoint row itself names, which is what the live
// read echoes back; the scheduling row is the fallback for a read that failed before it could
// say. An account re-pointed at another family must be scheduled under the family it now is.
func publishedPollProvider(state domain.PublishedState, result PublishedUsage) string {
	if providerID := strings.TrimSpace(result.ProviderID); providerID != "" {
		return providerID
	}
	return state.ProviderID
}

// insideFloor refuses a poll the family floor forbids even though the stored row is due. A row
// the seed pass stamped but nobody has asked is not guarded: no answer to stale, no run to back off.
func (w *QuotaPublishedWorker) insideFloor(state domain.PublishedState, now time.Time) bool {
	if state.LastAttemptAt == nil || (state.FetchedAt == nil && state.ConsecutiveFailures == 0) {
		return false
	}
	return now.Sub(*state.LastAttemptAt) < publishedPollInterval(state.ProviderID)
}

// rearmFloor pushes a floor-skipped row out of the due window. RecordAttempt writes the instant
// it is handed as last_attempt_at, so the real last attempt passes through unchanged and only
// the next one moves; a row left due would take budget every tick and starve the fleet.
func (w *QuotaPublishedWorker) rearmFloor(ctx context.Context, state domain.PublishedState) {
	last := *state.LastAttemptAt
	storeCtx, cancel := context.WithTimeout(ctx, w.policy.StoreTimeout)
	err := w.store.RecordAttempt(storeCtx, domain.PublishedAttempt{
		EndpointID: state.EndpointID, ProviderID: state.ProviderID,
		AttemptedAt: last, NextAttemptAt: last.Add(publishedPollInterval(state.ProviderID)),
	})
	cancel()
	if err != nil {
		w.logger.Error("published quota worker could not re-arm a floor-skipped endpoint",
			"endpoint", state.EndpointID, "error", err)
	}
}

// PublishedPollPolicy carries the sweep's knobs; the tick is not here because Run takes it.
type PublishedPollPolicy struct {
	// Budget is the most endpoints one sweep asks about, so a thousand accounts cost a
	// bounded number of calls per tick (AGENTS.md §1.7).
	Budget int
	// Concurrency caps the provider calls in flight inside one sweep (§1.6).
	Concurrency int
	// FetchTimeout bounds one provider read and StoreTimeout one cache call, per endpoint, so
	// one hung account cannot hold a sweep open.
	FetchTimeout time.Duration
	StoreTimeout time.Duration
	// SeedPageSize is how many accounts one tick examines for a missing schedule.
	SeedPageSize int
}

// DefaultPublishedPollPolicy is the policy the composition root uses. A one-minute tick against
// a two-minute floor means a due endpoint waits at most one tick, and it is the budget, not the
// tick, that decides how fast the fleet is walked.
func DefaultPublishedPollPolicy() PublishedPollPolicy {
	return PublishedPollPolicy{
		Budget: 40, Concurrency: 4,
		FetchTimeout: 25 * time.Second, StoreTimeout: 10 * time.Second,
		SeedPageSize: 200,
	}
}

// WithSweepLimits replaces the two numbers an operator may want to move without a code
// change: how many accounts one sweep asks, and how many provider calls it keeps in
// flight. Zero or negative leaves the default standing, so a deployment that sets
// nothing behaves exactly as it did before this existed.
func (p PublishedPollPolicy) WithSweepLimits(budget int, concurrency int) PublishedPollPolicy {
	if budget > 0 {
		p.Budget = budget
	}
	if concurrency > 0 {
		p.Concurrency = concurrency
	}
	return p
}
