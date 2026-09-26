// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_call.go
// @for       The outbound call itself: the retry loop, one attempt with its
//
//	classification, and the §4 deadlines the attempt runs under.
//
// @uses      internal/provider, bytes, context, errors, io, net/http, time.
// @reason    SPEC-API-001 §4 fixes the deadlines (connect 10s, total 120s, no
//
//	total cap while streaming with a 300s idle read) and AGENTS.md §1.1
//	asks for the split before the limit forces it. The loop owns the
//	decision rather than the plugin: the plugin only says whether an
//	outcome is worth repeating and the registry entry only says how many
//	attempts the provider allows, so neither can multiply a request on
//	its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// Do performs one call, retrying the same target per SPEC-API-001 §4. The
// returned body is the caller's to close.
func (t *Transport) Do(ctx context.Context, call Call) (*Upstream, error) {
	plugin := t.connectors.For(call.Provider)
	request := RequestFor(call)
	if err := applyShape(plugin, &request); err != nil {
		return nil, err
	}
	// A connector that declares it refuses a non-streaming request is obeyed
	// here rather than trusted to have rewritten its own body: the declaration
	// would be meaningless if the core still sent the request the provider
	// rejects. The rewrite is mechanical and wire-agnostic because every wire
	// the gateway translates names this member `stream`.
	//
	// The client's own shape is read first: it, not the rewritten upstream shape,
	// decides the attempt's deadline, so a call a client made for one body stays
	// bounded even when the provider answers it with a stream the gateway folds
	// (draft 021, the missing total bound on the fold).
	clientStream := call.Stream
	if forcesStream(plugin) && !request.Stream {
		streamed, err := forceStreamMember(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body = streamed
		request.Stream = true
	}
	url, err := plugin.Endpoint(request, call.Credential)
	if err != nil {
		return nil, wrapDataPlaneError(CodeUpstreamError, "upstream endpoint could not be resolved", err)
	}
	// The shaped request replaces the call's body and stream flag, so the
	// attempt below sends what the connector asked for rather than what the
	// caller asked for.
	call.Body = request.Body
	call.Stream = request.Stream
	deadline := attemptDeadline(clientStream, call.Provider)

	for retries := 0; ; retries++ {
		upstream, failure, err := t.attempt(ctx, plugin, call, url, deadline)
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
// only has to decide. deadline is what attemptDeadline reported: zero for a
// streamed client, which SPEC-API-001 §4 bounds by the idle read instead.
func (t *Transport) attempt(ctx context.Context, plugin provider.Plugin, call Call, url string, deadline time.Duration) (*Upstream, *UpstreamError, error) {
	attemptCtx, cancel := context.WithCancel(ctx)
	if deadline > 0 {
		attemptCtx, cancel = context.WithTimeout(ctx, deadline)
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

	response, err := t.dialer.Do(ctx, request)
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
