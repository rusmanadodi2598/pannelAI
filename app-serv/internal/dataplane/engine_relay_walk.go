// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_relay_walk.go
// @for       The exported Relay: resolve the model string, then walk a combo's members in order until one answers, and decide what the next member is worth.
// @uses      context, internal/domain, internal/reasoning.
// @reason    The member walk is the half of the request that can spend several accounts, and its three exits (a served member, an answer already streaming to the client, a class the router stopped on) are the rules a reviewer has to read together. Keeping them in one file beside the credential walk it delegates to is what makes the whole chain auditable without also keeping the engine's wiring in it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/reasoning"
)

// Relay runs the pipeline. When the request asks for a stream, every frame is
// written to sink as it arrives and Outcome.Body stays nil.
func (e *Engine) Relay(ctx context.Context, in Request, sink FrameSink) (Outcome, error) {
	// A client may address a model with a trailing "(level)" reasoning suffix
	// (SPEC-API-001 §7.15). The resolver must see the bare id, that is what
	// the catalog and the registry declare, while the injection seam below
	// must see the override, so the string is split once here and carried on
	// the request for every leg of the call.
	model, override := reasoning.ParseSuffix(in.Model)
	in.ThinkingOverride = override

	resolution, err := e.resolver.Resolve(ctx, model)
	if err != nil {
		return Outcome{}, err
	}
	// The name the answer carries is the string this caller sent, so a client that
	// reads `model` back can send it again: the resolved id drops the provider
	// prefix and an upstream alias (`auto`, `deepseek-v4.1-flash`) is not a name
	// the gateway routes.
	resolution.Requested = model

	// A fusion combo fans out to every member and has a judge synthesize the
	// final answer (SPEC-API-001 §7.7, strategy fusion), so it never walks the
	// member order below.
	if resolution.Combo.Strategy() == domain.ComboFusion {
		return e.relayFusion(ctx, in, resolution, sink)
	}

	// A combo tries its members in order (SPEC-API-001 §7.7, strategy fallback):
	// the first member is where the request starts, and a failure that the
	// endpoint layer reports as retryable-elsewhere moves to the next one.
	refs := resolution.Combo.Refs()
	if len(refs) == 0 {
		refs = []string{resolution.Provider.ID + "/" + resolution.ModelID}
	}
	// A round_robin combo rotates its leading member; every other strategy
	// keeps the stored order.
	refs = e.rotate(ctx, resolution, refs)

	// The §7.8 decision lives beside the seam it consults (vision.go). The list the
	// request itself produced is kept, because an adapter ref that happens to also
	// be one of this request's own members is not a substitution: the combo chose
	// that model on its own and the seam merely moved it. Measured live with
	// opencode/muse-spark-1.3-contributor-free, which is both a member of the
	// operator's combo and the configured adapter, reporting it as adapted would
	// say the gateway reached for something the caller never named when it did not.
	own := refs
	refs, adapted := e.augmentForVision(ctx, in, refs)

	// The walk tracks two failure kinds apart, because they answer the client
	// differently: a member that reached an upstream call owns
	// the error and the recorded identity, while a member refused before any
	// call (unresolvable, no endpoints, an untranslatable body) is only the
	// fallback answer when no member was called at all. The chain's error is
	// the first called failure's status with the last one's message, which is
	// how the reference reports an exhausted combo.
	var firstErr, lastErr error
	var lastOutcome Outcome
	var preCallErr error
	// One budget per request, so a combo of many members shares it.
	budget := e.newRequestBudget(ctx)
	// A member can answer 200 with nothing the client can read, which is a success
	// to the transport and so is not a failure to fail over *from*. It is held
	// aside instead: a later member may answer properly, and if none does, the
	// body is still what the first member would have served. Recording it as an
	// error would park a credential that answered exactly as asked.
	var truncated Outcome
	var hasTruncated bool
	for index, ref := range refs {
		// A seventh leg is bought with time the caller stopped giving.
		if budget.expired(e.clock()) {
			return lastOutcome, budget.error(resolution.Provider.ID, lastErr)
		}
		member, resolveErr := e.resolver.Resolve(ctx, ref)
		if resolveErr != nil {
			preCallErr = resolveErr
			continue
		}
		member.Combo = resolution.Combo
		member.Requested = resolution.Requested
		outcome, relayErr := e.relayOnce(ctx, in, member, sink, budget)
		if relayErr == nil {
			// An adapter model the request did not address is not the model that
			// billed, and the usage row belongs to the one the client asked for.
			// Membership of the adapter's own list answers that wherever the seam
			// placed it, which a leading-count test could not once a capable
			// member is allowed to go first, and the second test keeps a member
			// the combo already addressed from being reported as borrowed.
			if containsRef(adapted, ref) && !containsRef(own, ref) {
				outcome.Model = resolution.ModelID
				outcome.VisionAdapted = true
			}
			if outcome.Truncated && index < len(refs)-1 {
				truncated, hasTruncated = outcome, true
				continue
			}
			return outcome, nil
		}
		if outcome.ProviderID == "" {
			preCallErr = relayErr
			continue
		}
		if outcome.FramesWritten {
			// The client is already reading this member's answer, so this leg is
			// the end of the walk: another member would append its answer to the
			// stream the client has open and pay twice for one request.
			return outcome, relayErr
		}
		// A class the router stopped on is the end of the request, not of one
		// member: the failure says no other credential answers it differently, and
		// the next member is another credential.
		if outcome.ChainStopped {
			return outcome, relayErr
		}
		lastErr = relayErr
		if firstErr == nil {
			firstErr = relayErr
		}
		lastOutcome = outcome
		// A client error is not worth failing over from: the same request body
		// would be rejected identically by every other member, and trying them
		// spends accounts for nothing.
		if failure := AsError(relayErr); !failoverWorthy(failure.Code) {
			return outcome, relayErr
		}
	}
	if hasTruncated {
		return truncated, nil
	}
	if lastErr != nil {
		return lastOutcome, finalError(firstErr, lastErr)
	}
	return Outcome{}, preCallErr
}
