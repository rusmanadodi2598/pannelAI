// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine.go
// @for       One chat request end to end: resolve, select, translate, call, and
//
//	hand back either a translated body or a stream of frames.
//
// @uses      internal/domain, internal/schema, context, time.
// @reason    SPEC-API-001 §7.15 fixes the pipeline order (model resolve → format
//
//	translation → endpoint and key selection → upstream call → response
//	translation → usage recording). Keeping it in one place is what makes
//	the order auditable, and keeping it out of the handler is what keeps
//	the same path usable from a worker or a combo probe.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// FrameSink receives the frames of a streamed answer. It exists so the data plane
// never touches an http.ResponseWriter (AGENTS.md §1.5 keeps net/http out of the
// service layer) while still flushing each frame as it is produced.
type FrameSink interface {
	// WriteFrame writes one complete SSE frame. A partial write is an error the
	// caller stops the stream on.
	WriteFrame(frame []byte) error
	// Flush pushes what has been written to the client.
	Flush()
}

// Outcome reports what one relayed call produced, in the form accounting and
// logging need.
type Outcome struct {
	// Format is the wire format the caller's answer is written in.
	Format schema.DataPlaneFormat
	// ProviderID, EndpointID, and Model are the routing identity of the call.
	ProviderID string
	EndpointID string
	Model      string
	// Combo names the combo the request addressed, or "".
	Combo string
	// Body is the non-streamed answer, already in Format. It is nil for a
	// streamed answer, which the sink received frame by frame.
	Body []byte
	// Usage is the accounting the upstream reported, or nil when it reported
	// none, so a caller records nothing rather than a fabricated zero.
	Usage *schema.Usage
	// Streamed reports whether the answer went to a FrameSink.
	Streamed bool
	// LatencyMS is the upstream call's duration, measured with the engine's
	// clock.
	LatencyMS int64
}

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
		vision: deps.Vision, orders: deps.ComboOrder, clock: clock,
	}, nil
}

// Resolver exposes the resolver, so a caller can answer catalog questions with
// exactly the rule routing uses.
func (e *Engine) Resolver() *Resolver { return e.resolver }

// Relay runs the pipeline. When the request asks for a stream, every frame is
// written to sink as it arrives and Outcome.Body stays nil.
func (e *Engine) Relay(ctx context.Context, in Request, sink FrameSink) (Outcome, error) {
	resolution, err := e.resolver.Resolve(ctx, in.Model)
	if err != nil {
		return Outcome{}, err
	}

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

	// The §7.8 decision lives beside the seam it consults (vision.go).
	refs, adapterCount := e.augmentForVision(ctx, in, resolution, refs)

	// lastOutcome carries the identity of the last member that was actually
	// attempted, so a failure still tells the caller which provider, endpoint,
	// and model it failed against (register G17: the chat plane records a
	// failed call, and a zero outcome carries nothing to record).
	var lastErr error
	var lastOutcome Outcome
	for index, ref := range refs {
		member, resolveErr := e.resolver.Resolve(ctx, ref)
		if resolveErr != nil {
			lastErr = resolveErr
			continue
		}
		member.Combo = resolution.Combo
		outcome, relayErr := e.relayOnce(ctx, in, member, sink)
		if relayErr == nil {
			if index < adapterCount {
				outcome.Model = resolution.ModelID
			}
			return outcome, nil
		}
		lastErr = relayErr
		if outcome.ProviderID != "" {
			lastOutcome = outcome
		}
		// A client error is not worth failing over from: the same request body
		// would be rejected identically by every other member, and trying them
		// spends accounts for nothing.
		if failure := AsError(relayErr); !failoverWorthy(failure.Code) {
			return outcome, relayErr
		}
	}
	return lastOutcome, lastErr
}
