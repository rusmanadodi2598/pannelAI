// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_rotation.go
// @for       The rotation state a key carries: when the router last chose it, and what "least recently used" means once that is known.
// @uses      time.
// @reason    Choosing and serving are two different facts, and conflating them is why a burst never spread: only RecordSuccess stamped a key, so N concurrent selections read one identical timestamp and all picked the same credential, and a key chosen then refused at the network was neither parked nor stamped and stayed the oldest in the set forever. The rule lives beside the aggregate so no caller can select a key without advancing it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-14
package domain

import "time"

// MarkSelected records that the router chose this credential for one call. It is not a
// health transition and clears nothing: the call it announces has not happened, and its
// outcome arrives through RecordSuccess or RecordFailure. A key chosen and then lost to a
// DNS failure still advances, which is what stops it winning every later selection.
func (k *UpstreamKey) MarkSelected(now time.Time) {
	selected := now
	k.lastSelectedAt = &selected
	k.updatedAt = now
}

// IdleAt reports when rotation last got a chance to choose this key. The selection stamp
// wins wherever the two differ, because rotation asks "when was this last spent", not
// "when did this last work". A row predating migration 000019 has no stamp and falls back
// to the last success, so an upgraded database never reads an unused key as the oldest.
func (k UpstreamKey) IdleAt() *time.Time {
	if k.lastSelectedAt != nil {
		return k.lastSelectedAt
	}
	return k.lastUsedAt
}

// MarkSelected records that rotation chose this account, the keyless counterpart of
// RecordKeySelected: an OAuth or no-auth endpoint carries its credential on the row and
// owns no key, so without a stamp of its own it would keep no rotation state and a dead
// account would be chosen as often as a live one.
func (e *UpstreamEndpoint) MarkSelected(now time.Time) {
	selected := now
	e.lastSelectedAt = &selected
	e.updatedAt = now
}

// RecordKeySelected stamps one key of this endpoint, and the endpoint with it: the
// account was chosen at the same moment as the credential inside it. It returns the
// key it advanced, RecordKeySuccess's shape, so a caller can persist the child
// through the root it owns (AGENTS.md §2.2).
func (e *UpstreamEndpoint) RecordKeySelected(id string, now time.Time) (UpstreamKey, error) {
	index := e.keyIndex(id)
	if index < 0 {
		return UpstreamKey{}, NewNotFoundError("upstream key not found")
	}
	e.keys[index].MarkSelected(now)
	e.MarkSelected(now)
	return e.keys[index], nil
}

// LastSelectedAt reports when rotation last chose this account, or nil when it never
// has. The panel reads it for the same reason the degraded ordering does: the last
// success cannot say which account is next.
func (e UpstreamEndpoint) LastSelectedAt() *time.Time { return e.lastSelectedAt }
