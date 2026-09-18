// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_ports.go
// @for       The collaborator contracts the endpoint and node services are built
//
//	from: registry lookup, credential sealing, probing, and the
//	aggregate store the frozen repository contract extends.
//
// @uses      internal/domain, internal/registry, internal/repository, context.
// @reason    repository.EndpointRepository is frozen and aggregate-scoped. The
//
//	batch routes it backs are all-or-nothing across a set of rows, which a
//	per-aggregate contract cannot express, so the set operations are named
//	here as EndpointStore — the frozen interface embedded, not replaced —
//	and the concrete PostgreSQL repository implements them. The remaining
//	ports keep the service testable without a database, a registry, a
//	sealer, or an HTTP upstream (AGENTS.md §1.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ProviderIndex resolves a provider identifier, so the service can refuse a
// provider_id the registry does not know (SPEC-API-001 §6: an unknown provider id
// is a VALIDATION_ERROR). All and Categories are part of the contract because
// §7.4 lists and filters the registry, and a list that could only look up ids
// one at a time would force every caller to invent its own enumeration.
//
// It is an interface rather than a concrete *registry.Index because the runtime
// view must include custom nodes created during the process's life: a fixed index
// value would freeze the registry at boot and make a node just created through
// POST /provider-nodes unusable as a provider_id.
type ProviderIndex interface {
	// Provider resolves an id, alias, or node prefix to its entry.
	Provider(name string) (registry.Provider, bool)
	// All returns every entry, embedded and custom.
	All() []registry.Provider
	// Categories returns the measured category set.
	Categories() []string
}

// SecretSealer seals a credential before the aggregate stores it, so no layer
// above the service ever holds the plaintext and the aggregate never meets it
// (SPEC-API-001 §6). domain.Sealer is the one implementation.
type SecretSealer interface {
	Seal(plaintext string) (string, error)
}

// SecretOpener opens a sealed credential, for the one management path that needs
// it: probing an existing key to answer "does this credential work". The opened
// value lives for the duration of one probe and is never returned to a caller.
type SecretOpener interface {
	Open(sealed string) (string, error)
}

// CredentialSealer is both halves, which is what domain.Sealer provides. The two
// one-method ports stay separate so a path that only seals cannot open.
type CredentialSealer interface {
	SecretSealer
	SecretOpener
}

// BulkRowIndexer is satisfied by a store error that can name the batch row it
// refused.
//
// It is declared structurally rather than as a shared type on purpose: the
// concrete error lives in the repository package, internal/repository is frozen
// for this vertical, and the service must not import internal/repository/postgres
// (AGENTS.md §1.5). A repository error that implements this method is therefore
// recognized without either layer depending on the other's types.
type BulkRowIndexer interface {
	// BulkRowIndex reports the offending row's zero-based position, or false
	// when the error cannot be attributed to one row.
	BulkRowIndex() (index int, ok bool)
}

// EndpointStore is the storage the endpoint service depends on: the frozen
// aggregate-scoped contract, embedded, plus the set operations a batch route
// needs.
//
// Embedding rather than restating the methods is deliberate — the frozen
// interface stays the single definition of aggregate-scoped persistence, so a
// change there becomes a compile error here instead of a silent drift.
type EndpointStore interface {
	repository.EndpointRepository

	// CreateBatch persists several endpoints with their keys in ONE transaction,
	// so a rejected row leaves the account list exactly as it was
	// (SPEC-API-001 §8.1: nothing is written unless every row passes). An
	// implementation SHOULD return an error satisfying BulkRowIndexer when it
	// can attribute the refusal to one row.
	CreateBatch(ctx context.Context, endpoints []domain.UpstreamEndpoint) error

	// AddKeys appends several keys to one endpoint in ONE transaction, on the
	// same all-or-nothing terms.
	AddKeys(ctx context.Context, endpointID string, keys []domain.UpstreamKey) error

	// IDsByProvider returns every endpoint id of the provider in the priority
	// order they currently hold, which is what renumbering siblings needs. The
	// frozen contract has no such read because it is not aggregate-scoped.
	IDsByProvider(ctx context.Context, providerID string) ([]string, error)

	// FindOAuthEndpoint returns the id of the OAuth endpoint whose stored account
	// identity matches the given email or workspace id, or
	// domain.ErrEndpointNotFound when the account has never been imported.
	// SPEC-API-001 §8.1 makes a re-import an update rather than a duplicate, and
	// the match has to happen in storage: comparing in the service would mean
	// reading every endpoint of the provider and hoping the account is on the
	// page.
	FindOAuthEndpoint(ctx context.Context, providerID, email, workspaceID string) (string, error)
}
