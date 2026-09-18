// Package domain holds entities, value objects, and gateway business rules.
//
// @file      internal/domain/session.go
// @for       Creates and verifies opaque HMAC-signed dashboard session tokens.
// @uses      crypto/hmac, crypto/rand, crypto/sha256, encoding/base64.
// @reason    Session credentials must be unforgeable while remaining opaque to
//
//	clients and independently revocable through a Redis digest.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-17
package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const sessionNonceBytes = 32

// NewSessionToken creates a signed token and the digest used for Redis state.
func NewSessionToken(secret []byte) (string, string, error) {
	nonce := make([]byte, sessionNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	return encodeSession(nonce, secret), digestSession(nonce), nil
}

// ParseSessionToken verifies a token and returns its Redis lookup digest.
func ParseSessionToken(token string, secret []byte) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return "", errors.New("invalid session token")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(nonce) != sessionNonceBytes {
		return "", errors.New("invalid session nonce")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(signature) != sha256.Size {
		return "", errors.New("invalid session signature")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(nonce)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return "", errors.New("invalid session signature")
	}
	return digestSession(nonce), nil
}

func encodeSession(nonce, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(nonce)
	return base64.RawURLEncoding.EncodeToString(nonce) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func digestSession(nonce []byte) string {
	digest := sha256.Sum256(nonce)
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
