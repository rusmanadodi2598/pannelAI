// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_operator.go
// @for       The operator's transitions on an account: its tags, and the disable with the window that releases it.
// @uses      time, internal/domain (Tag, TagSet).
// @reason    A parked account re-enters rotation when its own window runs out, and nothing has to notice: the deadline is read against the wall clock where availability is already decided, so an expired park needs no sweeper and no worker, the same invariant a health park already holds. The row keeps `disabled` until a write replaces it, which is what makes the deadline auditable after the fact.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-16
package domain

import "time"

// Tags returns the account's tags in the order they were stored.
func (e UpstreamEndpoint) Tags() []Tag { return e.tags.Copy() }

// DisabledUntil is the deadline an operator park runs out at, or nil when the switch
// is permanent.
func (e UpstreamEndpoint) DisabledUntil() *time.Time { return e.disabledUntil }

// DisableWindowElapsed reports an account the operator parked whose window has run
// out. It still reads `disabled`, and the router serves it: the window states its own
// length, so releasing it is a read rather than a job.
func (e UpstreamEndpoint) DisableWindowElapsed(now time.Time) bool {
	return e.status == UpstreamEndpointDisabled && e.disabledUntil != nil && !now.Before(*e.disabledUntil)
}

// Disable switches the account out of rotation until `until`, or permanently when
// `until` is zero or already past. The health park goes with it: an account the
// operator switched off is not in a backoff, and leaving the timestamp would make the
// panel count down for something already stopped.
func (e *UpstreamEndpoint) Disable(until, now time.Time) {
	e.status = UpstreamEndpointDisabled
	e.disabledUntil = parkWindow(until, now)
	e.rateLimitedUntil = nil
	e.updatedAt = now
}

// Enable returns the account to rotation and clears what being switched off had
// accumulated: its window, the health park, and the breaker run that produced it, so a
// re-enabled account gets one honest attempt instead of inheriting a strike count.
func (e *UpstreamEndpoint) Enable(now time.Time) {
	e.status = UpstreamEndpointActive
	e.disabledUntil = nil
	e.rateLimitedUntil = nil
	e.consecutiveErrors = 0
	e.updatedAt = now
}

// AddTag marks one tag onto the account. It reports whether the set changed, so a bulk
// write can count what it actually did.
func (e *UpstreamEndpoint) AddTag(raw string, now time.Time) (bool, error) {
	added, err := e.tags.Add(raw)
	if err != nil {
		return false, err
	}
	if added {
		e.updatedAt = now
	}
	return added, nil
}

// RemoveTag drops one tag from the account.
func (e *UpstreamEndpoint) RemoveTag(raw string, now time.Time) (bool, error) {
	removed, err := e.tags.Remove(raw)
	if err != nil {
		return false, err
	}
	if removed {
		e.updatedAt = now
	}
	return removed, nil
}

// ClearTags takes every label off the account in one action.
func (e *UpstreamEndpoint) ClearTags(now time.Time) {
	if len(e.tags) == 0 {
		return
	}
	e.tags = nil
	e.updatedAt = now
}
