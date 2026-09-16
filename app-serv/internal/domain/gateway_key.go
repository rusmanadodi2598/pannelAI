// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/gateway_key.go
// @for       The GatewayKey aggregate root: the client-facing credential CLI
//
//	tools present as Authorization: Bearer against the data plane.
//
// @uses      internal/domain (ULID, error sentinels).
// @reason    SPEC-API-001 §7.3 needs a revocable key whose plaintext is shown
//
//	once and whose later reads expose only key_hint; the transitions
//	are enforced here so no caller can put the entity in a bad state.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     domain
// @stability experimental
// @since     2026-09-16
package domain

import "time"

// ID prefixes follow SPEC-API-001 §4: ULID strings with type prefixes.
const (
	IDPrefixGatewayKey       = "gky_"
	IDPrefixUpstreamKey      = "uky_"
	IDPrefixUpstreamEndpoint = "ep_"
	IDPrefixCombo            = "cmb_"
	IDPrefixProxy            = "prx_"
)

// GatewayKey is an aggregate root. Fields are unexported on purpose
// (AGENTS.md §2.2 Rich Domain Models): every transition goes through a method.
type GatewayKey struct {
	id           string
	name         string
	valueHash    string
	keyHint      string
	status       GatewayKeyStatus
	lastUsedAt   *time.Time
	requestCount int64
	createdAt    time.Time
	revokedAt    *time.Time
}

// NewGatewayKey is the only constructor for a freshly issued key. Callers pass
// the plaintext once; the domain keeps the digest and the masking hint only.
// NewGatewayKey is the only constructor for a freshly issued key. Callers pass
// the plaintext once; the domain keeps the digest, and the caller computes the
// masking hint (the visible prefix is configurable, so it is a service concern).
func NewGatewayKey(name, plaintext, keyHint string, now time.Time) GatewayKey {
	return GatewayKey{
		id:        IDPrefixGatewayKey + NewULID(now),
		name:      name,
		valueHash: HashKey(plaintext),
		keyHint:   keyHint,
		status:    GatewayKeyActive,
		createdAt: now,
	}
}

// Rehydrate rebuilds a stored row into an aggregate. It is intended for the
// repository load path only; never use it to mint a new key.
func RehydrateGatewayKey(
	id, name, valueHash, keyHint string,
	status GatewayKeyStatus,
	lastUsedAt *time.Time,
	requestCount int64,
	createdAt time.Time,
	revokedAt *time.Time,
) GatewayKey {
	return GatewayKey{
		id:           id,
		name:         name,
		valueHash:    valueHash,
		keyHint:      keyHint,
		status:       status,
		lastUsedAt:   lastUsedAt,
		requestCount: requestCount,
		createdAt:    createdAt,
		revokedAt:    revokedAt,
	}
}

// Accessors expose the state the response layer needs without allowing mutation.
func (k GatewayKey) ID() string               { return k.id }
func (k GatewayKey) Name() string             { return k.name }
func (k GatewayKey) ValueHash() string        { return k.valueHash }
func (k GatewayKey) KeyHint() string          { return k.keyHint }
func (k GatewayKey) Status() GatewayKeyStatus { return k.status }
func (k GatewayKey) LastUsedAt() *time.Time   { return k.lastUsedAt }
func (k GatewayKey) RequestCount() int64      { return k.requestCount }
func (k GatewayKey) CreatedAt() time.Time     { return k.createdAt }
func (k GatewayKey) RevokedAt() *time.Time    { return k.revokedAt }

// Rename applies the PATCH /gateway-keys/{id} name change after validation.
func (k *GatewayKey) Rename(name string) error {
	if name == "" {
		return NewValidationError("name must not be empty")
	}
	if len(name) > 120 {
		return NewValidationError("name must be at most 120 characters")
	}
	k.name = name
	return nil
}

// Transition applies a status change from a PATCH body. Revocation is a
// terminal state: once revoked, a key cannot be revived.
func (k *GatewayKey) Transition(next GatewayKeyStatus) error {
	if !next.IsValid() {
		return NewValidationError("invalid status: " + string(next))
	}
	if k.status == GatewayKeyRevoked {
		return ErrGatewayKeyRevoked
	}
	if next == GatewayKeyRevoked {
		return NewValidationError("use Revoke to revoke a gateway key")
	}
	k.status = next
	return nil
}

// Revoke soft-deletes the key. It is terminal and records the instant.
func (k *GatewayKey) Revoke(now time.Time) error {
	if k.status == GatewayKeyRevoked {
		return ErrGatewayKeyRevoked
	}
	k.status = GatewayKeyRevoked
	t := now
	k.revokedAt = &t
	return nil
}

// GatewayKeyStatus is the lifecycle state of a gateway key, as a value object.
type GatewayKeyStatus string

const (
	GatewayKeyActive   GatewayKeyStatus = "active"
	GatewayKeyRevoked  GatewayKeyStatus = "revoked"
	GatewayKeyDisabled GatewayKeyStatus = "disabled"
)

// ParseGatewayKeyStatus validates a wire value before it reaches the entity.
func ParseGatewayKeyStatus(s string) (GatewayKeyStatus, error) {
	st := GatewayKeyStatus(s)
	if !st.IsValid() {
		return "", NewValidationError("invalid status: " + s)
	}
	return st, nil
}

// IsValid reports whether s is one of the statuses the API accepts.
func (s GatewayKeyStatus) IsValid() bool {
	switch s {
	case GatewayKeyActive, GatewayKeyRevoked, GatewayKeyDisabled:
		return true
	default:
		return false
	}
}
