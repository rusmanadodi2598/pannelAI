// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine.go
// @for       One chat request end to end: resolve, select, translate, call, and
//
//	hand back either a translated body or a stream of frames.
//
// @uses      internal/domain, internal/reasoning, context, time.
// @reason    SPEC-API-001 §7.15 fixes the pipeline order (model resolve → format
//
//	translation → endpoint and key selection → upstream call → response
//	translation → usage recording), and §7.15 adds the reasoning
//	injection the client's model string and the stored mode resolve to.
//	Keeping it in one place is what makes the order auditable, and
//	keeping it out of the handler is what keeps the same path usable
//	from a worker or a combo probe.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/reasoning"
)

// FrameSink, TokenSaver, and ThinkingApplier are the pipeline's boundary ports,
// declared in seams.go, and Outcome is the result shape, declared in outcome.go.

// Engine executes the §7.15 pipeline for one chat request.
type Engine struct {
	resolver  *Resolver
	selector  *Selector
	transport *Transport
	// vision is the optional §7.8 seam. A nil one means no adapter is wired,
	// which serves every request un-augmented.
	vision VisionAugmenter
	// orders is the optional §7.7 combo-order seam. A nil one serves every combo
	// in stored priority order.
	orders ComboOrderer
	// saver rewrites the translated upstream body when an operator enabled a
	// token-saver group. A nil one keeps the pipeline pass-through.
	saver TokenSaver
	// thinking writes the resolved reasoning control into the translated body
	// (§7.15). A nil one leaves the body exactly as translation produced it.
	thinking ThinkingApplier
	// active records one provider as in flight for as long as its call runs, so
	// the live Usage stream can draw which nodes are routing now. A nil seam
	// records nothing.
	active ActiveRequests
	clock  func() time.Time
}

// EngineDeps holds the collaborators the engine needs.
type EngineDeps struct {
	Resolver  *Resolver
	Selector  *Selector
	Transport *Transport
	// Vision augments image-bearing requests aimed at a model that cannot read
	// images. Optional: nil keeps the pipeline free of the adapter entirely.
	Vision VisionAugmenter
	// ComboOrder advances a round-robin combo's position. Optional: nil serves
	// every combo in stored priority order, which is what a deployment without
	// Redis gets instead of a failure.
	ComboOrder ComboOrderer
	// TokenSaver rewrites the translated upstream body when enabled. Optional:
	// nil leaves the data plane pass-through, which is the safe default.
	TokenSaver TokenSaver
	// Thinking writes the resolved reasoning control into the translated body
	// (§7.15). Optional: nil leaves the reasoning fields exactly as the client
	// sent them, which is what a deployment without the settings service gets.
	Thinking ThinkingApplier
	// ActiveRequests marks one provider as in flight while its call runs, which
	// is what the §7.12 live stream draws. Optional: nil leaves the drawing with
	// no active node rather than failing a request.
	ActiveRequests ActiveRequests
	// Clock overrides the time source, so a test can measure latency and the
	// circuit window without sleeping.
	Clock func() time.Time
}

// NewEngine validates deps and returns an engine.
func NewEngine(deps EngineDeps) (*Engine, error) {
	if deps.Resolver == nil {
		return nil, internalError("model resolver is required", nil)
	}
	if deps.Selector == nil {
		return nil, internalError("endpoint selector is required", nil)
	}
	if deps.Transport == nil {
		return nil, internalError("upstream transport is required", nil)
	}
	clock := deps.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Engine{
		resolver: deps.Resolver, selector: deps.Selector, transport: deps.Transport,
		vision: deps.Vision, orders: deps.ComboOrder, saver: deps.TokenSaver,
		thinking: deps.Thinking, active: deps.ActiveRequests, clock: clock,
	}, nil
}

// Resolver exposes the resolver, so a caller can answer catalog questions with
// exactly the rule routing uses.
func (e *Engine) Resolver() *Resolver { return e.resolver }

// Relay runs the pipeline. When the request asks for a stream, every frame is
// written to sink as it arrives and Outcome.Body stays nil.
func (e *Engine) Relay(ctx context.Context, in Request, sink FrameSink) (Outcome, error) {
	// A client may address a model with a trailing "(level)" reasoning suffix
	// (SPEC-API-001 §7.15). The resolver must see the bare id — that is what
	// the catalog and the registry declare — while the injection seam below
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
	// operator's combo and the configured adapter — reporting it as adapted would
	// say the gateway reached for something the caller never named when it did not.
	own := refs
	refs, adapted := e.augmentForVision(ctx, in, refs)

	// The walk tracks two failure kinds apart, because they answer the client
	// differently (draft 028 F3): a member that reached an upstream call owns
	// the error and the recorded identity, while a member refused before any
	// call (unresolvable, no endpoints, an untranslatable body) is only the
	// fallback answer when no member was called at all. The chain's error is
	// the first called failure's status with the last one's message, which is
	// how the reference reports an exhausted combo.
	var firstErr, lastErr error
	var lastOutcome Outcome
	var preCallErr error
	// A member can answer 200 with nothing the client can read, which is a success
	// to the transport and so is not a failure to fail over *from*. It is held
	// aside instead: a later member may answer properly, and if none does, the
	// body is still what the first member would have served. Recording it as an
	// error would park a credential that answered exactly as asked.
	var truncated Outcome
	var hasTruncated bool
	for index, ref := range refs {
		member, resolveErr := e.resolver.Resolve(ctx, ref)
		if resolveErr != nil {
			preCallErr = resolveErr
			continue
		}
		member.Combo = resolution.Combo
		member.Requested = resolution.Requested
		outcome, relayErr := e.relayOnce(ctx, in, member, sink)
		if relayErr == nil {
			// An adapter model the request did not address is not the model that
			// billed, and the usage row belongs to the one the client asked for.
			// Membership of the adapter's own list answers that wherever the seam
			// placed it — which a leading-count test could not once a capable
			// member is allowed to go first — and the second test keeps a member
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
