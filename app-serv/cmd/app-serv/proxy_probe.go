// Command app-serv adapts the proxy connectivity port to HTTP.
//
// @file      cmd/app-serv/proxy_probe.go
// @for       The net/http implementation of service.ProxyProber, guarded by
//
//	internal/netguard (SPEC-API-001 §7.11, OWASP A01).
//
// @uses      internal/domain, internal/netguard, internal/service, context,
//
//	errors, io, net/http, net/url, strconv, time.
//
// @reason    §7.11 offers a connectivity test, and the test must reach the
//
//	candidate — but AGENTS.md §1.5 forbids net/http in the service layer.
//	The port lives in `internal/service`, this adapter lives in the
//	composition root. Two rules live here rather than in the service:
//	the destination is validated by the egress guard before any dial and
//	again inside the dialer (OWASP A01), and the URL fetched through the
//	proxy is a server-side constant, never a request field — a
//	client-supplied test URL would be a second SSRF seam.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// Proxy test budget. The operator is waiting on a button, so a candidate that
// has not carried a request in this window is reported unreachable rather than
// left hanging.
const (
	proxyTestConnectTimeout = 5 * time.Second
	proxyTestTimeout        = 10 * time.Second
	// proxyTestReadLimit bounds how much of the answer is read. The body is
	// discarded, never reflected: a proxy that answers with something huge must
	// not become a memory problem, and its bytes must not reach the caller
	// (OWASP A01 rule 8).
	proxyTestReadLimit = 1024
)

// proxyProber probes one candidate by making one request through it.
type proxyProber struct {
	guard   *netguard.Guard
	target  string
	timeout time.Duration
}

// newProxyProber builds the adapter. The target URL is the server's, not the
// request's, and the guard is the one the whole process shares.
func newProxyProber(guard *netguard.Guard, target string) *proxyProber {
	return &proxyProber{guard: guard, target: target, timeout: proxyTestTimeout}
}

// ProbeProxy dials the candidate and fetches the test URL through it.
func (p *proxyProber) ProbeProxy(ctx context.Context, target service.ProxyTarget) (service.ProxyProbeResult, error) {
	// Resolve and validate before dialing, so a refused destination is reported
	// as a refusal with its reason rather than as an opaque dial error. The
	// dialer's own Control hook repeats the check on the address it reaches,
	// which is what closes the rebinding window.
	if err := p.guard.CheckHost(ctx, target.Host); err != nil {
		return service.ProxyProbeResult{State: domain.EndpointTestFail, Message: refusalMessage(err)}, nil
	}

	proxyURL := &url.URL{
		Scheme: string(target.Protocol),
		Host:   target.Host + ":" + strconv.Itoa(target.Port),
	}
	if target.Username != "" {
		proxyURL.User = url.UserPassword(target.Username, target.Password)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		// The guard's dialer is what reaches the proxy, so the address actually
		// connected to is the address that was validated.
		DialContext:           p.guard.NewDialer(proxyTestConnectTimeout, 0).DialContext,
		TLSHandshakeTimeout:   proxyTestConnectTimeout,
		ResponseHeaderTimeout: p.timeout,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   p.timeout,
		// A redirect is a second destination; this test never follows one, so
		// there is no hop to revalidate (OWASP A01 rule 5).
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.target, nil)
	if err != nil {
		return service.ProxyProbeResult{}, err
	}
	started := time.Now()
	response, err := client.Do(request)
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		return service.ProxyProbeResult{State: domain.EndpointTestFail, LatencyMS: latency, Message: transportMessage(err)}, nil
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, proxyTestReadLimit))

	return classifyProxyStatus(response.StatusCode, latency), nil
}

// classifyProxyStatus turns the answer into a result. Any status the proxy
// carried proves the tunnel works — a 404 from the test URL is the target's
// business — so only the proxy's own refusals are reported as failures.
func classifyProxyStatus(status, latency int) service.ProxyProbeResult {
	outcome := service.ProxyProbeResult{LatencyMS: latency}
	switch {
	case status >= 200 && status < 400:
		outcome.State = domain.EndpointTestOK
	case status == http.StatusProxyAuthRequired,
		status == http.StatusUnauthorized,
		status == http.StatusForbidden:
		outcome.State = domain.EndpointTestFail
		outcome.Message = "the proxy rejected the credentials"
	default:
		outcome.State = domain.EndpointTestFail
		outcome.Message = "the proxy answered " + strconv.Itoa(status)
	}
	return outcome
}

// transportMessage renders a dial failure. A guard refusal keeps its reason,
// because that is the actionable part; every other transport failure is
// reported without the raw chain (AGENTS.md §1.3).
func transportMessage(err error) string {
	if errors.Is(err, netguard.ErrDenied) {
		return refusalMessage(err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "the proxy did not answer within the time limit"
	}
	return "the proxy could not be reached"
}

// refusalMessage keeps the guard's reason and drops the wrapper, so the panel
// shows "a private address" rather than the error chain.
func refusalMessage(err error) string {
	reason := netguard.Reason(err)
	if reason == "" {
		reason = "the address is not permitted"
	}
	return "the proxy address was refused: " + reason
}
