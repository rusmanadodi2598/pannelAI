// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_oauth_retry.go
// @for       The dead-letter OAuth account's retry ladder: when it is next asked for a token, and how the window widens each time that attempt fails.
// @uses      time.
// @reason    A dead-lettered refresh token is expired, so the worker's due test says "due" on every tick and an unfiltered sweep re-spends the whole attempt budget on a vendor that already refused it (draft 050 T2). The window is what turns that into a widening visit, and it is stored because a process restart must not reset an account that has already been visited ten times.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

import "time"

// The dead-letter retry ladder: the first visit an hour after the account was
// buried, doubling per failed visit, and an account that has been refused ten times
// is asked once a day.
const (
	RefreshRetryBase    = time.Hour
	RefreshRetryCeiling = 24 * time.Hour
	// refreshRetryMaxStep is where the doubling stops: 1h, 2h, 4h, 8h, 16h, then the
	// ceiling and it holds there. The shift is bounded so a long-lived grave cannot
	// overflow it, and it reaches the ceiling rather than stopping short of a day.
	refreshRetryMaxStep = 6
)

// RefreshRetryAt is when the worker may next attempt this account's refresh grant,
// or nil when nothing is scheduled: a live account and a freshly recovered one both
// read nil.
func (e UpstreamEndpoint) RefreshRetryAt() *time.Time { return e.refreshRetryAt }

// RefreshRetryRounds is how many times the account has been buried and revisited
// without a grant succeeding. It is the ladder's step, kept for the panel and for a
// reader who wants to know how long this account has been dead.
func (e UpstreamEndpoint) RefreshRetryRounds() int { return e.refreshRetryRounds }

// RefreshRetryDue reports whether a dead-lettered account may be attempted now. An
// account with no schedule is due: the schedule only ever withholds a grave.
func (e UpstreamEndpoint) RefreshRetryDue(now time.Time) bool {
	return e.refreshRetryAt == nil || !now.Before(*e.refreshRetryAt)
}

// ScheduleRefreshRetry buries the account, or revisits its grave, and returns the
// new deadline. The window is the ladder step it just took.
//
// The step is counted in memory and written from here, so two replicas burying the
// same account can both record step N+1. That is accepted deliberately: the cost is
// one wider visit than the ladder would have given, on a path that withholds nothing
// from serving, and the alternative is a second write statement per dead-letter.
func (e *UpstreamEndpoint) ScheduleRefreshRetry(now time.Time) time.Time {
	if e.refreshRetryRounds < refreshRetryMaxStep {
		e.refreshRetryRounds++
	}
	window := RefreshRetryBase << uint(e.refreshRetryRounds-1)
	if window <= 0 || window > RefreshRetryCeiling {
		window = RefreshRetryCeiling
	}
	deadline := now.Add(window)
	e.refreshRetryAt = &deadline
	e.updatedAt = now
	return deadline
}

// ClearRefreshRetry ends the ladder: a grant the vendor accepted means the account
// is not a grave any more, so the next refresh is due on the token's own clock.
func (e *UpstreamEndpoint) ClearRefreshRetry(now time.Time) {
	if e.refreshRetryAt == nil && e.refreshRetryRounds == 0 {
		return
	}
	e.refreshRetryAt = nil
	e.refreshRetryRounds = 0
	e.updatedAt = now
}
