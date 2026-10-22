// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_route.go
// @for       The pool-driven dial: one planned walk across the proxy pool with connect-stage failover and per-candidate transports.
// @uses      internal/domain, context, errors, log/slog, net/http, net/url, sync, time.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D1/D7-D9: the pool rows are the route when proxying is on, so the walk is what makes them serve traffic instead of decorating it. The walk is bounded (D7); a connect stage that died moves on, and so does an upstream that refused that one address, while a mid-response death must not re-send a request the upstream may have answered. Each candidate dials through a clone of the shared client so the egress guard still sees every address (D8), and the shared client itself keeps its static proxy behaviour for the callers that do not walk (D9).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-26
package dataplane

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ProxyRoutePlanner plans the proxy attempts one request may walk. Plan is
// consulted once per request with the provider that will answer and the
// destination host, because the provider's binding decides the candidate set
// (docs/PORT/009-PORT-PROVIDER-PROXY.md D2-D5); ReportFailure takes a candidate
// out of rotation for the wait the failure stated, whether the connect stage died
// or the upstream refused that address.
type ProxyRoutePlanner interface {
	Plan(ctx context.Context, providerID, host string) ([]domain.ProxyRouteAttempt, error)
	ReportFailure(ctx context.Context, proxyID string, ttl time.Duration)
}

// maxProxyTransports bounds the per-candidate client cache. A pool row that is
// edited or removed must not leave its transport (and its idle connections)
// behind forever; the cap keeps the cache to the candidates a rotated pool can
// plausibly serve in one cycle.
const maxProxyTransports = 32

// ProxyDialer serves one request through the plan the planner returns: an empty
// plan (proxying off, the host exempt, nothing usable) reaches the shared
// client unchanged, a planned walk tries each candidate in order and reports
// the connect failures.
type ProxyDialer struct {
	client *http.Client
	routes ProxyRoutePlanner
	// transports caches the per-candidate clients by proxy URL, so a rotating
	// pool reuses its connections instead of building a fresh pool per request.
	// Insertion order is kept alongside because a map alone cannot say which
	// entry is oldest when the cache is full.
	transportMu    sync.Mutex
	transports     map[string]*http.Client
	transportOrder []string
	// logger records which candidate carried the request. Nil falls back to the
	// process logger, so a wiring that forgets it still leaves the trace.
	logger *slog.Logger
}

// Do performs the request through the plan. providerID is the registry entry
// that will answer, which the plan resolves the binding against; an empty value
// is a caller with no provider (an unscoped probe), which follows the global
// setting. The returned body is the caller's to close, exactly as
// http.Client.Do would hand it over.
func (d *ProxyDialer) Do(ctx context.Context, request *http.Request, providerID string) (*http.Response, error) {
	if d.routes == nil {
		return d.client.Do(request)
	}
	plan, err := d.routes.Plan(ctx, providerID, request.URL.Host)
	if err != nil {
		return nil, wrapDataPlaneError(CodeUpstreamError, "the proxy plan could not be read", err)
	}
	if len(plan) == 0 {
		return d.client.Do(request)
	}
	if len(plan) > domain.MaxProxyRouteAttempts {
		plan = plan[:domain.MaxProxyRouteAttempts]
	}
	var lastErr error
	var skipped []string
	for index, candidate := range plan {
		attemptClient, clientErr := d.clientFor(candidate.URL)
		if clientErr != nil {
			// A candidate whose URL cannot shape a transport is skipped, not
			// fatal: the walk exists to survive broken rows, and the planner
			// already refuses a sealed credential it cannot open.
			lastErr = clientErr
			continue
		}
		response, attemptErr := d.attempt(ctx, attemptClient, request)
		if attemptErr == nil {
			refusal := classifyAddressRefusal(response)
			if !refusal.failover {
				d.noteServed(ctx, providerID, candidate.ID, request.URL.Host, response.StatusCode, skipped)
				return response, nil
			}
			// The vendor refused this address rather than the request, so the next
			// one is worth its round trip. With no candidate left, or no way to
			// rebuild the payload, the refusal is the answer: it goes back intact
			// rather than being dressed up as a gateway failure.
			d.routes.ReportFailure(ctx, candidate.ID, refusal.parkFor)
			if index == len(plan)-1 || request.GetBody == nil {
				d.noteServed(ctx, providerID, candidate.ID, request.URL.Host, response.StatusCode, skipped)
				return response, nil
			}
			// reason: this answer is being discarded in favour of the next
			// candidate's, so a close error names a body no caller will read.
			_ = response.Body.Close()
			skipped = append(skipped, candidate.ID)
			continue
		}
		lastErr = attemptErr
		if !isProxyConnectFailure(attemptErr) {
			// The upstream answered or the caller gave up: neither is the
			// proxy's failure, so spending the next candidate would repeat a
			// request the wire may already have delivered.
			return nil, attemptErr
		}
		d.routes.ReportFailure(ctx, candidate.ID, domain.ProxyFailureCooldown)
	}
	return nil, lastErr
}

// attempt performs one candidate's call with a restored body, because the
// previous attempt's client consumed the original reader even though it never
// left the connect stage.
func (d *ProxyDialer) attempt(ctx context.Context, client *http.Client, request *http.Request) (*http.Response, error) {
	attemptRequest := request.Clone(ctx)
	if attemptRequest.GetBody != nil {
		body, err := attemptRequest.GetBody()
		if err != nil {
			return nil, wrapDataPlaneError(CodeUpstreamError, "the request body could not be restored for the next proxy attempt", err)
		}
		attemptRequest.Body = body
	}
	return client.Do(attemptRequest)
}

// clientFor builds (or reuses) the per-candidate client: the shared transport's
// clone with the candidate as its proxy. The clone keeps the shared dialer, so
// the egress guard validates the proxy's address at connect time the same way
// it does for the static route (D8).
func (d *ProxyDialer) clientFor(candidate *url.URL) (*http.Client, error) {
	if candidate == nil || candidate.Host == "" {
		return nil, errors.New("the proxy candidate has no address")
	}
	base, ok := d.client.Transport.(*http.Transport)
	if !ok {
		return nil, errors.New("the shared client carries no transport to clone the guard from")
	}
	key := candidate.String()
	d.transportMu.Lock()
	cached, ok := d.transports[key]
	d.transportMu.Unlock()
	if ok {
		return cached, nil
	}
	clone := base.Clone()
	clone.Proxy = http.ProxyURL(candidate)
	clone.OnProxyConnectResponse = rejectConnectResponse
	// The per-candidate client carries the shared client's redirect policy, so
	// a proxied call cannot follow a 3xx the direct path refuses. A shared
	// client that states none gets the gateway's own no-follow policy.
	checkRedirect := d.client.CheckRedirect
	if checkRedirect == nil {
		checkRedirect = noRedirect
	}
	client := &http.Client{Transport: clone, CheckRedirect: checkRedirect}

	d.transportMu.Lock()
	defer d.transportMu.Unlock()
	if d.transports == nil {
		d.transports = make(map[string]*http.Client, maxProxyTransports)
	}
	if existing, ok := d.transports[key]; ok {
		// Another request cached the same candidate while this one cloned.
		clone.CloseIdleConnections()
		return existing, nil
	}
	d.transports[key] = client
	d.transportOrder = append(d.transportOrder, key)
	for len(d.transportOrder) > maxProxyTransports {
		oldest := d.transportOrder[0]
		d.transportOrder = d.transportOrder[1:]
		evicted, present := d.transports[oldest]
		if !present {
			continue
		}
		delete(d.transports, oldest)
		evicted.CloseIdleConnections()
	}
	return client, nil
}
