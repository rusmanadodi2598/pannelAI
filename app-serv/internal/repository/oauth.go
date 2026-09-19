// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/oauth.go
// @for       The OAuth state replay-guard boundary (SPEC-API-001 §4, §7.4).
// @uses      context, time.
// @reason    §4 makes the OAuth `state` single-use with a 10-minute TTL, so a
//
//	callback replayed or guessed cannot mint tokens. The guard is a
//	storage concern the flow service depends on, never a driver: an
//	in-memory implementation backs the tests, Redis backs production.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package repository

import (
	"context"
	"errors"
	"time"
)

// ErrStateAlreadyStaged reports a Stage call whose state value is already in
// flight. It is a domain refusal rather than a driver error so the service can
// map it to VALIDATION_ERROR instead of a 500.
var ErrStateAlreadyStaged = errors.New("oauth state is already staged")

// OAuthStateStore is the single-use staging area for an in-flight OAuth
// authorization: the state value maps to the flow's private context (provider,
// PKCE verifier, redirect) until the callback consumes it exactly once.
type OAuthStateStore interface {
	// Stage records the state's payload and refuses a value already staged.
	Stage(ctx context.Context, state string, payload []byte, ttl time.Duration) error
	// Take removes and returns the state's payload. A state that never existed,
	// expired, or was already taken reports ok=false — the replay answer.
	Take(ctx context.Context, state string) (payload []byte, ok bool, err error)
}
