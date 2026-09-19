// Command app-serv performs one guarded probe request and classifies its answer.
//
// @file      cmd/app-serv/provider_probe_call.go
// @for       The probe's single outbound request: destination check, credential
//
//	placement, and the status classification the panel reads.
//
// @uses      internal/domain, internal/provider, internal/registry,
//
//	internal/service, context, fmt, net/http, time.
//
// @reason    The classification is the whole value of a connectivity test: an
//
//	operator reads its result to decide whether to replace a credential or
//	fix a URL. It is separate from the port surface in provider_probe.go
//	because that file carries the two probe shapes, and AGENTS.md §1.1
//	asks for the split before the limit forces it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
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
	// Resolve and validate before sending anything, so a refused destination is
	// reported as a refusal with its reason rather than as an unreachable host.
	// The dialer's Control hook repeats the check on the address it reaches,
	// which is what closes the rebinding window (OWASP A01).
	if err := p.guard.CheckHost(ctx, req.URL.Hostname()); err != nil {
		return failure(refusalMessage("upstream", err)), nil
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

// failure builds a failed outcome with the given English explanation.
func failure(message string) service.ProbeOutcome {
	return service.ProbeOutcome{State: domain.EndpointTestFail, Message: message}
}
