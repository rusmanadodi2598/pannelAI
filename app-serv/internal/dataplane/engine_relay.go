// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_relay.go
// @for       One resolved provider's leg of the §7.15 pipeline: walk the provider's credentials, translate the request once, call, and translate the answer back.
// @uses      internal/domain, internal/reasoning, context, time.
// @reason    SPEC-API-001 §7.7 fixes the failover order as credential-first, so this leg walks the provider's healthy credentials before it gives up to the next combo member, and each attempt's outcome reports the identity it was attempted with so the chat plane records a failed call (register G17). It is also where the §7.15 reasoning injection runs, on the body the upstream receives. The health decision a finished leg makes lives in engine_outcome.go, so this file stays inside the AGENTS.md §1.1 line budget and the abort rule has one home.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package dataplane

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/reasoning"
)

// relayOnce handles one resolved provider: walk its credentials, translate, call,
// and translate the answer back. The request is translated once, before any
// credential is spent, because a translation failure would repeat identically for
// every credential: it fails the leg with a zero outcome, since no upstream call
// happened and there is no identity to record. Each credential that fails in a
// failover-worthy way is excluded and the next healthy one tried; a request-shaped
// refusal is handed back immediately, because the same body would be refused
// identically everywhere. When no credential remains the leg reports the last
// in-leg failure with the last attempted identity, and when the very first selection
// is refused it reports the refusal with no identity at all.
func (e *Engine) relayOnce(ctx context.Context, in Request, resolution Resolution, sink FrameSink, budget *RequestBudget) (Outcome, error) {
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
			// The entry's own declaration travels with the call: a provider
			// that states the thinking shape its upstream reads outranks the
			// model's family format (§7.15).
			ProviderThinkingFormat: resolution.Provider.Transport.ThinkingFormat,
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
		e.recordServedOutcome(ctx, &outcome, selection)
		return outcome, nil
	}
	spent := make(map[string]struct{})
	// The leg walks one snapshot. Re-asking the provider per attempt re-read every account
	// and re-rolled the cursor mid-walk, so a failover could return to an account it had
	// already spent (SPEC-API-001 §7.5).
	plan, err := e.selector.BeginPlan(ctx, resolution.Provider.ID)
	if err != nil {
		return Outcome{}, err
	}
	if len(plan.endpoints) == 0 {
		return Outcome{}, domain.NewNoProviderAvailableError(
			"no upstream endpoint is configured for provider " + resolution.Provider.ID)
	}
	var memberErr error
	for {
		// Both bounds are checked before another credential is picked: an expired clock must
		// not also cost the caller a selection, and the attempt count is what catches an
		// upstream answering instantly with an error a thousand times.
		if budget.expired(e.clock()) {
			return outcome, budget.error(resolution.Provider.ID, memberErr)
		}
		selection, ok, nextErr := plan.Next(ctx, spent)
		if nextErr != nil || !ok {
			if memberErr != nil {
				return outcome, memberErr
			}
			if nextErr != nil {
				return Outcome{}, nextErr
			}
			return Outcome{}, domain.NewNoProviderAvailableError(
				"every upstream endpoint for provider " + resolution.Provider.ID +
					" is unavailable, has no usable key, or has spent its budget")
		}
		if !budget.spend() {
			return outcome, budget.error(resolution.Provider.ID, memberErr)
		}
		spent[CandidateID(selection)] = struct{}{}
		outcome.ProviderID = resolution.Provider.ID
		outcome.EndpointID = selection.Endpoint.ID()
		outcome.Model = resolution.ModelID

		// The marker opens once the credential is known and closes when this
		// attempt ends, whichever way it ends, so the drawing lights a node for
		// exactly as long as the provider is being called.
		// The combo the client addressed travels with it: the member resolved here
		// is what answered, and the combo is the stage the request entered through,
		// so the drawing can light both. A nil seam returns a release that does
		// nothing.
		release := e.markActive(ctx, ActiveMarker{
			ProviderID: resolution.Provider.ID,
			EndpointID: outcome.EndpointID,
			Model:      resolution.ModelID,
			Combo:      resolution.Combo.Name(),
		})
		started := e.clock()
		budget.markCalled()
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
			// The walk's clock travels with the call: the transport clamps its
			// in-attempt waits and its first-byte cap to what is left of it, so
			// one credential cannot spend the whole walk before the next is
			// tried.
			Budget: budget,
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
