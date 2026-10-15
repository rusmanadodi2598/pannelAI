// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_reads.go
// @for       The read surface of an UpstreamKey: every stored field the router, the repository and the panel are allowed to see.
// @uses      time.
// @reason    A key carries eighteen stored facts and three of them (the runs per failure class, the selection stamp, the failure class) exist only because rotation and parking needed them, so the read list grows while the transitions do not. Keeping the accessors in their own file is what keeps the entity's mutation rules on screen together, and there is deliberately no accessor for the plaintext value: only EncryptedValue is readable, and only the repository and the transport see it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-14
package domain

import "time"

func (k UpstreamKey) ID() string                   { return k.id }
func (k UpstreamKey) EndpointID() string           { return k.endpointID }
func (k UpstreamKey) Label() string                { return k.label }
func (k UpstreamKey) EncryptedValue() string       { return k.valueEncrypted }
func (k UpstreamKey) Hint() string                 { return k.keyHint }
func (k UpstreamKey) Priority() int                { return k.priority }
func (k UpstreamKey) Status() UpstreamKeyStatus    { return k.status }
func (k UpstreamKey) LastUsedAt() *time.Time       { return k.lastUsedAt }
func (k UpstreamKey) LastSelectedAt() *time.Time   { return k.lastSelectedAt }
func (k UpstreamKey) LastError() string            { return k.lastError }
func (k UpstreamKey) LastErrorClass() string       { return k.lastErrorClass }
func (k UpstreamKey) ConsecutiveErrors() int       { return k.consecutiveErrors }
func (k UpstreamKey) ConsecutiveAuth() int         { return k.consecutiveAuth }
func (k UpstreamKey) ConsecutiveRateLimit() int    { return k.consecutiveRateLimit }
func (k UpstreamKey) ConsecutiveBilling() int      { return k.consecutiveBilling }
func (k UpstreamKey) RateLimitedUntil() *time.Time { return k.rateLimitedUntil }
func (k UpstreamKey) ResetSource() string          { return k.resetSource }
func (k UpstreamKey) SignatureID() string          { return k.signatureID }
func (k UpstreamKey) CreatedAt() time.Time         { return k.createdAt }
func (k UpstreamKey) UpdatedAt() time.Time         { return k.updatedAt }
