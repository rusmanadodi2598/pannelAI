// Command app-serv adapts the connectivity probe port to HTTP.
//
// @file      cmd/app-serv/provider_probe.go
// @for       The net/http implementation of service.EndpointProber and
//
//	service.NodeProber.
//
// @uses      internal/domain, internal/provider, internal/registry,
//
//	internal/service, net/http, time.
//
// @reason    SPEC-API-001 §7.5 and §7.4 offer a connectivity test, and the probe
//
//	must reach the upstream — but AGENTS.md §1.5 forbids net/http in the
//	service layer. The port lives in `internal/service`, this adapter
//	lives in the composition root, and that split is what keeps the
//	service testable with a fake and the adapter free of business
//	rules. It reaches the upstream through the `provider.Plugin` seam,
//	so no probe special-cases a provider id.
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

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// probeConnectTimeout bounds the dial and TLS handshake, independently of the
// total budget the service imposes. A probe whose TCP connect never completes
// should fail in ten seconds rather than consume the whole window: the operator
// is waiting on a button, and "unreachable" is the answer they need.
const probeConnectTimeout = 10 * time.Second

// httpEndpointProber probes an endpoint through the plugin seam.
type httpEndpointProber struct {
	index      *registry.Index
	connectors *provider.Connectors
	client     *http.Client
}

// newHTTPEndpointProber builds the adapter. Its only dependencies are the
// registry (to resolve a provider) and the connectors (to authenticate), so it
// holds no repository and no sealer: the service hands over an opened credential
// for the duration of one call.
func newHTTPEndpointProber(index *registry.Index, connectors *provider.Connectors) *httpEndpointProber {
	return &httpEndpointProber{
		index:      index,
		connectors: connectors,
		client: &http.Client{
			// The per-call context carries the total budget; the client's own
			// timeout is a backstop so a hung connection cannot outlive it.
			Timeout: probeConnectTimeout + 5*time.Second,
		},
	}
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

// probe performs one authenticated request and classifies the outcome.
//
// Classification is the whole job: an upstream that answers with any 2xx or a
// 4xx that is not an authentication failure proves the host is reachable and the
// URL is right, while 401/403 proves the credential was checked and rejected.
// Only those two are reported as a credential failure, because reporting a 404
// as "your key is wrong" sends an operator to replace a working credential.
func (p *httpEndpointProber) probe(ctx context.Context, target string, entry registry.Provider, connector provider.Plugin, cred provider.Credential) (service.ProbeOutcome, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return service.ProbeOutcome{}, fmt.Errorf("probe %s: building the request: %w", entry.ID, err)
	}
	for name, value := range entry.Transport.Headers {
		req.Header.Set(name, value)
	}
	if err := connector.ApplyAuth(req, cred); err != nil {
		// An unusable credential is a probe answer, not an adapter fault: the
		// operator needs to see "no credential" in the panel.
		return failure(err.Error()), nil
	}

	started := time.Now()
	resp, err := p.client.Do(req)
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		// The call never completed, so there is no status; the message stays
		// short because a transport error can carry a long internal chain.
		return service.ProbeOutcome{
			State:     domain.EndpointTestFail,
			LatencyMS: latency,
			Message:   "the upstream could not be reached",
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	outcome := service.ProbeOutcome{LatencyMS: latency, Status: resp.StatusCode}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		outcome.State = domain.EndpointTestOK
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		outcome.State = domain.EndpointTestFail
		outcome.Message = "the upstream rejected this credential"
	default:
		// Reachable but unhappy: the host and URL are proven, which is what a
		// connectivity test asks, so the failure message names the status rather
		// than blaming the credential.
		outcome.State = domain.EndpointTestFail
		outcome.Message = fmt.Sprintf("the upstream answered %d", resp.StatusCode)
	}
	return outcome, nil
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

// failure builds a failed outcome with the given English explanation.
func failure(message string) service.ProbeOutcome {
	return service.ProbeOutcome{State: domain.EndpointTestFail, Message: message}
}
