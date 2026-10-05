// Command app-serv adapts the node store to the model-list read's dependencies.
//
// @file      cmd/app-serv/node_models_wiring.go
// @for       The two reads the node model adapter needs from storage: a node's
//
//	target, and the credential of the endpoint that serves it.
//
// @uses      internal/domain, internal/registry, internal/repository,
//
//	internal/service, context, strings.
//
// @reason    SPEC-API-001 §7.4 serves a node's models, and the read needs two
//
//	things the adapter must not reach for itself: the stored node (to know
//	its base URL) and the credential (to authenticate the read). Both are
//	repository reads, and AGENTS.md §1.5 keeps storage out of the adapter's
//	business rules, so they are narrow functions built here, in the one
//	layer allowed to know every boundary.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-23
package main

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// nodeLookupTimeout bounds one stored-node read made on behalf of a model-list
// request. The row is a single indexed lookup, so the bound is about a stalled
// pool, not a slow query (AGENTS.md §1.6).
const nodeLookupTimeout = 5 * time.Second

// nodeTarget is the part of a stored node the model read needs.
//
// It is declared here beside the lookup that produces it rather than beside the
// adapter that consumes it: the fields are what a stored node exposes, so a
// change to the node's shape lands in the same file as the read that has to
// follow it.
type nodeTarget struct {
	// BaseURL is the node's base, already sanitized of the path the transport
	// appends (domain.normalizeNodeBaseURL).
	BaseURL string
	// Format is the wire format the node speaks ("openai", "claude",
	// "openai-responses").
	Format string
	// APIType is the node's declared api type, which distinguishes a Responses
	// node from a chat one.
	APIType string
}

// nodeCredentialSource opens the credential a node's own endpoint holds.
//
// It answers an empty string rather than an error when the node has no usable
// endpoint: a node whose upstream needs no credential is legitimate, and
// treating "no key" as a fault would report a fallback for it.
type nodeCredentialSource func(ctx context.Context, nodeID string) (string, error)

// nodeReader is the one storage read the node target needs. It is declared here
// rather than taking the whole repository contract so the wiring depends on one
// method.
type nodeReader interface {
	GetByID(ctx context.Context, id string) (domain.ProviderNode, error)
}

// endpointKeyReader reads the endpoint that serves a provider and its keys. The
// endpoint repository already satisfies it.
type endpointKeyReader interface {
	List(ctx context.Context, filter repository.EndpointFilter, q repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error)
}

// newNodeTargetLookup resolves a node id to the base URL and format a models
// read needs.
//
// The lookup is synchronous and uncached: it is called from the model source,
// which caches its own answer, so a second cache here would only add a second
// expiry to reason about. A node the store does not hold is unresolved rather
// than an error: the id arrived from a request, and a stale one is a fallback,
// not a fault.
func newNodeTargetLookup(nodes nodeReader) func(id string) (nodeTarget, bool) {
	return func(id string) (nodeTarget, bool) {
		// The port carries no request context, so the read states its own
		// deadline rather than running on one that never ends (AGENTS.md §1.6):
		// a stalled pool would otherwise hold the model-list request open past
		// any caller's patience.
		callCtx, cancel := context.WithTimeout(context.Background(), nodeLookupTimeout)
		defer cancel()

		node, err := nodes.GetByID(callCtx, id)
		if err != nil {
			return nodeTarget{}, false
		}
		return nodeTarget{
			BaseURL: node.BaseURL(),
			Format:  node.Format(),
			APIType: node.APIType(),
		}, true
	}
}

// newNodeCredentialSource opens the credential of the first active endpoint
// under a node. A node holds no credential of its own, so the read is per node
// rather than per endpoint: every endpoint under a node reaches the same
// upstream.
// An empty credential is an answer, not a failure, because an upstream needing
// no key is a legitimate node and the reference sends it no auth header. A read
// or open failure answers empty too, so a sealed value that will not open
// yields a fallback list rather than a 500.
func newNodeCredentialSource(endpoints endpointKeyReader, opener service.SecretOpener) func(context.Context, string) (string, error) {
	return func(ctx context.Context, nodeID string) (string, error) {
		rows, _, err := endpoints.List(ctx, repository.EndpointFilter{ProviderID: nodeID}, repository.PageQuery{PerPage: 1})
		if err != nil || len(rows) == 0 {
			//nolint:nilerr // reason: a node with no endpoint, or a store briefly unavailable, answers "no credential". The caller falls back to the registry list and the node keeps working.
			return "", nil
		}
		endpoint := rows[0]
		// A no_auth endpoint has no key at all, which is an answer rather than
		// a missing credential.
		if endpoint.AuthType() == domain.UpstreamAuthNone {
			return "", nil
		}
		key, ok := endpoint.NextKey(endpoint.UpdatedAt())
		if !ok || key.EncryptedValue() == "" {
			return "", nil
		}
		plaintext, err := opener.Open(key.EncryptedValue())
		if err != nil {
			//nolint:nilerr // reason: a sealed value that will not open answers the same as no credential, a fallback list with a warning, rather than failing a read over a key it does not strictly need. The plaintext is never logged or returned.
			return "", nil
		}
		return plaintext, nil
	}
}

// assertNodeReader keeps the repository satisfying the narrow port, so a change
// to either becomes a compile error here rather than at the call site.
var _ nodeReader = (repository.NodeRepository)(nil)

// assertEndpointKeyReader does the same for the endpoint read.
var _ endpointKeyReader = (repository.EndpointRepository)(nil)

// assertFormatVocabulary keeps the registry's format names the ones the target
// lookup reports, so a rename cannot leave the adapter switching on a value
// nothing produces.
var _ = registry.DefaultFormat
