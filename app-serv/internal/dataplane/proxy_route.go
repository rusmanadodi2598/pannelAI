// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_route.go
// @for       The pool-driven dial: one planned walk across the proxy pool with
//
//	connect-stage failover and per-candidate transports.
//
// @uses      internal/domain, context, errors, net, net/http, net/url, sync.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D1/D7-D9: the pool rows are the
//
//	route when proxying is on, so the walk is what makes them serve
//	traffic instead of decorating it. The walk is bounded (D7), only
//	connect-stage errors fail over (a mid-response death must not
//	re-send a request the upstream may have answered), each candidate
//	dials through a clone of the shared client so the egress guard
//	still sees every address (D8), and the shared client itself keeps
//	its static proxy behaviour for the callers that do not walk (D9).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package dataplane

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ProxyRoutePlanner plans the proxy attempts one request may walk. Plan is
// consulted once per request with the provider that will answer and the
// destination host, because the provider's binding decides the candidate set
// (docs/PORT/009-PORT-PROVIDER-PROXY.md D2-D5); ReportFailure parks a candidate
// whose connect stage died so the next request skips it.
type ProxyRoutePlanner interface {
	Plan(ctx context.Context, providerID, host string) ([]domain.ProxyRouteAttempt, error)
	ReportFailure(ctx context.Context, proxyID string)
}

// ProxyDialer serves one request through the plan the planner returns: an empty
// plan (proxying off, the host exempt, nothing usable) reaches the shared
// client unchanged, a planned walk tries each candidate in order and reports
// the connect failures.
type ProxyDialer struct {
	client *http.Client
	routes ProxyRoutePlanner
	// transports caches the per-candidate clients by proxy URL, so a rotating
	// pool reuses its connections instead of building a fresh pool per request.
	transports sync.Map // map[string]*http.Client
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
	for _, candidate := range plan {
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
			return response, nil
		}
		lastErr = attemptErr
		if !isProxyConnectFailure(attemptErr) {
			// The upstream answered or the caller gave up: neither is the
			// proxy's failure, so spending the next candidate would repeat a
			// request the wire may already have delivered.
			return nil, attemptErr
		}
		d.routes.ReportFailure(ctx, candidate.ID)
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
	shared := d.client.Transport
	if shared == nil {
		shared = http.DefaultTransport
	}
	base, ok := shared.(*http.Transport)
	if !ok {
		return nil, errors.New("the shared transport cannot carry a proxy candidate")
	}
	key := candidate.String()
	if cached, ok := d.transports.Load(key); ok {
		if client, ok := cached.(*http.Client); ok {
			return client, nil
		}
	}
	clone := base.Clone()
	clone.Proxy = http.ProxyURL(candidate)
	client := &http.Client{Transport: clone}
	if actual, loaded := d.transports.LoadOrStore(key, client); loaded {
		if existing, ok := actual.(*http.Client); ok {
			return existing, nil
		}
	}
	return client, nil
}

// isProxyConnectFailure classifies the failures the walk may fail over on: a
// dial error through the proxy (the OpError the transport wraps in url.Error)
// or a timeout waiting for it. A TLS failure the destination answered for and
// every mid-response error are the upstream's, not the pool's.
func isProxyConnectFailure(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
