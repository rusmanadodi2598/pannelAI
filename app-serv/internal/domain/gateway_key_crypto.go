// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/gateway_key_crypto.go
// @for       The digest helper for gateway key secrets.
// @uses      crypto/sha256, encoding/hex (standard library only).
// @reason    A gateway key is stored only as a SHA-256 lookup digest
//
//	(SPEC-API-001 §6); the plaintext is shown once and compared by
//	hashing the presented secret. Masking is a service-layer concern
//	because the visible prefix is configurable (GATEWAY_KEY_PREFIX).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-16
package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashKey returns the SHA-256 hex digest stored in gateway_keys.value_hash.
// Lookups hash the presented secret and compare digests, so plaintext is never
// retrievable from storage (SPEC-API-001 §6).
func HashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
