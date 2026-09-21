// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_relay.go
// @for       One resolved provider's leg of the §7.15 pipeline: select an
//
//	account, translate the request, call, and translate the answer back.
//
// @uses      internal/schema, context, time.
// @reason    SPEC-API-001 §7.12's accounting needs the identity of the call that
//
//	failed, not only of the one that succeeded (register G17), so this leg
//	builds its outcome before the first fallible step and returns it with
//	the error. Keeping the leg here is also what holds engine.go inside
//	the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// relayOnce handles one resolved provider: select, translate, call, and translate
// the answer back.
//
// The outcome is built before anything can fail, so every exit — a refused
// selection, a failed dial, a failed translation, a broken stream — reports the
// provider, endpoint, model, and combo the call was attempted with. The chat
// plane records a failed call from that identity (register G17), and a zero
// outcome would carry nothing to record.
func (e *Engine) relayOnce(ctx context.Context, in Request, resolution Resolution, sink FrameSink) (Outcome, error) {
	outcome := Outcome{
		Format:     in.ClientFormat,
		ProviderID: resolution.Provider.ID,
		Model:      resolution.ModelID,
		Combo:      resolution.Combo.Name(),
		Streamed:   in.Stream,
	}

	selection, err := e.selector.Select(ctx, resolution.Provider.ID)
	if err != nil {
		return outcome, err
	}
	outcome.EndpointID = selection.Endpoint.ID()

	body, err := upstreamBody(in, resolution)
	if err != nil {
		return outcome, err
	}
	if e.saver != nil {
		body = e.saver.Apply(ctx, body, resolution.Target, resolution.UpstreamID, in.TokenSaverBypass)
	}

	started := e.clock()
	upstream, err := e.transport.Do(ctx, Call{
		Provider:   resolution.Provider,
		Model:      resolution.Model,
		Credential: selection.Credential,
		Body:       body,
		Stream:     in.Stream,
		// A chat completion is never idempotent: the upstream may have begun
		// generating, so the transport caps its retries.
		Idempotent: false,
	})
	if err != nil {
		e.recordFailure(ctx, selection, err)
		outcome.LatencyMS = e.elapsedMS(started)
		return outcome, e.translateCallError(err)
	}
	defer func() {
		// reason: the body is read to completion by the caller of this function,
		// so a close error here reports nothing a request path can act on.
		_ = upstream.Close()
	}()

	if err := e.answer(ctx, upstream, resolution, in, sink, selection, started, &outcome); err != nil {
		return outcome, err
	}
	outcome.LatencyMS = e.elapsedMS(started)
	// A served request clears the key's circuit state, which is what makes a
	// recovered credential usable again on the next call.
	if err := e.selector.RecordSuccess(ctx, selection); err != nil {
		return outcome, err
	}
	return outcome, nil
}

// elapsedMS is the call's duration since started, clamped at zero so a clock
// that steps backwards cannot produce a negative counter the usage aggregate
// rejects.
func (e *Engine) elapsedMS(started time.Time) int64 {
	ms := e.clock().Sub(started).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

// answer reads one upstream answer into the outcome, in whichever of the three
// shapes the call produced: a stream the client asked for, a stream folded back
// for a client that asked for one body, or a plain non-streamed body.
//
// The dispatch lives here rather than inline so the relay leg reads as the
// pipeline it is, and so the failure bookkeeping for every shape happens once.
func (e *Engine) answer(
	ctx context.Context,
	upstream *Upstream,
	resolution Resolution,
	in Request,
	sink FrameSink,
	selection Selection,
	started time.Time,
	outcome *Outcome,
) error {
	body, usage, err := e.readAnswer(ctx, upstream, resolution, in, sink, outcome)
	if err != nil {
		e.recordFailure(ctx, selection, err)
		outcome.LatencyMS = e.elapsedMS(started)
		return err
	}
	outcome.Body = body
	outcome.Usage = usage
	return nil
}

// readAnswer performs the translation the call's shape calls for.
func (e *Engine) readAnswer(
	ctx context.Context,
	upstream *Upstream,
	resolution Resolution,
	in Request,
	sink FrameSink,
	outcome *Outcome,
) ([]byte, *schema.Usage, error) {
	switch {
	case in.Stream:
		return nil, nil, e.relayStream(ctx, upstream, resolution, in, sink, outcome)
	case e.transport.ForcesStream(resolution.Provider):
		// The provider only answers a stream, so the single body the client
		// asked for is folded back out of it. The fold returns the upstream's
		// own non-streamed wire, so the same answer translator serves it.
		return e.translateFolded(upstream, resolution, in)
	default:
		return e.translateAnswer(upstream, resolution, in)
	}
}
