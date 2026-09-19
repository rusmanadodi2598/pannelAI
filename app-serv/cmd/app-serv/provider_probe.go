// Command app-serv adapts the connectivity probe port to HTTP.
//
// @file      cmd/app-serv/provider_probe.go
// @for       The net/http implementation of service.EndpointProber and
//
//	service.NodeProber.
//
// @uses      internal/dataplane, internal/domain, internal/netguard,
//
//	internal/provider, internal/registry, internal/service, net/http,
//	time.
//
// @reason    SPEC-API-001 §7.5 and §7.4 offer a connectivity test, and the probe
//
//	must reach the upstream — but AGENTS.md §1.5 forbids net/http in the
//	service layer. The port lives in `internal/service`, this adapter
//	lives in the composition root, and that split is what keeps the
//	service testable with a fake and the adapter free of business
//	rules. It reaches the upstream through the `provider.Plugin` seam,
//	so no probe special-cases a provider id. The destination goes
//	through the process's egress guard (OWASP A01): a node's base_url is
//	operator input, and a probe must not be the one dial that skips the
//	policy the data plane follows.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// probeConnectTimeout bounds the dial and TLS handshake, independently of the
// total budget the service imposes. A probe whose TCP connect never completes
// should fail in ten seconds rather than consume the whole window: the operator
// is waiting on a button, and "unreachable" is the answer they need.
const probeConnectTimeout = 10 * time.Second

// providerLookup is the one registry read the prober needs. It is an interface
// so the prober can be handed the composition root's runtime overlay: an
// endpoint belonging to a custom provider node must resolve to that node when
// the operator probes it, not to an unknown provider.
type providerLookup interface {
	Provider(name string) (registry.Provider, bool)
}

// httpEndpointProber probes an endpoint through the plugin seam.
type httpEndpointProber struct {
	index      providerLookup
	connectors *provider.Connectors
	guard      *netguard.Guard
	client     *http.Client
}

// newHTTPEndpointProber builds the adapter. Its only dependencies are the
// registry (to resolve a provider), the connectors (to authenticate), and the
// process's egress guard (to refuse a destination the operator's allowlist does
// not name), so it holds no repository and no sealer: the service hands over an
// opened credential for the duration of one call.
func newHTTPEndpointProber(index providerLookup, connectors *provider.Connectors, guard *netguard.Guard) *httpEndpointProber {
	// The probe shares the gateway's pool configuration and the guard's dialer,
	// so the address actually reached is the address that was validated.
	client := dataplane.NewHTTPClient(dataplane.HTTPClientDeps{
		Dialer: guard.NewDialer(probeConnectTimeout, 0),
	})
	// The per-call context carries the total budget; the client's own timeout
	// is a backstop so a hung connection cannot outlive it.
	client.Timeout = probeConnectTimeout + 5*time.Second
	return &httpEndpointProber{index: index, connectors: connectors, guard: guard, client: client}
}

// ProbeEndpoint asks the provider's own validation surface whether the given
// credential is accepted.
//
// The request targets `transport.validate_url` (the registry's model-list
// endpoint) rather than the chat path: a probe must not spend the operator's
// tokens or require a model name, and every provider that can chat can also
// list its models. When an entry declares no validate URL the probe reports a
// failure with that reason instead of guessing a path, because a guess would
// produce a confusing 404 that reads like a credential problem.
func (p *httpEndpointProber) ProbeEndpoint(ctx context.Context, endpoint domain.UpstreamEndpoint, key domain.UpstreamKey, credential string) (service.ProbeOutcome, error) {
	entry, connector, err := p.resolve(endpoint.ProviderID())
	if err != nil {
		return service.ProbeOutcome{}, err
	}

	target := entry.Transport.ValidateURL
	if target == "" {
		return failure("this provider does not declare a validation endpoint"), nil
	}

	// The credential family comes from the ENDPOINT, not from which variable the
	// caller happened to fill. A provider may route OAuth through one header and
	// static keys through another (claude reads x-api-key for a key and
	// Authorization for a token), so populating both fields would make the
	// header and the value disagree and report a working account as rejected.
	cred := p.credentialFor(endpoint, key, credential)
	return p.probe(ctx, target, entry, connector, cred)
}

// credentialFor builds the credential for one endpoint, naming exactly one
// family so nothing downstream has to guess which header it belongs in.
func (p *httpEndpointProber) credentialFor(endpoint domain.UpstreamEndpoint, key domain.UpstreamKey, credential string) provider.Credential {
	switch endpoint.AuthType() {
	case domain.UpstreamAuthNone:
		return provider.NoCredential(endpoint.ID())
	case domain.UpstreamAuthOAuth:
		return provider.OAuthToken(endpoint.ID(), key.ID(), credential)
	default:
		return provider.StaticKey(endpoint.ID(), key.ID(), credential)
	}
}

// ProbeNode asks whether a custom node's base URL answers with the supplied
// credential.
//
// The node's own base URL is the only address available — a custom node has no
// registry entry to take a validate URL from — so the probe appends the models
// path, which is the convention every OpenAI- and Anthropic-compatible server
// follows.
func (p *httpEndpointProber) ProbeNode(ctx context.Context, node domain.ProviderNode, credential string) (service.ProbeOutcome, error) {
	entry := registry.Provider{
		ID:        node.ID(),
		Category:  "apikey",
		AuthType:  registry.AuthAPIKey,
		Transport: registry.Transport{Format: node.Format(), BaseURL: node.BaseURL()},
	}
	if credential == "" {
		entry.AuthType = registry.AuthNone
	}

	target := node.BaseURL() + "/models"
	connector := p.connectors.For(entry)

	// A node is always a static key when it has one, and explicitly anonymous
	// when it does not, so neither case relies on an empty credential being
	// interpreted generously.
	cred := provider.NoCredential(node.ID())
	if credential != "" {
		cred = provider.StaticKey(node.ID(), "", credential)
	}
	return p.probe(ctx, target, entry, connector, cred)
}

// resolve finds the registry entry and connector for a provider id.
//
// A failure here is a real error, not a probe outcome: an unknown provider means
// the stored endpoint references something the registry does not know, which is
// a configuration fault rather than an upstream answer.
func (p *httpEndpointProber) resolve(providerID string) (registry.Provider, provider.Plugin, error) {
	entry, ok := p.index.Provider(providerID)
	if !ok {
		return registry.Provider{}, nil, fmt.Errorf("probe: provider %q is not in the registry", providerID)
	}
	return entry, p.connectors.For(entry), nil
}
