// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_key_reads.go
// @for       Read helpers for the keys an UpstreamEndpoint owns, including the
//
//	selection the router performs.
//
// @uses      internal/domain (UpstreamKey), sort, time.
// @reason    The router asks the same two questions on every request ("which key
//
//	may I spend" and "which one would you pick"), and the panel shows
//	the second as its answer to "why did my request fail over". Keeping
//	both beside the ordering rule is what stops the two answers from
//	diverging.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"sort"
	"time"
)

// Keys returns a copy of the key list, so a caller cannot reorder or truncate
// the aggregate's own collection.
func (e UpstreamEndpoint) Keys() []UpstreamKey {
	out := make([]UpstreamKey, len(e.keys))
	copy(out, e.keys)
	return out
}

// Key returns a copy of one key by id, or an empty key when absent. A copy
// rather than a reference: health transitions run through this endpoint, so a
// caller mutating a detached key cannot silently corrupt the stored state.
func (e UpstreamEndpoint) Key(id string) UpstreamKey {
	for _, key := range e.keys {
		if key.ID() == id {
			return key
		}
	}
	return UpstreamKey{}
}

// KeyByPriority returns a copy of the key at a priority, or an empty key.
func (e UpstreamEndpoint) KeyByPriority(priority int) UpstreamKey {
	for _, key := range e.keys {
		if key.Priority() == priority {
			return key
		}
	}
	return UpstreamKey{}
}

// HasKey reports whether the endpoint owns a key with that id.
func (e UpstreamEndpoint) HasKey(id string) bool {
	for _, key := range e.keys {
		if key.ID() == id {
			return true
		}
	}
	return false
}

// ValidKeys returns the keys routing may spend, ordered by priority. A disabled
// key and a key inside its backoff window are excluded, so a caller never has
// to repeat the health rule.
func (e UpstreamEndpoint) ValidKeys(now time.Time) []UpstreamKey {
	valid := make([]UpstreamKey, 0, len(e.keys))
	for _, key := range e.keys {
		if key.Available(now) {
			valid = append(valid, key)
		}
	}
	sort.SliceStable(valid, func(a, b int) bool { return valid[a].Priority() < valid[b].Priority() })
	return valid
}

// NextKey reports the key routing would pick now: the least-recently-used
// healthy one, ties broken by priority then id. Rotation spreads spend across
// a key set instead of pinning the highest priority; the panel shows the same
// answer, so an operator can see why a request failed over without
// re-simulating the rule.
func (e UpstreamEndpoint) NextKey(now time.Time) (UpstreamKey, bool) {
	return e.NextKeySkipping(now, nil)
}

// NextKeySkipping is NextKey over the keys outside the spent set, which is what
// a request that already tried one credential asks: the same rule, minus the
// candidates this request has spent.
func (e UpstreamEndpoint) NextKeySkipping(now time.Time, spent map[string]struct{}) (UpstreamKey, bool) {
	valid := e.ValidKeys(now)
	best := -1
	for i, key := range valid {
		if _, ok := spent[key.ID()]; ok {
			continue
		}
		if best < 0 || keyIdleBefore(key, valid[best]) {
			best = i
		}
	}
	if best < 0 {
		return UpstreamKey{}, false
	}
	return valid[best], true
}

// keyIdleBefore orders candidates by idle time: a key never used beats a used
// one, an older use beats a newer one, and equal instants fall back to the
// stored priority then id so the order is stable across calls.
func keyIdleBefore(a, b UpstreamKey) bool {
	aUsed, bUsed := a.LastUsedAt(), b.LastUsedAt()
	switch {
	case aUsed == nil && bUsed != nil:
		return true
	case bUsed == nil && aUsed != nil:
		return false
	case aUsed != nil && bUsed != nil && !aUsed.Equal(*bUsed):
		return aUsed.Before(*bUsed)
	case a.Priority() != b.Priority():
		return a.Priority() < b.Priority()
	default:
		return a.ID() < b.ID()
	}
}

// ActiveKeyCount reports how many keys are in the active status. It is exported
// because the panel disables a delete control with it before the API refuses.
func (e UpstreamEndpoint) ActiveKeyCount() int { return e.activeKeyCount() }
