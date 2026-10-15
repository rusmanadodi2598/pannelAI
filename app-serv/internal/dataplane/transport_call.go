// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_call.go
// @for       The outbound call itself: the retry loop, one attempt with its classification, and the §4 deadlines the attempt runs under.
// @uses      internal/provider, bytes, context, errors, io, net/http, time.
// @reason    SPEC-API-001 §4 fixes the deadlines (connect 10s, total 120s, no total cap while streaming with a 300s idle read) and AGENTS.md §1.1 asks for the split before the limit forces it. The loop owns the decision rather than the plugin: the plugin only says whether an outcome is worth repeating and the registry entry only says how many attempts the provider allows, so neither can multiply a request on its own.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
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
	// The caller's context travels on the request because the Transformer seam
	// has no parameter to carry it, and a connector that reads during shaping
	// must stop when the client does rather than finish on its own clock.
	request.Context = ctx
	if err := applyShape(plugin, &request); err != nil {
		return nil, err
	}
	// A connector that declares it refuses a non-streaming request is obeyed here rather than
	// trusted to have rewritten its own body, since the declaration is meaningless if the core
	// still sends what the provider rejects. The rewrite is mechanical and wire-agnostic because
	// every wire the gateway translates names this member `stream`.
	//
	// The client's own shape is read first: it, not the rewritten upstream shape, decides the
	// attempt's deadline, so a call made for one body stays bounded even when the provider
	// answers it with a stream the gateway folds back.
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

	// The session slot is taken before the attempt's clock starts, and the wait
	// is handed back to the walk's budget, so a request that queued behind
	// another on the same upstream session still gets its full bound. The slot
	// travels with the answer body: the upstream request is in flight until the
	// caller closes it.
	release, err := t.acquireSession(ctx, plugin, call)
	if err != nil {
		return nil, timeoutError(err)
	}
	handedOff := false
	defer func() {
		if !handedOff {
			release()
		}
	}()

	deadline := attemptDeadline(clientStream, call.Provider)
	// TotalTimeout is documented as bounding one non-streamed call end to end, so
	// the budget is measured once here and each attempt gets what is left of it.
	// Passing the full deadline per attempt let one call run twice the bound plus
	// the backoff between them, and hold a handler goroutine that much longer than
	// the contract says. The walk's own clock is the tighter bound wherever it
	// reaches: one credential may not outlive the request's walk, which is what
	// keeps a rate-limited upstream's honored Retry-After from swallowing it.
	ends := time.Time{}
	if deadline > 0 {
		ends = time.Now().Add(deadline)
	}
	if call.Budget != nil && (ends.IsZero() || call.Budget.deadline.Before(ends)) {
		ends = call.Budget.deadline
	}

	for retries := 0; ; retries++ {
		attemptDeadlineLeft := deadline
		if !ends.IsZero() {
			left := time.Until(ends)
			if left <= 0 {
				return nil, timeoutError(context.DeadlineExceeded)
			}
			// A streamed client keeps no context deadline: the idle guard owns
			// the stream once it has spoken, and the walk must not cut an answer
			// the caller is already reading. Only its pre-commit wait below is
			// bounded by the walk.
			if deadline > 0 && left < attemptDeadlineLeft {
				attemptDeadlineLeft = left
			}
		}
		// The first-byte wait is part of the pre-commit phase, so the walk's
		// clock clamps it too.
		firstByte := firstByteTimeout(call.Provider)
		if !ends.IsZero() {
			if left := time.Until(ends); left < firstByte {
				firstByte = left
			}
		}
		upstream, failure, err := t.attempt(ctx, plugin, call, url, attemptDeadlineLeft, firstByte)
		switch {
		case err != nil:
			decision := DecideRetry(call.Provider, plugin, Attempt{Retries: retries, Idempotent: call.Idempotent})
			if !decision.Retry {
				return nil, err
			}
			wait, ok := boundedWait(decision.After, ends)
			if !ok {
				// The wait cannot fit in what is left of the walk, so the retry
				// would begin after the walk's own clock expired. The failure in
				// hand is the real cause, so it is reported now rather than as
				// the timeout the sleep would have earned.
				return nil, err
			}
			if waitErr := sleep(ctx, wait); waitErr != nil {
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
				Retries: retries, Status: failure.Status, Header: failure.Header,
				Idempotent: call.Idempotent, Replayable: failure.Replayable,
			})
			if !decision.Retry {
				return nil, failure
			}
			wait, ok := boundedWait(decision.After, ends)
			if !ok {
				return nil, failure
			}
			if waitErr := sleep(ctx, wait); waitErr != nil {
				return nil, timeoutError(waitErr)
			}
		default:
			// The connector's rename record travels with the answer: a tool
			// it had to conceal on the way out is restored to the caller's
			// own name on the way back. It is the same shaped request every
			// attempt sent, so it is read once, here.
			upstream.ToolAliases = request.ToolAliases
			upstream.Body = &sessionBody{ReadCloser: upstream.Body, release: release}
			handedOff = true
			return upstream, nil
		}
	}
}

// attempt performs exactly one outbound call and classifies its outcome, so Do
// only has to decide. deadline is what attemptDeadline reported: zero for a
// streamed client, which SPEC-API-001 §4 bounds by the idle read instead.
// firstByte bounds how long a streamed answer may stay silent before its first
// byte; zero disables the wait (a connector whose provider is known to answer
// promptly never needs it, and a test may pin the other bounds alone).
func (t *Transport) attempt(ctx context.Context, plugin provider.Plugin, call Call, url string, deadline, firstByte time.Duration) (*Upstream, *UpstreamError, error) {
	var (
		attemptCtx context.Context
		cancel     context.CancelFunc
	)
	if deadline > 0 {
		attemptCtx, cancel = context.WithTimeout(ctx, deadline)
	} else {
		attemptCtx, cancel = context.WithCancel(ctx)
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

	// The attempt's context travels with the request: a proxy walk clones the
	// request with the context it is handed, so passing the caller's context
	// here would replace the attempt deadline (and the idle guard's cancel)
	// with one that has neither.
	response, err := t.dialer.Do(attemptCtx, request, call.Provider.ID)
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
		// A provider that wraps its answer in an envelope states its real status
		// inside the body, so the first frame is read before anything is piped:
		// a refusal there is an upstream failure the router can act on, not a
		// served answer that gets billed.
		stream := response.Body
		if envelope, ok := plugin.(provider.StreamEnvelope); ok {
			unwrapped, failure := envelope.OpenStream(response.Body)
			if failure != nil {
				cancel()
				// Nothing was piped: the refusal was read out of the first frame
				// precisely so that nothing could be. Repeating the call is
				// therefore safe, which is what the budget below is told.
				return nil, &UpstreamError{
					Status:     failure.Status,
					Header:     response.Header,
					Message:    failure.Message,
					Body:       []byte(failure.Message),
					Replayable: true,
				}, nil
			}
			stream = unwrapped
		}
		// The first byte is awaited here, inside the attempt, because only the
		// transport can turn a silent upstream into a retry against the same
		// target and a failover to the next credential: once a body is handed
		// out, a read that stalls is the caller's to report, not the walk's.
		if firstByte > 0 {
			guarded, err := firstByteWithin(stream, firstByte, cancel)
			if err != nil {
				cancel()
				// reason: the read already failed, so a close error adds
				// nothing to the timeout being reported.
				_ = stream.Close()
				return nil, nil, err
			}
			stream = guarded
		}
		// The guard owns the cancel: it fires when reads stall, and it releases
		// the context when the caller closes the body.
		return &Upstream{
			Status: response.StatusCode,
			Header: response.Header,
			Usage:  usage,
			Body:   newIdleGuard(stream, idleTimeout(call.Provider), cancel),
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

// clampWait bounds a wait to what is left of the walk's clock. The second
// answer is false when the wait cannot fit: sleeping it would spend the rest of
// the walk and leave nothing for the retry it was waiting for, so the caller
// reports the failure it already has instead of the timeout the sleep would
// earn. That is what turns a rate-limited upstream's honored Retry-After
// (minutes) into an immediate RATE_LIMITED rather than a masked timeout.
func clampWait(wait, remaining time.Duration) (time.Duration, bool) {
	if wait <= 0 {
		return 0, true
	}
	if remaining <= 0 || wait >= remaining {
		return 0, false
	}
	return wait, true
}

// boundedWait applies clampWait to a walk clock. A zero ends means no walk
// bound reaches this call, which is the case for a direct Do a test drives.
func boundedWait(wait time.Duration, ends time.Time) (time.Duration, bool) {
	if ends.IsZero() {
		return wait, true
	}
	return clampWait(wait, time.Until(ends))
}
