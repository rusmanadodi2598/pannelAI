// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_relay.go
// @for       One resolved provider's leg of the §7.15 pipeline: walk the
//
//	provider's credentials, translate the request once, call, and
//	translate the answer back.
//
// @uses      internal/domain, internal/reasoning, internal/schema, context, time.
// @reason    SPEC-API-001 §7.7 fixes the failover order as credential-first, so
//
//	this leg walks the provider's healthy credentials before it gives
//	up to the next combo member, and each attempt's outcome reports the
//	identity it was attempted with so the chat plane records a failed
//	call (register G17). It is also where the §7.15 reasoning injection
//	runs, on the body the upstream receives. Keeping the leg here is
//	what holds engine.go inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/reasoning"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// relayOnce handles one resolved provider: walk its credentials, translate,
// call, and translate the answer back.
//
// The request is translated once, before any credential is spent: a translation
// failure would repeat identically for every credential, so it fails the leg
// with a zero outcome: no upstream call happened, so there is no identity to
// record (draft 028 F3).
//
// Each credential that fails in a failover-worthy way is excluded and the next
// healthy one is tried; a request-shaped refusal is handed back immediately,
// because the same body would be refused identically everywhere (draft 028 F2).
// When no credential remains, the leg reports the last in-leg failure with the
// last attempted identity; when the very first selection is refused, it reports
// the refusal with no identity at all.
func (e *Engine) relayOnce(ctx context.Context, in Request, resolution Resolution, sink FrameSink) (Outcome, error) {
	body, err := upstreamBody(in, resolution)
	if err != nil {
		return Outcome{}, err
	}
	// Reasoning is normalized first and the token saver second, which is the
	// reference's own order (chatCore.js applies thinking inside translation,
	// then runs the savers on the final body): a saver that rewrites messages
	// must see the body the upstream will actually receive, thinking fields
	// included.
	if e.thinking != nil {
		body = e.thinking.Apply(ctx, body, reasoning.Call{
			Wire:       resolution.Target,
			ProviderID: resolution.Provider.ID,
			ModelID:    resolution.ModelID,
			ClientRaw:  in.Raw,
			Override:   in.ThinkingOverride,
		})
	}
	if e.saver != nil {
		body = e.saver.Apply(ctx, body, resolution.Target, resolution.UpstreamID, in.TokenSaverBypass)
	}

	outcome := Outcome{
		Format:   in.ClientFormat,
		Combo:    resolution.Combo.Name(),
		Streamed: in.Stream,
	}
	// finish serves one successful call and owns its resources: the body closes
	// and the activity marker releases when it returns, whichever way it
	// returns. The guards live here rather than in the loop body so exactly one
	// attempt registers them, and so the loop stays free of a defer it would
	// otherwise carry per iteration.
	finish := func(upstream *Upstream, release func(), selection Selection, started time.Time) (Outcome, error) {
		defer func() {
			// reason: the body is read to completion by the caller of this
			// function, so a close error here reports nothing a request path
			// can act on.
			_ = upstream.Close()
		}()
		defer release()

		if err := e.answer(ctx, upstream, resolution, in, sink, selection, started, &outcome); err != nil {
			return outcome, err
		}
		outcome.LatencyMS = e.elapsedMS(started)
		// A served request clears the key's parked state, which is what makes a
		// recovered credential usable again on the next call.
		if err := e.selector.RecordSuccess(ctx, selection); err != nil {
			return outcome, err
		}
		return outcome, nil
	}
	spent := make(map[string]struct{})
	var memberErr error
	for {
		selection, err := e.selector.SelectNext(ctx, resolution.Provider.ID, spent)
		if err != nil {
			if memberErr != nil {
				return outcome, memberErr
			}
			return Outcome{}, err
		}
		spent[CandidateID(selection)] = struct{}{}
		outcome.ProviderID = resolution.Provider.ID
		outcome.EndpointID = selection.Endpoint.ID()
		outcome.Model = resolution.ModelID

		// The marker opens once the credential is known and closes when this
		// attempt ends, whichever way it ends, so the drawing lights a node for
		// exactly as long as the provider is being called (SPEC-UI-001 §6.5).
		// A nil seam returns a release that does nothing.
		release := e.markActive(ctx, resolution.Provider.ID, outcome.EndpointID, resolution.ModelID)
		started := e.clock()
		upstream, callErr := e.transport.Do(ctx, Call{
			Provider: resolution.Provider,
			Model:    resolution.Model,
			// The translation's own answer, which is what a multi-endpoint
			// provider picks its endpoint by.
			Wire:       resolution.Target,
			Credential: selection.Credential,
			Body:       body,
			Stream:     in.Stream,
			// A chat completion is never idempotent: the upstream may have begun
			// generating, so the transport caps its retries.
			Idempotent: false,
		})
		if callErr != nil {
			release()
			e.recordFailure(ctx, selection, callErr)
			outcome.LatencyMS = e.elapsedMS(started)
			failure := e.translateCallError(callErr)
			if failoverWorthy(AsError(failure).Code) {
				memberErr = failure
				continue
			}
			return outcome, failure
		}
		return finish(upstream, release, selection, started)
	}
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
	// A streamed answer reports its usage through the outcome inside relayStream,
	// and hands back no separate body: overwriting here would discard the numbers
	// the upstream sent and record the call as 0/0 (draft 021 F5).
	if usage != nil {
		outcome.Usage = usage
	}
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
