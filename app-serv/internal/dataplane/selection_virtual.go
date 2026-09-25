// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/selection_virtual.go
// @for       The virtual credential-free endpoint: how a provider that needs no
//
//	account answers when the operator configured none.
//
// @uses      internal/domain, internal/provider, context, strings.
// @reason    The reference injects a virtual connection for every `noAuth`
//
//	provider (src/sse/services/auth.js:45-63, `accessToken: "public"`), so
//	a free lane is usable the moment its provider is listed. This port
//	required a stored row instead: with none, selection answered
//	NO_PROVIDER_AVAILABLE and the operator had to invent an endpoint
//	before the free tier could be reached (draft 029 §4.8, F8).
//
//	The injection is deliberately narrow. It applies only when the provider
//	needs no credential AND the operator stored no endpoint for it, so a
//	stored row always wins and a provider that wants a key is never
//	invented. The endpoint it builds carries no key, which is what makes
//	the health writes a no-op and the credential the connector's own
//	public bearer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// VirtualEndpointIDPrefix names a synthesized endpoint. The prefix is what makes
// one recognizable in a log or an error message, and it cannot collide with a
// stored row because a ULID never starts with it.
const VirtualEndpointIDPrefix = "virtual:"

// VirtualEndpointLabel is the label a synthesized endpoint carries.
const VirtualEndpointLabel = "Public (no credential)"

// RegistryReader is the one question the virtual-endpoint rule asks the provider
// registry: whether this provider answers without a credential. It is a seam
// rather than a *registry.Index because the composition root overlays stored
// custom nodes, and a node the operator created must be answerable by the same
// rule.
type RegistryReader interface {
	Provider(name string) (registry.Provider, bool)
}

// virtualEndpoint builds the endpoint a credential-free provider answers on when
// the operator configured none. It reports false when the provider is unknown,
// needs a credential, or has a stored endpoint, because in each of those cases
// inventing one would either guess at an account or shadow the operator's own.
func virtualEndpoint(reg RegistryReader, providerID string, stored []domain.UpstreamEndpoint, now time.Time) (domain.UpstreamEndpoint, bool) {
	if len(stored) > 0 || reg == nil {
		return domain.UpstreamEndpoint{}, false
	}
	entry, found := reg.Provider(providerID)
	if !found || !entry.NeedsNoCredential() {
		return domain.UpstreamEndpoint{}, false
	}
	endpoint, err := domain.NewUpstreamEndpoint(
		VirtualEndpointIDPrefix+entry.ID, entry.ID, VirtualEndpointLabel,
		domain.UpstreamAuthNone, 1, now,
	)
	if err != nil {
		// A provider id that cannot name an endpoint is a registry problem, not
		// a request problem: reporting it here would turn every request into a
		// validation error, so the rule declines and selection answers the
		// NO_PROVIDER_AVAILABLE it would have answered before.
		return domain.UpstreamEndpoint{}, false
	}
	return endpoint, true
}

// virtualCandidates returns the provider's stored endpoints, or the single
// synthesized one when it has none and needs none. It takes no context because
// it reads nothing: the caller already performed the repository read and passes
// its result here, so a failure is forwarded rather than retried.
func (s *Selector) virtualCandidates(
	providerID string,
	stored []domain.UpstreamEndpoint,
	readErr error,
) ([]domain.UpstreamEndpoint, error) {
	if readErr != nil {
		return nil, readErr
	}
	if len(stored) > 0 {
		return stored, nil
	}
	if s.registry == nil {
		return stored, nil
	}
	endpoint, ok := virtualEndpoint(s.registry, providerID, stored, s.clock())
	if !ok {
		return stored, nil
	}
	return []domain.UpstreamEndpoint{endpoint}, nil
}
