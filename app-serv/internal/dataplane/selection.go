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

// BudgetGate reports whether an endpoint has already spent its budget. It is a
// one-method seam because selection asks exactly one question, and it keeps the
// router from importing the quota repository (AGENTS.md §1.5).
//
// The second return being false means "allowed", which is also what an endpoint
// with no cap answers: an uncapped endpoint is not an exhausted one.
type BudgetGate interface {
	// Exhausted reports whether the endpoint's month-to-date spend has reached
	// its stored budget cap (SPEC-API-001 §7.12).
	Exhausted(ctx context.Context, endpointID string) (bool, error)
}

// CredentialStrategy answers the credential rotation policy for one provider
// (SPEC-API-001 §7.5): fill-first, or round-robin with a sticky limit. It is a
// one-question seam, and it keeps the data plane from importing the settings
// service (AGENTS.md §1.5). A policy that cannot be read degrades to fill-first
// rather than failing the request, because rotation is an optimisation.
type CredentialStrategy interface {
	RotationPolicy(ctx context.Context, providerID string) (domain.RotationPolicy, error)
}

// Selector picks the endpoint and key for a provider (SPEC-API-001 §7.5).
type Selector struct {
	endpoints  repository.EndpointRepository
	opener     SecretOpener
	cursor     CursorStore
	gates      BudgetGate
	strategies CredentialStrategy
	// registry answers the one question the virtual-endpoint rule asks: whether
	// this provider needs no credential. It is optional, and without it a
	// provider with no stored endpoint is simply unavailable.
	registry RegistryReader
	clock    func() time.Time
}

// SelectorDeps holds the collaborators selection needs.
type SelectorDeps struct {
	Endpoints  repository.EndpointRepository
	Opener     SecretOpener
	Cursor     CursorStore
	Strategies CredentialStrategy
	// Gate is the optional §7.12 budget check. A nil one selects every
	// configured endpoint, which is the behaviour a deployment without quota
	// caps had before the check existed.
	Gate BudgetGate
	// Registry supplies the provider entries the virtual-endpoint rule reads. A
	// nil one disables the rule, which is the behaviour every deployment had
	// before it existed.
	Registry RegistryReader
}

// NewSelector validates deps and returns a selector. The opener is optional so a
// credential-free provider can still be routed; selecting a key that needs
// decryption without one fails rather than sending a sealed value upstream. The
// strategy seam is optional too: without it the walk is fill-first, the
// documented default and the degradation a failed policy read gets.
func NewSelector(deps SelectorDeps) (*Selector, error) {
	if deps.Endpoints == nil {
		return nil, domain.NewValidationError("endpoint repository is required")
	}
	return &Selector{
		endpoints: deps.Endpoints, opener: deps.Opener, cursor: deps.Cursor, gates: deps.Gate,
		strategies: deps.Strategies, registry: deps.Registry, clock: time.Now,
	}, nil
}

// Select returns the first available endpoint that can serve a request: the
// no-history form of SelectNext, for callers that make one attempt and have
// nothing to exclude (media, embeddings). The chat engine walks candidates
// through SelectNext instead.
func (s *Selector) Select(ctx context.Context, providerID string) (Selection, error) {
	return s.SelectNext(ctx, providerID, nil)
}

// overBudget reports whether an endpoint must be skipped for budget. A gate read
// that fails is treated as "not over budget", because a quota lookup is a
// bookkeeping question and a control-plane outage must not take the data plane
// down: the alternative would let a quota table lock every provider out.
//
// The direction is deliberate and stated rather than implied: failing closed here
// would turn a transient read error into a gateway-wide outage, while failing
// open costs at most one request beyond a cap the operator set.
func (s *Selector) overBudget(ctx context.Context, endpointID string) bool {
	if s.gates == nil || endpointID == "" {
		return false
	}
	exhausted, err := s.gates.Exhausted(ctx, endpointID)
	if err != nil {
		return false
	}
	return exhausted
}

// HasBudgetGate reports whether a gate is wired. It exists so the composition
// root and a test can state the wiring without reaching into the selector's
// fields, and so a typed-nil gate — an interface holding a nil pointer, which
// is not itself nil — is caught by the caller rather than at the first request.
func (s *Selector) HasBudgetGate() bool { return s.gates != nil }

// candidates loads the provider's active endpoints in priority order. A tie on
// priority is broken by id, so the order is stable across calls.
func (s *Selector) candidates(ctx context.Context, providerID string) ([]domain.UpstreamEndpoint, error) {
	found, _, err := s.endpoints.List(ctx, repository.EndpointFilter{
		ProviderID: providerID,
		Status:     string(domain.UpstreamEndpointActive),
	}, repository.PageQuery{Page: 1, PerPage: MaxEndpointsPerProvider})
	// A credential-free provider with no stored row answers on the virtual
	// endpoint, which is how a free lane becomes usable the moment its provider
	// is listed (the reference injects the same connection).
	found, err = s.virtualCandidates(providerID, found, err)
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

// offset asks the cursor where to start. It is only consulted under round-robin:
// fill-first reads neither the cursor nor the sticky limit, so it leaves no
// rotation state behind. A Redis outage must not take the data plane down, so a
// failure here falls back to plain priority order instead of failing the request.
func (s *Selector) offset(ctx context.Context, providerID string, size, stickyLimit int) int {
	if s.cursor == nil || size <= 1 {
		return 0
	}
	next, err := s.cursor.NextOffset(ctx, providerID, size, stickyLimit)
	if err != nil || next < 0 {
		return 0
	}
	return next % size
}

// rotationPolicy resolves the credential walk for one provider. A nil seam and a
// failed read both degrade to fill-first: the request is served in priority order
// rather than failed, like a cursor Redis cannot answer.
func (s *Selector) rotationPolicy(ctx context.Context, providerID string) domain.RotationPolicy {
	if s.strategies == nil {
		return domain.RotationPolicy{Strategy: domain.RotationFillFirst}
	}
	policy, err := s.strategies.RotationPolicy(ctx, providerID)
	if err != nil {
		return domain.RotationPolicy{Strategy: domain.RotationFillFirst}
	}
	return policy
}
