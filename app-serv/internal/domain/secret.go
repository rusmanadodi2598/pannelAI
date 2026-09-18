// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/secret.go
// @for       The one place a stored credential crosses between plaintext and
//
//	ciphertext: AES-256-GCM sealing for upstream keys and OAuth tokens.
//
// @uses      crypto/aes, crypto/cipher, crypto/rand, encoding/base64, fmt, strings.
// @reason    SPEC-API-001 §6 stores upstream credentials encrypted with a key
//
//	from the environment. There must be exactly one implementation:
//	sealing and opening have to agree on the key, the nonce
//	discipline, and the storage format, or a value written by one path
//	becomes unreadable by another, and a second implementation is
//	precisely how that happens. The aggregate itself never sees the
//	plaintext, so this type is what the service layer hands it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

// secretVersion prefixes every sealed value. It is part of the stored format so
// a future key rotation can tell an old ciphertext from a new one and refuse it
// explicitly, rather than attempting a decrypt that fails as an opaque integrity
// error.
const secretVersion = "v1"

// secretParts is how many colon-separated fields a sealed value has:
// version, nonce, ciphertext.
const secretParts = 3

// Sealer seals and opens stored credentials with AES-256-GCM.
//
// GCM is authenticated, so a tampered ciphertext is rejected instead of
// decrypted into plausible garbage. The nonce is generated per call and stored
// beside the ciphertext: reusing a nonce under one key is the failure that
// breaks GCM completely, so it is never derived from the plaintext or a counter
// shared across processes.
type Sealer struct {
	key []byte
}

// NewSealer builds a sealer from a 32-byte key.
//
// The length is re-checked here even though config.Config already validates it:
// this type is the last gate before a credential is written, and a short key
// silently selecting AES-128 is exactly the kind of weakening that should not
// depend on an env-parse test staying green.
func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != 32 {
		return nil, NewValidationError("encryption key must be exactly 32 bytes (AES-256)")
	}
	// The key is copied so a caller mutating its own slice cannot change the
	// sealer's key afterwards.
	owned := make([]byte, len(key))
	copy(owned, key)
	return &Sealer{key: owned}, nil
}

// Seal encrypts a plaintext into the stored form `v1:<nonce>:<ciphertext>`,
// where both parts are unpadded base64.
//
// An empty plaintext is sealed like any other value rather than rejected: an
// empty credential is a data problem the caller decides about, and a special
// case here would make "sealed empty" and "not sealed" indistinguishable.
func (s *Sealer) Seal(plaintext string) (string, error) {
	gcm, err := s.aead()
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		// A CSPRNG failure must abort: sealing with a weak or repeated nonce
		// would be worse than refusing to store the credential.
		return "", fmt.Errorf("domain: generating a nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return strings.Join([]string{
		secretVersion,
		base64.RawStdEncoding.EncodeToString(nonce),
		base64.RawStdEncoding.EncodeToString(sealed),
	}, ":"), nil
}

// Open decrypts a stored value, returning the plaintext.
//
// Every malformed shape is an error rather than a best effort: an unknown
// version prefix, a wrong field count, invalid base64, or a ciphertext shorter
// than the GCM tag all mean the stored value is not something this sealer wrote.
func (s *Sealer) Open(sealed string) (string, error) {
	parts := strings.Split(sealed, ":")
	if len(parts) != secretParts {
		return "", NewValidationError("stored secret is malformed")
	}
	if parts[0] != secretVersion {
		return "", NewValidationError("stored secret has an unsupported version")
	}

	nonce, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", NewValidationError("stored secret has an invalid nonce")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", NewValidationError("stored secret has an invalid body")
	}

	gcm, err := s.aead()
	if err != nil {
		return "", err
	}
	if len(nonce) != gcm.NonceSize() {
		return "", NewValidationError("stored secret has an invalid nonce length")
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// The wrapped error is deliberately not returned to a caller that might
		// render it: it distinguishes nothing a client should act on, and the
		// caller learns only that the value could not be opened.
		return "", NewValidationError("stored secret could not be decrypted")
	}
	return string(plaintext), nil
}

// aead builds the GCM cipher for this sealer's key.
func (s *Sealer) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("domain: building the cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("domain: building GCM: %w", err)
	}
	return gcm, nil
}

// MaskSecret renders the hint shown in list and detail responses for a stored
// credential: the leading characters the user recognises plus the last four,
// with the middle elided.
//
// It deliberately reveals no more than the family and the tail. A value too
// short to hide is returned unchanged, which is the honest result: masking it
// would reconstruct it entirely while looking concealed.
func MaskSecret(plaintext string) string {
	const head, tail = 3, 4
	if len(plaintext) <= head+tail {
		return plaintext
	}
	return plaintext[:head] + "\u2026" + plaintext[len(plaintext)-tail:]
}
