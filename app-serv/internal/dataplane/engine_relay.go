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

	if in.Stream {
		if err := e.relayStream(ctx, upstream, resolution, in, sink, &outcome); err != nil {
			e.recordFailure(ctx, selection, err)
			outcome.LatencyMS = e.elapsedMS(started)
			return outcome, err
		}
	} else {
		body, usage, err := e.translateAnswer(upstream, resolution, in)
		if err != nil {
			e.recordFailure(ctx, selection, err)
			outcome.LatencyMS = e.elapsedMS(started)
			return outcome, err
		}
		outcome.Body = body
		outcome.Usage = usage
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
