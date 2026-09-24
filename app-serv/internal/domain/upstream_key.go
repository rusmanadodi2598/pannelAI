// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key.go
// @for       The UpstreamKey entity: one credential under an upstream endpoint,
//
//	with the circuit-breaker state the router reads to decide whether a
//	key may be used (SPEC-API-001 §7.5).
//
// @uses      internal/domain (ULID, AppError constructors).
// @reason    Key health is a domain rule, not a controller concern: the
//
//	threshold and the backoff decide whether a request is served or
//	failed over, so they are enforced here where no caller can bypass
//	them, and the stored value is opaque to everything above.
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

// UpstreamKeyStatus is the lifecycle state of an upstream key.
type UpstreamKeyStatus string

const (
	UpstreamKeyActive   UpstreamKeyStatus = "active"
	UpstreamKeyDisabled UpstreamKeyStatus = "disabled"
	UpstreamKeyError    UpstreamKeyStatus = "error"
)

// ParseUpstreamKeyStatus validates a wire value before it reaches the entity.
func ParseUpstreamKeyStatus(s string) (UpstreamKeyStatus, error) {
	status := UpstreamKeyStatus(s)
	switch status {
	case UpstreamKeyActive, UpstreamKeyDisabled, UpstreamKeyError:
		return status, nil
	default:
		return "", NewValidationError("invalid status: " + s)
	}
}

// IsValid reports whether the caller may transition to this status directly.
// `error` is set by RecordFailure, not by a PATCH, so a caller cannot hand-set
// the circuit state that health tracking owns.
func (s UpstreamKeyStatus) IsValid() bool {
	return s == UpstreamKeyActive || s == UpstreamKeyDisabled
}

// UpstreamKey is one credential under an upstream endpoint. Fields are
// unexported on purpose (AGENTS.md §2.2): every transition goes through a
// method, and the stored secret has no accessor at all.
type UpstreamKey struct {
	id                string
	endpointID        string
	label             string
	valueEncrypted    string
	keyHint           string
	priority          int
	status            UpstreamKeyStatus
	lastUsedAt        *time.Time
	lastError         string
	consecutiveErrors int
	rateLimitedUntil  *time.Time
	createdAt         time.Time
	updatedAt         time.Time
}

// NewUpstreamKey is the only constructor for a freshly added key. The value
// arrives already encrypted: encryption is a service concern because the key
// material comes from configuration, while the aggregate only guarantees the
// plaintext never becomes part of its state.
func NewUpstreamKey(label, endpointID, id, valueEncrypted, keyHint string, priority int, now time.Time) (UpstreamKey, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return UpstreamKey{}, NewValidationError("key label is required")
	}
	if len(label) > 120 {
		return UpstreamKey{}, NewValidationError("key label must be at most 120 characters")
	}
	if strings.TrimSpace(endpointID) == "" {
		return UpstreamKey{}, NewValidationError("endpoint id is required")
	}
	if valueEncrypted == "" {
		return UpstreamKey{}, NewValidationError("key value is required")
	}
	if strings.TrimSpace(keyHint) == "" {
		return UpstreamKey{}, NewValidationError("key hint is required")
	}
	if priority < 1 {
		return UpstreamKey{}, NewValidationError("priority must be at least 1")
	}
	if id == "" {
		id = IDPrefixUpstreamKey + NewULID(now)
	}
	return UpstreamKey{
		id:             id,
		endpointID:     endpointID,
		label:          label,
		valueEncrypted: valueEncrypted,
		keyHint:        keyHint,
		priority:       priority,
		status:         UpstreamKeyActive,
		createdAt:      now,
		updatedAt:      now,
	}, nil
}

// RehydrateUpstreamKey rebuilds a stored row. It is for the repository load
// path only; never use it to add a key.
func RehydrateUpstreamKey(
	id, endpointID, label, valueEncrypted, keyHint string,
	priority int,
	status UpstreamKeyStatus,
	lastUsedAt *time.Time,
	lastError string,
	consecutiveErrors int,
	rateLimitedUntil *time.Time,
	createdAt, updatedAt time.Time,
) UpstreamKey {
	return UpstreamKey{
		id:                id,
		endpointID:        endpointID,
		label:             label,
		valueEncrypted:    valueEncrypted,
		keyHint:           keyHint,
		priority:          priority,
		status:            status,
		lastUsedAt:        lastUsedAt,
		lastError:         lastError,
		consecutiveErrors: consecutiveErrors,
		rateLimitedUntil:  rateLimitedUntil,
		createdAt:         createdAt,
		updatedAt:         updatedAt,
	}
}

// Accessors expose state without allowing mutation. There is deliberately no
// accessor for the plaintext value: only EncryptedValue is readable, and only
// the repository and the transport see it.
func (k UpstreamKey) ID() string                   { return k.id }
func (k UpstreamKey) EndpointID() string           { return k.endpointID }
func (k UpstreamKey) Label() string                { return k.label }
func (k UpstreamKey) EncryptedValue() string       { return k.valueEncrypted }
func (k UpstreamKey) Hint() string                 { return k.keyHint }
func (k UpstreamKey) Priority() int                { return k.priority }
func (k UpstreamKey) Status() UpstreamKeyStatus    { return k.status }
func (k UpstreamKey) LastUsedAt() *time.Time       { return k.lastUsedAt }
func (k UpstreamKey) LastError() string            { return k.lastError }
func (k UpstreamKey) ConsecutiveErrors() int       { return k.consecutiveErrors }
func (k UpstreamKey) RateLimitedUntil() *time.Time { return k.rateLimitedUntil }
func (k UpstreamKey) CreatedAt() time.Time         { return k.createdAt }
func (k UpstreamKey) UpdatedAt() time.Time         { return k.updatedAt }

// Update applies a PATCH to the mutable fields. An empty label or a non-positive
// priority is rejected rather than silently ignored, so a client typo surfaces.
func (k *UpstreamKey) Update(label string, priority int, now time.Time) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return NewValidationError("key label is required")
	}
	if len(label) > 120 {
		return NewValidationError("key label must be at most 120 characters")
	}
	if priority < 1 {
		return NewValidationError("priority must be at least 1")
	}
	k.label = label
	k.priority = priority
	k.updatedAt = now
	return nil
}

// Transition applies a status change from a PATCH. `error` is not a settable
// value: the circuit breaker owns that state, and letting a client set it would
// let the panel hide a key the router still considers healthy.
func (k *UpstreamKey) Transition(next UpstreamKeyStatus) error {
	if !next.IsValid() {
		return NewValidationError("invalid status: " + string(next))
	}
	k.status = next
	if next == UpstreamKeyDisabled {
		// A disabled key is not in a backoff; keeping the timestamp would make
		// the panel show a countdown for a key that is switched off.
		k.rateLimitedUntil = nil
	}
	return nil
}
