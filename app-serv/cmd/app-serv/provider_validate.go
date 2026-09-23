// Command app-serv adapts the stateless credential check to HTTP.
//
// @file      cmd/app-serv/provider_validate.go
// @for       The net/http implementation of service.CredentialValidator: the
// //
//
//	models probe, the chat fallback, and the Anthropic status rule.
//
// @uses      internal/domain, internal/netguard, internal/provider,
//
//	internal/registry, internal/service, context, fmt, net/http,
//	strings, time. The request shapes are in provider_validate_request.go.
//
// @reason    SPEC-API-001 §7.4 has no validate route, so draft 017 §4.6's finding
//
//	is that a credential can only be tested after it is stored. The
//	reference validates before the write, and its two non-obvious rules are
//	ported here: an upstream that does not serve `/models` is probed with a
//	one-token chat request instead of being reported broken, and an
//	Anthropic wire treats anything but 401/403 as proof the key was
//	accepted.
//
//	The destination is operator-supplied on both paths, so every request
//	goes through the process egress guard (OWASP A01), pre-flight and at
//	connect time.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// chatProbeMaxTokens is the one token the fallback asks for. A validation spends
// as little of the operator's budget as the upstream will accept.
const chatProbeMaxTokens = 1

// probeModelFallback names the model a chat probe uses when the caller supplied
// none. Providers reject an empty model name before they look at the credential,
// which would report a working key as broken.
const probeModelFallback = "test"

// validateNode implements service.CredentialValidator for a node's base URL.
func (p *httpEndpointProber) ValidateNode(ctx context.Context, check service.CredentialCheck) (service.ProbeOutcome, error) {
	entry := registry.Provider{
		ID:        "validate",
		Category:  "apikey",
		AuthType:  registry.AuthAPIKey,
		Transport: registry.Transport{Format: nodeFormat(check), BaseURL: check.BaseURL},
	}
	connector := p.connectors.For(entry)
	cred := provider.StaticKey("", "", check.Credential)
	if check.Credential == "" {
		cred = provider.NoCredential("validate")
	}
	return p.validate(ctx, validateRequest{
		base:      strings.TrimSuffix(strings.TrimSpace(check.BaseURL), "/"),
		entry:     entry,
		connector: connector,
		cred:      cred,
		anthropic: check.NodeType == string(domain.NodeAnthropicCompatible),
		modelID:   check.ModelID,
	})
}

// validateRequest is one stateless check's inputs.
type validateRequest struct {
	// base is the node's base URL with no trailing slash.
	base string
	// entry is the provider entry the credential is placed for.
	entry registry.Provider
	// connector places the credential.
	connector provider.Plugin
	// cred is the credential, named as one family.
	cred provider.Credential
	// anthropic selects the status rule.
	anthropic bool
	// direct means the base URL is the probe target itself: a claude-format base
	// that already IS the messages endpoint, so no path is appended.
	direct bool
	// modelID is the model a chat probe names, when the caller supplied one.
	modelID string
}

// validate runs the models probe, then the chat fallback when the models path is
// not there.
//
// The fallback is the reference's rule (provider-nodes/validate/route.js:80-96):
// many compatible servers do not serve `/models` but answer chat completions
// perfectly well, so a 404 there is a fact about the server rather than about the
// credential. A 401/403 is not retried: the credential was read and rejected, and
// asking again spends a request to learn the same answer.
func (p *httpEndpointProber) validate(ctx context.Context, req validateRequest) (service.ProbeOutcome, error) {
	// A provider whose base IS its messages endpoint serves no model list, so the
	// credential is proved by POSTing a one-token message to that exact URL. The
	// Anthropic rule then decides: only 401/403 mean the key was rejected.
	if req.direct {
		outcome, err := p.probeWith(ctx, req.base, req, http.MethodPost, chatProbeBody(modelIDOrDefault(req.modelID)))
		if err != nil {
			return outcome, err
		}
		outcome.Method = service.ProbeMethodChat
		if req.anthropic {
			return service.ValidateOutcomeForAnthropic(outcome.Status), nil
		}
		return outcome, nil
	}
	modelsTarget := req.base + "/models"
	outcome, err := p.probeWith(ctx, modelsTarget, req, http.MethodGet, nil)
	if err != nil {
		return outcome, err
	}
	outcome.Method = service.ProbeMethodModels
	if req.anthropic {
		return service.ValidateOutcomeForAnthropic(outcome.Status), nil
	}
	if outcome.State == domain.EndpointTestOK || outcome.Status == http.StatusUnauthorized || outcome.Status == http.StatusForbidden {
		return outcome, nil
	}
	// The models path is missing or unhappy for a reason that is not the
	// credential, so the chat path is asked before the credential is blamed.
	body := chatProbeBody(modelIDOrDefault(req.modelID))
	chatOutcome, err := p.probeWith(ctx, req.base+"/chat/completions", req, http.MethodPost, body)
	if err != nil {
		return chatOutcome, err
	}
	chatOutcome.Method = service.ProbeMethodChat
	return chatOutcome, nil
}

// probeWith performs one request and classifies the status.
//
// It differs from probe() in two ways the stateless check needs: it can POST a
// body, and it does not touch the endpoint/key vocabulary, because a validation
// has neither.
func (p *httpEndpointProber) probeWith(ctx context.Context, target string, req validateRequest, method string, body []byte) (service.ProbeOutcome, error) {
	var reader *bytes.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return failure("the validation request could not be built"), nil
	}
	if err := p.guard.CheckHost(ctx, httpReq.URL.Hostname()); err != nil {
		return failure(refusalMessage("upstream", err)), nil
	}
	for name, value := range req.entry.Transport.Headers {
		httpReq.Header.Set(name, value)
	}
	if len(body) > 0 {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if err := req.connector.ApplyAuth(httpReq, req.cred); err != nil {
		return failure(err.Error()), nil
	}

	started := time.Now()
	resp, err := p.client.Do(httpReq)
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		return service.ProbeOutcome{State: domain.EndpointTestFail, LatencyMS: latency, Message: "the upstream could not be reached"}, nil
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
		outcome.State = domain.EndpointTestFail
		outcome.Message = fmt.Sprintf("the upstream answered %d", resp.StatusCode)
	}
	return outcome, nil
}

// assertCredentialValidator keeps the adapter satisfying the service port, so a
// signature change becomes a compile error here rather than at the call site.
var _ service.CredentialValidator = (*httpEndpointProber)(nil)

// assertGuardDialer records that this file's requests share the process guard's
// client, which is what cmd/app-serv/egress_guard_assert_test.go asserts.
var _ = netguard.ErrDenied
