// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine.go
// @for       The Engine itself: its seams, its dependency contract, and the resolver it hands back to catalog questions.
// @uses      log/slog, time, internal/registry, internal/repository.
// @reason    SPEC-API-001 §7.15 fixes the pipeline order (model resolve → format translation → endpoint and key selection → upstream call → response translation → usage recording), and §7.15 adds the reasoning injection the client's model string and the stored mode resolve to. Keeping it in one place is what makes the order auditable, and keeping it out of the handler is what keeps the same path usable from a worker or a combo probe.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import (
	"log/slog"
	"time"
)

// FrameSink, TokenSaver, and ThinkingApplier are the pipeline's boundary ports,
// declared in seams.go, and Outcome is the result shape, declared in outcome.go.

// Engine executes the §7.15 pipeline for one chat request.
type Engine struct {
	resolver  *Resolver
	selector  *Selector
	transport *Transport
	// budgets answers the failover budget. Optional: nil applies the documented one.
	budgets FailoverBudgetReader
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
	// logger records the accounting writes the relay swallows on purpose: a health
	// row that did not land must never replace the answer a client already got,
	// but it still has to be findable (AGENTS.md §1.6).
	logger *slog.Logger
	clock  func() time.Time
}

// EngineDeps holds the collaborators the engine needs, of which Resolver,
// Selector and Transport are the only ones NewEngine refuses to run without.
type EngineDeps struct {
	Resolver  *Resolver
	Selector  *Selector
	Transport *Transport
	// Budgets answers the §7.7 failover budget. Optional: nil gets the documented bound.
	Budgets FailoverBudgetReader
	// Logger records the degraded paths the relay chooses to swallow. Optional:
	// nil writes to slog.Default(), so no wiring can hide a failed health row.
	Logger *slog.Logger
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

// NewEngine rejects a nil resolver, selector or transport and defaults the clock
// to time.Now, so a wiring that states no seams still measures latency.
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
		budgets: deps.Budgets,
		vision:  deps.Vision, orders: deps.ComboOrder, saver: deps.TokenSaver,
		thinking: deps.Thinking, active: deps.ActiveRequests, logger: deps.Logger, clock: clock,
	}, nil
}

// Resolver exposes the resolver, so a caller can answer catalog questions with
// exactly the rule routing uses.
func (e *Engine) Resolver() *Resolver { return e.resolver }
