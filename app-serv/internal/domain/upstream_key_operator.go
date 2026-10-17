// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_operator.go
// @for       The operator's transitions on one credential: its tags, and the disable with the window that releases it.
// @uses      time, internal/domain (Tag, TagSet).
// @reason    A key is disabled on its own as often as its account is, and a key that re-enters rotation when its own window runs out needs no sweeper to release it. The rules are the endpoint's, restated here because the two aggregates own different counters and only their own roots may clear them.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-16
package domain

import "time"

// Tags returns the key's tags in the order they were stored.
func (k UpstreamKey) Tags() []Tag { return k.tags.Copy() }

// DisabledUntil is the deadline an operator park runs out at, or nil when the switch is
// permanent.
func (k UpstreamKey) DisabledUntil() *time.Time { return k.disabledUntil }

// DisableWindowElapsed reports a credential the operator parked whose window has run
// out. The row still reads `disabled`, and rotation picks it up: the window states its
// own length, so releasing it is a read rather than a job.
func (k UpstreamKey) DisableWindowElapsed(now time.Time) bool {
	return k.status == UpstreamKeyDisabled && k.disabledUntil != nil && !now.Before(*k.disabledUntil)
}

// Disable switches the credential out of rotation until `until`, or permanently when
// `until` is zero or already past. The health park goes with it, because a switched-off
// key is not in a backoff and a countdown on the panel would describe something that
// has already stopped.
func (k *UpstreamKey) Disable(until, now time.Time) {
	k.status = UpstreamKeyDisabled
	k.disabledUntil = parkWindow(until, now)
	k.rateLimitedUntil = nil
	k.updatedAt = now
}

// Enable returns the credential to rotation and clears what being switched off had
// accumulated: its window, the health park, and every class's run, so a re-enabled key
// faces one honest attempt rather than inheriting a streak it was never allowed to
// spend.
func (k *UpstreamKey) Enable(now time.Time) {
	k.status = UpstreamKeyActive
	k.disabledUntil = nil
	k.rateLimitedUntil = nil
	k.consecutiveErrors = 0
	k.consecutiveAuth = 0
	k.consecutiveRateLimit = 0
	k.consecutiveBilling = 0
	k.updatedAt = now
}

// AddTag marks one tag onto the credential.
func (k *UpstreamKey) AddTag(raw string, now time.Time) (bool, error) {
	added, err := k.tags.Add(raw)
	if err != nil {
		return false, err
	}
	if added {
		k.updatedAt = now
	}
	return added, nil
}

// RemoveTag drops one tag from the credential.
func (k *UpstreamKey) RemoveTag(raw string, now time.Time) (bool, error) {
	removed, err := k.tags.Remove(raw)
	if err != nil {
		return false, err
	}
	if removed {
		k.updatedAt = now
	}
	return removed, nil
}

// ClearTags takes every label off the credential in one action.
func (k *UpstreamKey) ClearTags(now time.Time) {
	if len(k.tags) == 0 {
		return
	}
	k.tags = nil
	k.updatedAt = now
}
