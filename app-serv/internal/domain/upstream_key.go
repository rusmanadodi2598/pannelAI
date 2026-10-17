// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key.go
// @for       The UpstreamKey entity: one credential under an upstream endpoint, with the circuit-breaker state the router reads to decide whether a key may be used (SPEC-API-001 §7.5).
// @uses      internal/domain (ULID, AppError constructors).
// @reason    Key health is a domain rule, not a controller concern: the threshold and the backoff decide whether a request is served or failed over, so they are enforced here where no caller can bypass them, and the stored value is opaque to everything above.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
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

// KeyRotationState is the load-path carrier for when a key was chosen and how long each
// failure class has been hitting it, so RehydrateUpstreamKey gains one argument rather than
// five. It is a plain value with exported fields like EndpointParity: it transports stored
// rows, is never returned outside this package, and the rules stay on UpstreamKey's methods.
type KeyRotationState struct {
	// LastSelectedAt is when the router last spent this credential, which is not when it last
	// worked: a key chosen and then lost at the network still advanced, and ordering reads this.
	LastSelectedAt *time.Time
	// The three runs replace one shared counter as the ladder's step, because a run of 429s
	// was raising the credential window. A class nobody failed with reads zero.
	ConsecutiveAuth      int
	ConsecutiveRateLimit int
	ConsecutiveBilling   int
	// LastErrorClass names which run the last failure advanced, spelled as KeyFailureClass
	// names itself. The health write needs it to choose its column and the panel needs it
	// because last_error holds the vendor's sentence, not the fact that it was a 429.
	LastErrorClass string
	// ResetSource and SignatureID are the provenance of the park that is on the row:
	// which statement of the vendor's produced the wait, and which signature
	// recognised the failure. They read out to an operator only; nothing in the
	// router decides anything from them, and a row parked before the classifier
	// existed carries neither.
	ResetSource string
	SignatureID string
}

// UpstreamKey is one credential under an upstream endpoint. Fields are
// unexported on purpose (AGENTS.md §2.2): every transition goes through a
// method, and the stored secret has no accessor at all.
type UpstreamKey struct {
	id                   string
	endpointID           string
	label                string
	valueEncrypted       string
	keyHint              string
	priority             int
	status               UpstreamKeyStatus
	lastUsedAt           *time.Time
	lastSelectedAt       *time.Time
	lastError            string
	consecutiveErrors    int
	consecutiveAuth      int
	consecutiveRateLimit int
	consecutiveBilling   int
	lastErrorClass       string
	rateLimitedUntil     *time.Time
	// resetSource and signatureID record how a park's rest was arrived at: which
	// of the vendor's own statements produced it, and which signature recognised
	// the failure. They are diagnostics for an operator reading a parked key, not
	// inputs to the health model, so nothing decides a window from them and a park
	// written by an older binary simply carries neither.
	resetSource string
	signatureID string
	// tags and disabledUntil are the operator's own marks: the sets a bulk action
	// addresses this credential by, and the deadline its park runs out at. Their
	// rules live in upstream_key_operator.go.
	tags          TagSet
	disabledUntil *time.Time
	createdAt     time.Time
	updatedAt     time.Time
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
	rotation KeyRotationState,
	rateLimitedUntil *time.Time,
	createdAt, updatedAt time.Time,
	operator OperatorState,
) UpstreamKey {
	return UpstreamKey{
		id:                   id,
		endpointID:           endpointID,
		label:                label,
		valueEncrypted:       valueEncrypted,
		keyHint:              keyHint,
		priority:             priority,
		status:               status,
		lastUsedAt:           lastUsedAt,
		lastSelectedAt:       rotation.LastSelectedAt,
		lastError:            lastError,
		consecutiveErrors:    consecutiveErrors,
		consecutiveAuth:      rotation.ConsecutiveAuth,
		consecutiveRateLimit: rotation.ConsecutiveRateLimit,
		consecutiveBilling:   rotation.ConsecutiveBilling,
		lastErrorClass:       rotation.LastErrorClass,
		resetSource:          rotation.ResetSource,
		signatureID:          rotation.SignatureID,
		rateLimitedUntil:     rateLimitedUntil,
		tags:                 operator.Tags,
		disabledUntil:        operator.DisabledUntil,
		createdAt:            createdAt,
		updatedAt:            updatedAt,
	}
}

// Accessors live in upstream_key_reads.go: the read list grows as rotation and
// parking learn new facts, while the transitions here stay one per state change.

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
	// A PATCH states no window, so it leaves none behind: a stale deadline would
	// release a switch the operator moved by hand.
	k.disabledUntil = nil
	if next == UpstreamKeyDisabled {
		// A disabled key is not in a backoff; keeping the timestamp would make
		// the panel show a countdown for a key that is switched off.
		k.rateLimitedUntil = nil
	}
	return nil
}
