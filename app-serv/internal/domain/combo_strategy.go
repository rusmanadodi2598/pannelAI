// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/combo_strategy.go
// @for       The combo strategy value object and the pure round-robin rotation
//
//	that turns a model list plus its state into the next order.
//
// @uses      strings, internal/domain (AppError constructors).
// @reason    SPEC-API-001 §7.7 ports combo.js, whose rotation is a state
//
//	machine over (model list, sticky limit, state). Keeping it a pure
//	function here means the distribution rule is unit-testable without
//	a clock, a database, or Redis, and the Redis store only persists
//	the state this function returns.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import "strings"

// ComboStrategy is how a combo distributes a request across its models. The set
// is closed and mirrored by a CHECK constraint on combos.strategy.
type ComboStrategy string

const (
	// ComboFallback tries the models in priority order until one answers.
	ComboFallback ComboStrategy = "fallback"
	// ComboRoundRobin rotates the leading model, keeping a sticky count of
	// requests on it before switching.
	ComboRoundRobin ComboStrategy = "round_robin"
	// ComboFusion fans out and lets judge_model write the final answer.
	ComboFusion ComboStrategy = "fusion"
)

// ParseComboStrategy validates a wire value before it reaches the aggregate.
func ParseComboStrategy(value string) (ComboStrategy, error) {
	strategy := ComboStrategy(strings.TrimSpace(value))
	if !strategy.IsValid() {
		return "", NewValidationError("invalid strategy: " + value)
	}
	return strategy, nil
}

// IsValid reports whether the strategy is one the router can execute.
func (s ComboStrategy) IsValid() bool {
	switch s {
	case ComboFallback, ComboRoundRobin, ComboFusion:
		return true
	default:
		return false
	}
}

// UsesRotation reports whether the strategy reads sticky_limit and the stored
// rotation state. The other two strategies are order-fixed, so a rotation write
// for them would be state nobody reads.
func (s ComboStrategy) UsesRotation() bool { return s == ComboRoundRobin }

// RotationState is where a round-robin combo currently sits: which model leads
// (Index) and how many requests it has already served (Uses).
type RotationState struct {
	Index int
	Uses  int
}

// Normalized folds a stored state into the range the current model list can
// serve. A stored index outlives an edit that removed models, so it is reduced
// modulo the list length instead of being trusted.
func (s RotationState) Normalized(length int) RotationState {
	if length <= 0 {
		return RotationState{}
	}
	index := s.Index % length
	if index < 0 {
		index += length
	}
	uses := s.Uses
	if uses < 0 {
		uses = 0
	}
	return RotationState{Index: index, Uses: uses}
}

// NextOrder returns the model order this request uses and the state the next
// request continues from (SPEC-API-001 §7.7, ported from combo.js
// getRotatedModels).
//
// Only round_robin rotates, and a list of fewer than two models has nothing to
// distribute, so both cases return the input order with the state untouched.
// stickyLimit retries on one model before the leading model advances; a value
// below 1 is floored to 1 because a zero would leave the state advancing on
// every request and the caller with a rotation it never asked for.
func (s ComboStrategy) NextOrder(models []string, stickyLimit int, state RotationState) ([]string, RotationState) {
	order := make([]string, len(models))
	copy(order, models)
	if !s.UsesRotation() || len(order) < 2 {
		return order, state
	}
	if stickyLimit < 1 {
		stickyLimit = 1
	}
	current := state.Normalized(len(order))
	rotated := RotateRefs(order, current.Index)
	uses := current.Uses + 1
	if uses >= stickyLimit {
		return rotated, RotationState{Index: (current.Index + 1) % len(order)}
	}
	return rotated, RotationState{Index: current.Index, Uses: uses}
}

// RotationRequestIndex returns the index of the model leading the request at
// position requests (zero-based) of a round-robin combo with length models and
// a sticky limit of stickyLimit consecutive requests per model.
//
// It is the closed form of NextOrder: iterating NextOrder once per request
// visits exactly these indices, which TestRotationRequestIndex_MatchesNextOrder
// pins. The Redis-backed store persists one request counter and derives the
// index from it rather than storing the index and the use count, so the
// distribution rule exists once, here, instead of again inside a Lua script that
// could drift from it.
func RotationRequestIndex(requests, stickyLimit, length int) int {
	if length <= 1 {
		return 0
	}
	if stickyLimit < 1 {
		stickyLimit = 1
	}
	if requests < 0 {
		requests = 0
	}
	return (requests / stickyLimit) % length
}

// RotateRefs shifts the leading n refs to the end, so the model that leads this
// request is the one the caller tries first and the rest stay in priority order
// behind it.
//
// It is exported because the Redis-backed rotation store applies it to the index
// RotationRequestIndex derived from the stored counter: the store owns the
// atomic count and this function owns what an index means, and splitting them
// would let the stored state and the served order disagree.
func RotateRefs(refs []string, n int) []string {
	out := make([]string, len(refs))
	if len(refs) == 0 {
		return out
	}
	offset := n % len(refs)
	if offset < 0 {
		offset += len(refs)
	}
	copy(out, refs[offset:])
	copy(out[len(refs)-offset:], refs[:offset])
	return out
}
