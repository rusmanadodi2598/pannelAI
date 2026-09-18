// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/selection.go
// @for       Candidate endpoint ordering, the key routing may spend, and the
//
//	health record written back after an upstream outcome.
//
// @uses      internal/domain, internal/provider, internal/repository.
// @reason    SPEC-API-001 §7.5 fixes the rule (endpoint by priority, then a
//
//	healthy key inside it, circuit-broken keys skipped). It is policy
//	the whole data plane depends on, so it lives in one place: this
//	file consults the circuit state the domain owns and writes it back
//	through RecordKeyHealth, so the panel's answer and the router's
//	answer cannot diverge and no second health model exists.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// CursorKeyPrefix is the Redis key space for the round-robin endpoint cursor.
// The cursor is the only selection state Redis holds: endpoint availability and
// key health live in the domain and in PostgreSQL, so losing this namespace
// degrades rotation to priority order without changing which account is usable.
const CursorKeyPrefix = "pannelai:dataplane:endpoint-cursor:"

// MaxEndpointsPerProvider bounds the candidate list. A provider with more
// accounts than this is a configuration mistake, and the bound keeps selection
// from reading an unbounded result set (AGENTS.md §1.7).
const MaxEndpointsPerProvider = 100

// CursorStore advances the round-robin position over one provider's endpoints.
// It is an interface because the data plane must not import a Redis client
// (AGENTS.md §1.5 keeps driver access in the repository layer), so the
// composition root adapts the Redis implementation to it.
type CursorStore interface {
	// NextOffset returns the zero-based offset to start selecting at, then moves
	// the cursor on. A sticky budget keeps one offset for a bounded number of
	// calls, so one account serves a short burst instead of the pool
	// alternating on every request.
	NextOffset(ctx context.Context, providerID string, size int, stickyLimit int) (int, error)
}

// CursorKey returns the Redis key the cursor for one provider occupies. The
// provider id is hashed so an id with odd characters cannot collide with
// another key in the namespace.
func CursorKey(providerID string) string {
	digest := sha256.Sum256([]byte(providerID))
	return CursorKeyPrefix + hex.EncodeToString(digest[:])
}

// Selection is one usable endpoint, the key routing picked inside it, and the
// credential that key stands for.
type Selection struct {
	Endpoint domain.UpstreamEndpoint
	Key      domain.UpstreamKey
	// Credential is assembled for exactly one request and never stored, so a
	// logged selection cannot leak a secret.
	Credential provider.Credential
}

// Selector picks the endpoint and key for a provider (SPEC-API-001 §7.5).
type Selector struct {
	endpoints repository.EndpointRepository
	opener    SecretOpener
	cursor    CursorStore
	clock     func() time.Time
	// stickyLimit is how many consecutive requests one endpoint serves before
	// rotation moves on.
	stickyLimit int
}

// SelectorDeps holds the collaborators selection needs.
type SelectorDeps struct {
	Endpoints   repository.EndpointRepository
	Opener      SecretOpener
	Cursor      CursorStore
	StickyLimit int
}

// NewSelector validates deps and returns a selector. The opener is optional so a
// credential-free provider can still be routed; selecting a key that needs
// decryption without one fails rather than sending a sealed value upstream.
func NewSelector(deps SelectorDeps) (*Selector, error) {
	if deps.Endpoints == nil {
		return nil, domain.NewValidationError("endpoint repository is required")
	}
	limit := deps.StickyLimit
	if limit < 1 {
		limit = 1
	}
	return &Selector{
		endpoints: deps.Endpoints, opener: deps.Opener, cursor: deps.Cursor,
		clock: time.Now, stickyLimit: limit,
	}, nil
}

// Select returns the first available endpoint that owns a usable key, in
// priority order, rotated by the cursor when one is configured.
func (s *Selector) Select(ctx context.Context, providerID string) (Selection, error) {
	now := s.clock()
	endpoints, err := s.candidates(ctx, providerID)
	if err != nil {
		return Selection{}, err
	}
	if len(endpoints) == 0 {
		return Selection{}, domain.NewNoProviderAvailableError(
			"no upstream endpoint is configured for provider " + providerID)
	}

	offset := s.offset(ctx, providerID, len(endpoints))
	for i := range endpoints {
		endpoint := endpoints[(offset+i)%len(endpoints)]
		if !endpoint.Available(now) {
			continue
		}
		key, ok := endpoint.NextKey(now)
		if !ok {
			continue
		}
		credential, err := s.credential(endpoint, key)
		if err != nil {
			return Selection{}, err
		}
		return Selection{Endpoint: endpoint, Key: key, Credential: credential}, nil
	}
	return Selection{}, domain.NewNoProviderAvailableError("every upstream endpoint for provider " +
		providerID + " is unavailable or has no usable key")
}

// candidates loads the provider's active endpoints in priority order. A tie on
// priority is broken by id, so the order is stable across calls.
func (s *Selector) candidates(ctx context.Context, providerID string) ([]domain.UpstreamEndpoint, error) {
	found, _, err := s.endpoints.List(ctx, repository.EndpointFilter{
		ProviderID: providerID,
		Status:     string(domain.UpstreamEndpointActive),
	}, repository.PageQuery{Page: 1, PerPage: MaxEndpointsPerProvider})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(found, func(a, b int) bool {
		if found[a].Priority() != found[b].Priority() {
			return found[a].Priority() < found[b].Priority()
		}
		return found[a].ID() < found[b].ID()
	})
	return found, nil
}

// offset asks the cursor where to start.
//
// Rotation is an optimisation, not a correctness input: a Redis outage must not
// take the data plane down, so a failure here falls back to plain priority order
// instead of failing the request.
func (s *Selector) offset(ctx context.Context, providerID string, size int) int {
	if s.cursor == nil || size <= 1 {
		return 0
	}
	next, err := s.cursor.NextOffset(ctx, providerID, size, s.stickyLimit)
	if err != nil || next < 0 {
		return 0
	}
	return next % size
}

// RecordSuccess applies a served request to the key and persists its health, so
// the circuit the domain owns is what changes and nothing else does.
func (s *Selector) RecordSuccess(ctx context.Context, selection Selection) error {
	updated, err := selection.Endpoint.RecordKeySuccess(selection.Key.ID(), s.clock())
	if err != nil {
		return err
	}
	return s.endpoints.RecordKeyHealth(ctx, updated)
}

// RecordFailure applies a failed attempt to the key and persists its health, so
// the circuit the domain owns is what changes and nothing else does.
func (s *Selector) RecordFailure(ctx context.Context, selection Selection, reason string) error {
	updated, err := selection.Endpoint.RecordKeyFailure(selection.Key.ID(), reason, s.clock())
	if err != nil {
		return err
	}
	return s.endpoints.RecordKeyHealth(ctx, updated)
}

// PersistKey records a key whose health was already updated in memory, so a
// failure discovered after the response body was read needs no second mutation.
func (s *Selector) PersistKey(ctx context.Context, key domain.UpstreamKey) error {
	return s.endpoints.RecordKeyHealth(ctx, key)
}
