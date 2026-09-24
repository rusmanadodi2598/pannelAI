// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_keys.go
// @for       Key CRUD and health transitions on an UpstreamEndpoint, plus the
//
//	read helpers the router and the panel use to pick a key.
//
// @uses      internal/domain (UpstreamKey, error constructors), sort, strings.
// @reason    AGENTS.md §2.2 makes the aggregate root the only mutation boundary,
//
//	so every key change runs here rather than on a detached key value;
//	this file holds that surface, and the "keep one usable credential"
//	invariant is enforced on the collection because no single key can
//	see it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"time"
)

// AddKey creates and attaches a key, assigning the next free priority when the
// caller passes none. It rejects a duplicate label, because two keys of one
// endpoint are told apart by their label in every response.
func (e *UpstreamEndpoint) AddKey(label, valueEncrypted, keyHint string, priority int, now time.Time) (UpstreamKey, error) {
	label = strings.TrimSpace(label)
	if e.hasLabel(label) {
		return UpstreamKey{}, NewConflictError("a key with label " + label + " already exists on this endpoint")
	}
	if priority < 1 {
		priority = e.nextKeyPriority()
	}
	key, err := NewUpstreamKey(label, e.id, "", valueEncrypted, keyHint, priority, now)
	if err != nil {
		return UpstreamKey{}, err
	}
	e.keys = append(e.keys, key)
	e.updatedAt = now
	return key, nil
}

// AttachKey adds an already-built key. It serves the repository load path and
// tests that need a specific key identity.
func (e *UpstreamEndpoint) AttachKey(key UpstreamKey) {
	e.keys = append(e.keys, key)
}

// RemoveKey deletes a key, refusing when that would leave an api_key endpoint
// unable to route (SPEC-API-001 §7.5).
func (e *UpstreamEndpoint) RemoveKey(id string, now time.Time) error {
	index := e.keyIndex(id)
	if index < 0 {
		return NewNotFoundError("upstream key not found")
	}
	if e.authType == UpstreamAuthAPIKey && e.activeKeyCount()-activeKeys(e.keys[index]) < 1 {
		return NewConflictError("an api_key endpoint must keep at least one active key")
	}

	e.keys = append(e.keys[:index], e.keys[index+1:]...)
	e.updatedAt = now
	return nil
}

// UpdateKey applies a PATCH to one key. The stored value is replaced only when a
// new one is supplied, because the value is write-only on the wire: a PATCH that
// omits it must keep the existing credential rather than blank it.
func (e *UpstreamEndpoint) UpdateKey(id, label, valueEncrypted, keyHint string, priority int, status string, now time.Time) (UpstreamKey, error) {
	index := e.keyIndex(id)
	if index < 0 {
		return UpstreamKey{}, NewNotFoundError("upstream key not found")
	}

	key := e.keys[index]
	if valueEncrypted != "" {
		replaced, err := NewUpstreamKey(key.Label(), e.id, key.ID(), valueEncrypted, keyHint, key.Priority(), key.CreatedAt())
		if err != nil {
			return UpstreamKey{}, err
		}
		// The replacement keeps the health the key already earned: a rotated
		// credential is the same account, so its circuit state is still valid.
		replaced.status = key.status
		replaced.lastUsedAt = key.lastUsedAt
		replaced.lastError = key.lastError
		replaced.consecutiveErrors = key.consecutiveErrors
		replaced.rateLimitedUntil = key.rateLimitedUntil
		key = replaced
	}
	if err := key.Update(label, priority, now); err != nil {
		return UpstreamKey{}, err
	}
	if status != "" {
		parsed, err := ParseUpstreamKeyStatus(status)
		if err != nil {
			return UpstreamKey{}, err
		}
		if err := key.Transition(parsed); err != nil {
			return UpstreamKey{}, err
		}
	}
	e.keys[index] = key
	e.updatedAt = now
	return key, nil
}

// SetKeyStatus applies a status change to one key. It exists because Key returns
// a copy: a caller cannot mutate the aggregate through a detached value
// (AGENTS.md §2.2), so the transition has to run here.
func (e *UpstreamEndpoint) SetKeyStatus(id, status string, now time.Time) (UpstreamKey, error) {
	parsed, err := ParseUpstreamKeyStatus(status)
	if err != nil {
		return UpstreamKey{}, err
	}
	index := e.keyIndex(id)
	if index < 0 {
		return UpstreamKey{}, NewNotFoundError("upstream key not found")
	}
	// Refuse a change that would leave an api_key endpoint with no active key,
	// the same invariant RemoveKey enforces.
	if parsed != UpstreamKeyActive && e.authType == UpstreamAuthAPIKey {
		if e.activeKeyCount()-activeKeys(e.keys[index]) < 1 {
			return UpstreamKey{}, NewConflictError("an api_key endpoint must keep at least one active key")
		}
	}
	if err := e.keys[index].Transition(parsed); err != nil {
		return UpstreamKey{}, err
	}
	e.updatedAt = now
	return e.keys[index], nil
}

// RecordKeyFailure applies a failed attempt to one key and returns its state, so
// the caller can log which key was parked.
func (e *UpstreamEndpoint) RecordKeyFailure(id, reason string, class KeyFailureClass, now time.Time) (UpstreamKey, error) {
	index := e.keyIndex(id)
	if index < 0 {
		return UpstreamKey{}, NewNotFoundError("upstream key not found")
	}
	e.keys[index].RecordFailure(reason, class, now)
	e.updatedAt = now
	return e.keys[index], nil
}

// RecordKeySuccess clears the health state of one key after a served request.
func (e *UpstreamEndpoint) RecordKeySuccess(id string, now time.Time) (UpstreamKey, error) {
	index := e.keyIndex(id)
	if index < 0 {
		return UpstreamKey{}, NewNotFoundError("upstream key not found")
	}
	e.keys[index].RecordSuccess(now)
	e.lastUsedAt = &now
	e.updatedAt = now
	return e.keys[index], nil
}

// keyIndex finds a key's position, or -1 when the endpoint does not own it.
func (e UpstreamEndpoint) keyIndex(id string) int {
	for i, key := range e.keys {
		if key.ID() == id {
			return i
		}
	}
	return -1
}

// hasLabel reports whether a key already uses the label.
func (e UpstreamEndpoint) hasLabel(label string) bool {
	for _, key := range e.keys {
		if key.Label() == label {
			return true
		}
	}
	return false
}

// activeKeyCount reports how many keys are in the active status.
func (e UpstreamEndpoint) activeKeyCount() int {
	count := 0
	for _, key := range e.keys {
		if key.status == UpstreamKeyActive {
			count++
		}
	}
	return count
}

// activeKeys reports 1 when the key is active, so a removal can ask what the
// count would become without mutating anything first.
func activeKeys(key UpstreamKey) int {
	if key.status == UpstreamKeyActive {
		return 1
	}
	return 0
}

// nextKeyPriority returns one past the highest key priority, so an added key
// lands last rather than in the middle of an operator's ordering.
func (e UpstreamEndpoint) nextKeyPriority() int {
	highest := 0
	for _, key := range e.keys {
		if key.Priority() > highest {
			highest = key.Priority()
		}
	}
	return highest + 1
}
