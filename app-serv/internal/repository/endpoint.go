// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/endpoint.go
// @for       Storage boundaries for upstream endpoints, their keys, and custom
//
//	provider nodes.
//
// @uses      context, internal/domain.
// @reason    AGENTS.md §1.5 requires services to depend on these interfaces and
//
//	never on a driver, and §2.2 requires repositories to save aggregate
//	roots: the endpoint (with its keys) and the node are the roots here,
//	so a key is never loaded or stored on its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package repository

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// EndpointFilter narrows an endpoint list. A zero value means "no filter",
// which is distinct from a filter matching nothing, so a caller can list all.
type EndpointFilter struct {
	ProviderID string
	Status     string
}

// EndpointRepository is the storage boundary for upstream endpoints and their
// keys (SPEC-API-001 §7.5). An implementation MUST save the aggregate root —
// endpoint and keys together — so a partially written account cannot exist.
type EndpointRepository interface {
	// Create persists a new endpoint with its keys in one transaction. A
	// duplicate (provider_id, label) must yield domain.ErrEndpointExists so the
	// service can map it to CONFLICT.
	Create(ctx context.Context, endpoint domain.UpstreamEndpoint) error

	// List returns one page of endpoints, ordered by provider then priority, so
	// the ordering the router reads is the ordering the panel shows.
	List(ctx context.Context, filter EndpointFilter, q PageQuery) (endpoints []domain.UpstreamEndpoint, total int64, err error)

	// GetByID loads one endpoint with its keys. A missing row must yield
	// domain.ErrEndpointNotFound.
	GetByID(ctx context.Context, id string) (domain.UpstreamEndpoint, error)

	// Update persists the endpoint's own fields. Keys are untouched: they have
	// their own methods because a key change is a different concern.
	Update(ctx context.Context, endpoint domain.UpstreamEndpoint) error

	// Delete removes the endpoint and, by cascade, its keys.
	Delete(ctx context.Context, id string) error

	// AddKey persists a new key under an existing endpoint.
	AddKey(ctx context.Context, endpointID string, key domain.UpstreamKey) error

	// UpdateKey persists a key's mutable fields and, when the sealed value is
	// non-empty, its replacement credential.
	UpdateKey(ctx context.Context, key domain.UpstreamKey) error

	// DeleteKey removes one key.
	DeleteKey(ctx context.Context, endpointID, keyID string) error

	// RecordKeyHealth persists a key's circuit-breaker state after an upstream
	// call, without touching its label, priority, or credential.
	RecordKeyHealth(ctx context.Context, key domain.UpstreamKey) error

	// Reorder assigns new priorities to every endpoint of one provider, so a
	// priority change cannot leave two endpoints claiming the same slot.
	Reorder(ctx context.Context, providerID string, orderedIDs []string) error
}

// NodeRepository is the storage boundary for custom provider nodes
// (SPEC-API-001 §7.4).
type NodeRepository interface {
	// Create persists a node. A duplicate prefix must yield
	// domain.ErrNodePrefixTaken so the service can map it to CONFLICT.
	Create(ctx context.Context, node domain.ProviderNode) error

	// List returns every node, ordered by prefix so the list is stable.
	List(ctx context.Context) ([]domain.ProviderNode, error)

	// GetByID loads one node. A missing row must yield domain.ErrNodeNotFound.
	GetByID(ctx context.Context, id string) (domain.ProviderNode, error)

	// Update persists a node's mutable fields.
	Update(ctx context.Context, node domain.ProviderNode) error

	// Delete removes a node.
	Delete(ctx context.Context, id string) error

	// CountEndpoints reports how many endpoints reference the node's provider id,
	// so a delete can refuse while one still does.
	CountEndpoints(ctx context.Context, providerID string) (int64, error)
}
