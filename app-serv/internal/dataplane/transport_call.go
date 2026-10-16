// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_call.go
// @for       The retry loop around one upstream target: what the transport decides after an attempt, and how long it waits before deciding again.
// @uses      context, time, internal/dataplane/retry.go for the attempt budget.
// @reason    SPEC-API-001 §4 fixes the deadlines (connect 10s, total 120s, no total cap while streaming with a 300s idle read) and AGENTS.md §1.1 asks for the split before the limit forces it. The loop owns the decision rather than the plugin: the plugin only says whether an outcome is worth repeating and the registry entry only says how many attempts the provider allows, so neither can multiply a request on its own.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package dataplane

import (
	"context"
	"time"
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
		return nil, sessionWaitTimeout(err)
	}
	handedOff := false
	defer func() {
		if !handedOff {
			release()
		}
	}()

	deadline := attemptDeadline(clientStream, call.Provider)
	// TotalTimeout bounds one non-streamed call end to end, so the budget is measured
	// once here and each attempt gets what is left of it. The walk's own clock is the
	// tighter bound wherever it reaches: one credential may not outlive the
	// request's walk, which is what keeps a rate-limited upstream's honored
	// Retry-After from swallowing it.
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
			// A refusal the class or the signature says cannot answer differently
			// a moment later is not asked again on this key: a spent allowance, a
			// refused credential and a gate that closes on this gateway all spend
			// the walk instead of the retry budget. The answer is stamped where the
			// refusal was recognised, so the retry and the park read one verdict.
			if !failure.Retryable {
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
