// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_operator_keys.go
// @for       The operator's transitions on one credential seen through the account that owns it: the park with its window, and the tags.
// @uses      time, internal/domain (UpstreamKey, error constructors).
// @reason    A bulk action addresses keys directly, but §2.2 makes the endpoint the only mutation boundary, and the "keep one usable credential" rule is a fact about the collection no key can see. Routing those actions through the root is what keeps a mass disable from emptying an account, and it enforces the rule over the set the action walks rather than over the snapshot each row was read from.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-16
package domain

import "time"

// keySlot locates one credential inside the account. The pointer is into the
// aggregate's own storage, so a caller that mutates through it mutates the root.
func (e *UpstreamEndpoint) keySlot(id string) (int, *UpstreamKey, error) {
	index := e.keyIndex(id)
	if index < 0 {
		return -1, nil, NewNotFoundError("upstream key not found")
	}
	return index, &e.keys[index], nil
}

// DisableKey parks one credential until `until`, or permanently when `until` is zero or
// already past. The account's floor is checked first: a set of disables that would leave
// a key-backed account with nothing usable is refused on the row that does it.
func (e *UpstreamEndpoint) DisableKey(id string, until, now time.Time) error {
	index, key, err := e.keySlot(id)
	if err != nil {
		return err
	}
	if err := e.refusesDeactivation(index, UpstreamKeyDisabled); err != nil {
		return err
	}
	key.Disable(until, now)
	e.updatedAt = now
	return nil
}

// EnableKey releases one credential and clears what the switch had accumulated.
func (e *UpstreamEndpoint) EnableKey(id string, now time.Time) error {
	_, key, err := e.keySlot(id)
	if err != nil {
		return err
	}
	key.Enable(now)
	e.updatedAt = now
	return nil
}

// KeyDisableWindowElapsed reports a credential under this account whose operator window
// has run out, the same fact the key states for itself, answered by id.
func (e UpstreamEndpoint) KeyDisableWindowElapsed(id string, now time.Time) bool {
	for _, key := range e.keys {
		if key.ID() == id {
			return key.DisableWindowElapsed(now)
		}
	}
	return false
}

// AddKeyTag marks one tag onto a credential and answers whether the set moved.
func (e *UpstreamEndpoint) AddKeyTag(id, raw string, now time.Time) (bool, error) {
	_, key, err := e.keySlot(id)
	if err != nil {
		return false, err
	}
	added, err := key.AddTag(raw, now)
	if err != nil {
		return false, err
	}
	if added {
		e.updatedAt = now
	}
	return added, nil
}

// RemoveKeyTag drops one tag from a credential.
func (e *UpstreamEndpoint) RemoveKeyTag(id, raw string, now time.Time) (bool, error) {
	_, key, err := e.keySlot(id)
	if err != nil {
		return false, err
	}
	removed, err := key.RemoveTag(raw, now)
	if err != nil {
		return false, err
	}
	if removed {
		e.updatedAt = now
	}
	return removed, nil
}

// ClearKeyTags takes every label off one credential.
func (e *UpstreamEndpoint) ClearKeyTags(id string, now time.Time) error {
	_, key, err := e.keySlot(id)
	if err != nil {
		return err
	}
	before := len(key.Tags())
	key.ClearTags(now)
	if len(key.Tags()) != before {
		e.updatedAt = now
	}
	return nil
}
