// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport.go
// @for       The outbound HTTP call: URL and authentication through the provider
//
//	plugin seam, the §4 deadlines, and the retry loop.
//
// @uses      internal/provider, context, bytes, net, net/http, sort, time.
// @reason    SPEC-API-001 §4 fixes the timeouts (connect 10s, total 120s, no
//
//	total cap while streaming with a 300s idle read) and AGENTS.md §1.6
//	requires every outbound call to run under a context deadline.
//	Nothing here branches on a provider id: the URL and the credential
//	placement come from provider.Plugin, which is what makes a new
//	provider a registry entry rather than a change to shared code.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Deadlines fixed by SPEC-API-001 §4. A registry entry may override the total
// and the idle read through timeout_ms and stall_timeout_ms; the connect deadline
// has no override because a TCP handshake has no provider-specific shape.
const (
	// ConnectTimeout bounds the TCP and TLS handshake.
	ConnectTimeout = 10 * time.Second
	// TotalTimeout bounds one non-streamed call end to end.
	TotalTimeout = 120 * time.Second
	// IdleTimeout bounds the gap between two reads of a streamed body. A stream
	// has no total cap because a long answer is not a failure, so the gap is
	// what detects a dead upstream.
	IdleTimeout = 300 * time.Second
	// maxUpstreamBodyBytes bounds a non-streamed response body, which is
	// buffered so the context can be released before the caller reads it.
	maxUpstreamBodyBytes = 32 << 20
	// errorBodyBytes bounds how much of an upstream error body is read, so a
	// provider answering with a huge page cannot be buffered.
	errorBodyBytes = 64 << 10
)

// SecretOpener decrypts a stored credential for exactly one request. It is an
// interface so the data plane never constructs a sealer with the process key:
// sealing and opening must share one implementation, or a value written by one
// path becomes unreadable by the other.
type SecretOpener interface {
	Open(sealed string) (string, error)
}

// Transport performs outbound calls through the plugin seam.
type Transport struct {
	connectors *provider.Connectors
	client     *http.Client
}

// TransportDeps holds the collaborators the transport needs.
type TransportDeps struct {
	// Connectors resolves a provider id to the plugin that handles it.
	Connectors *provider.Connectors
	// Client overrides the default HTTP client, so a deployment can install its
	// own pool limits. The default already carries the §1.6 deadlines and
	// explicit pool limits.
	Client *http.Client
}

// NewTransport validates deps and returns a transport.
func NewTransport(deps TransportDeps) (*Transport, error) {
	if deps.Connectors == nil {
		return nil, internalError("provider connectors are required", nil)
	}
	client := deps.Client
	if client == nil {
		client = NewHTTPClient()
	}
	return &Transport{connectors: deps.Connectors, client: client}, nil
}

// NewHTTPClient builds the HTTP client the gateway calls upstreams with.
//
// The pool limits are set explicitly rather than left at the library default
// (AGENTS.md §1.7): an unbounded pool fans out to a failing upstream as fast as
// to a healthy one, and the idle cap stops a long tail of providers from holding
// sockets open.
func NewHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: ConnectTimeout, KeepAlive: 30 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   16,
			MaxConnsPerHost:       64,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   ConnectTimeout,
			ResponseHeaderTimeout: TotalTimeout,
			ExpectContinueTimeout: time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}

// Do performs one call, retrying the same target per SPEC-API-001 §4. The
// returned body is the caller's to close.
//
// The retry loop owns the decision, not the plugin: the plugin only says whether
// an outcome is worth repeating and the registry entry only says how many attempts
// the provider allows, so neither can multiply a request on its own.
func (t *Transport) Do(ctx context.Context, call Call) (*Upstream, error) {
	plugin := t.connectors.For(call.Provider)
	url, err := plugin.Endpoint(RequestFor(call), call.Credential)
	if err != nil {
		return nil, wrapDataPlaneError(CodeUpstreamError, "upstream endpoint could not be resolved", err)
	}

	for retries := 0; ; retries++ {
		upstream, failure, err := t.attempt(ctx, plugin, call, url)
		switch {
		case err != nil:
			decision := DecideRetry(call.Provider, plugin, Attempt{Retries: retries, Idempotent: call.Idempotent})
			if !decision.Retry {
				return nil, err
			}
			if waitErr := sleep(ctx, decision.After); waitErr != nil {
				return nil, timeoutError(waitErr)
			}
		case failure != nil:
			// A quota rejection parks the account instead of retrying it:
			// repeating a request the account cannot pay for only delays the
			// failover to an account that can serve it.
			if plugin.IsQuotaError(failure.Status, failure.Body) {
				return nil, failure
			}
			decision := DecideRetry(call.Provider, plugin, Attempt{
				Retries: retries, Status: failure.Status, Header: failure.Header, Idempotent: call.Idempotent,
			})
			if !decision.Retry {
				return nil, failure
			}
			if waitErr := sleep(ctx, decision.After); waitErr != nil {
				return nil, timeoutError(waitErr)
			}
		default:
			return upstream, nil
		}
	}
}

// attempt performs exactly one outbound call and classifies its outcome, so Do
// only has to decide.
func (t *Transport) attempt(ctx context.Context, plugin provider.Plugin, call Call, url string) (*Upstream, *UpstreamError, error) {
	// A streamed call has no total cap, so only the non-streamed shape gets a
	// deadline here; the idle guard covers a stalled stream instead.
	attemptCtx, cancel := context.WithCancel(ctx)
	if !call.Stream {
		attemptCtx, cancel = context.WithTimeout(ctx, totalTimeout(call.Provider))
	}

	request, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, url, bytes.NewReader(call.Body))
	if err != nil {
		cancel()
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "upstream request could not be built", err)
	}
	applyHeaders(request.Header, call)
	if err := plugin.ApplyAuth(request, call.Credential); err != nil {
		cancel()
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "upstream credential could not be applied", err)
	}

	response, err := t.client.Do(request)
	if err != nil {
		cancel()
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, nil, timeoutError(err)
		}
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "the upstream could not be reached", err)
	}

	usage := plugin.DecodeUsage(response.StatusCode, response.Header)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		failure := readUpstreamError(response)
		cancel()
		return nil, failure, nil
	}

	if call.Stream {
		// The guard owns the cancel: it fires when reads stall, and it releases
		// the context when the caller closes the body.
		return &Upstream{
			Status: response.StatusCode,
			Header: response.Header,
			Usage:  usage,
			Body:   newIdleGuard(response.Body, idleTimeout(call.Provider), cancel),
		}, nil, nil
	}

	// A non-streamed body is buffered so the context can be released before the
	// caller reads it, which lets the total deadline bound the whole exchange.
	body, readErr := readBounded(response.Body, maxUpstreamBodyBytes)
	cancel()
	if readErr != nil {
		// reason: the body is fully consumed or the limit was hit, so a close
		// error adds nothing to the read error already reported.
		_ = response.Body.Close()
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "the upstream response could not be read", readErr)
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "the upstream response could not be read", closeErr)
	}
	return &Upstream{
		Status: response.StatusCode,
		Header: response.Header,
		Usage:  usage,
		Body:   io.NopCloser(bytes.NewReader(body)),
	}, nil, nil
}

// applyHeaders copies the registry entry's declared headers and the shape headers
// onto the outbound request. A connector may add to these; nothing here is
// provider-specific, which is why the entry carries them as data.
func applyHeaders(dst http.Header, call Call) {
	declared := call.Provider.Transport.Headers
	keys := make([]string, 0, len(declared))
	for key := range declared {
		keys = append(keys, key)
	}
	// Sorted so a test comparing the outbound request is not order-dependent.
	sort.Strings(keys)
	for _, key := range keys {
		dst.Set(key, declared[key])
	}
	if dst.Get("Content-Type") == "" {
		dst.Set("Content-Type", "application/json")
	}
	if call.Stream {
		dst.Set("Accept", "text/event-stream")
	}
}

// sleep waits, or returns early when the caller's context ends.
func sleep(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// totalTimeout reports the total deadline for a non-streamed call, honouring a
// registry override.
func totalTimeout(entry registry.Provider) time.Duration {
	if ms := entry.Transport.TimeoutMS; ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return TotalTimeout
}

// idleTimeout reports how long a streamed body may stay silent, honouring a
// registry override.
func idleTimeout(entry registry.Provider) time.Duration {
	if ms := entry.Transport.StallTimeoutMS; ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return IdleTimeout
}
