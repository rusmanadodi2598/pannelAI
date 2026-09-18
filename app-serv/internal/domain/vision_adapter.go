// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/vision_adapter.go
// @for       The VisionAdapter aggregate root: the capability fallback that
//
//	routes an image-bearing request to a vision-capable model
//	(SPEC-API-001 §7.8).
//
// @uses      internal/domain (ModelRef, ComboStrategy, AppError constructors),
//
//	sort, strings, time.
//
// @reason    The adapater is a routing rule — when it fires, how it rotates,
//
//	and which models it may name — so it is an aggregate with its own
//	invariants rather than a settings blob. Its ordering rule is the
//	same ComboStrategy.NextOrder the combo round-robin uses, because
//	the reference rotates both the same way, and duplicating that
//	would let the two rotations drift.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"time"
)

// VisionCapabilityCheck reports whether a catalog model is vision-capable.
//
// This is the seam the capability data will arrive through. SPEC-API-001 §7.8
// validates adapter models against a "vision" capability, but the capability
// table is NOT in the embedded registry yet: the reference resolves it from a
// separate pattern table that the registry port does not carry. Rather than
// hardcode model-name heuristics here — which would silently mis-classify every
// model the heuristics do not know — the aggregate takes the predicate as a
// dependency, and the catalog supplies it once the capability data lands.
// Until then the service injects a predicate that rejects everything, so the
// API never claims a model is vision-capable on no evidence.
type VisionCapabilityCheck func(ref ModelRef) bool

// VisionAdapter is the capability fallback configuration. Fields are unexported
// on purpose (AGENTS.md §2.2): every change goes through Replace, so a caller
// cannot store a model list the predicate would reject.
type VisionAdapter struct {
	enabled    bool
	roundRobin bool
	models     []ModelRef
	updatedAt  time.Time
}

// DefaultVisionAdapter returns the adapter a fresh install serves: disabled and
// empty, because enabling it on no models would change routing with no effect
// (SPEC-UI §6.4 renders exactly that state as a warning).
func DefaultVisionAdapter() VisionAdapter {
	return VisionAdapter{models: make([]ModelRef, 0)}
}

// NewVisionAdapter validates a replacement configuration. capable is the
// capability predicate described on VisionCapabilityCheck; a nil predicate
// rejects every model rather than accepting any, because "we cannot tell"
// must not read as "yes".
func NewVisionAdapter(enabled, roundRobin bool, models []ModelRef, capable VisionCapabilityCheck, now time.Time) (VisionAdapter, error) {
	adapter := VisionAdapter{models: make([]ModelRef, 0, len(models))}
	if err := adapter.Replace(enabled, roundRobin, models, capable, now); err != nil {
		return VisionAdapter{}, err
	}
	return adapter, nil
}

// RehydrateVisionAdapter rebuilds the stored configuration. For the repository
// load path only: a stored list was validated when it was written, and
// re-validating on read would turn a capability-table change into a 500.
func RehydrateVisionAdapter(enabled, roundRobin bool, models []ModelRef, updatedAt time.Time) VisionAdapter {
	out := VisionAdapter{enabled: enabled, roundRobin: roundRobin, updatedAt: updatedAt}
	out.models = make([]ModelRef, len(models))
	copy(out.models, models)
	return out
}

// Replace applies a whole-configuration PUT (§7.8). The model list is replaced
// as a set, never merged, so a removal cannot leave a stale entry behind.
func (a *VisionAdapter) Replace(enabled, roundRobin bool, models []ModelRef, capable VisionCapabilityCheck, now time.Time) error {
	// A nil predicate rejects every model, which is the documented reading of the
	// seam: "we cannot tell" must not become "yes". Checking once here rather than
	// at each call keeps the rule from depending on which branch runs first.
	if capable == nil {
		capable = func(ModelRef) bool { return false }
	}
	unique := make([]ModelRef, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, ref := range models {
		if ref.IsZero() {
			return NewValidationError("vision adapter models must be provider/model references")
		}
		if !capable(ref) {
			return NewValidationError("model is not vision-capable: " + ref.String())
		}
		if _, dup := seen[ref.String()]; dup {
			continue
		}
		seen[ref.String()] = struct{}{}
		unique = append(unique, ref)
	}
	a.enabled = enabled
	a.roundRobin = roundRobin
	a.models = unique
	a.updatedAt = now.UTC()
	return nil
}

// Enabled reports whether image-bearing requests are adapted at all.
func (a VisionAdapter) Enabled() bool { return a.enabled }

// RoundRobin reports whether successive adaptions rotate the model list.
func (a VisionAdapter) RoundRobin() bool { return a.roundRobin }

// UpdatedAt reports when the configuration last changed.
func (a VisionAdapter) UpdatedAt() time.Time { return a.updatedAt }

// Models returns a copy of the configured model list.
func (a VisionAdapter) Models() []ModelRef {
	out := make([]ModelRef, len(a.models))
	copy(out, a.models)
	return out
}

// ModelStrings renders the configured list for the rotation helper, which
// operates on model strings.
func (a VisionAdapter) ModelStrings() []string {
	out := make([]string, 0, len(a.models))
	for _, ref := range a.models {
		out = append(out, ref.String())
	}
	return out
}

// Active reports whether the adapter would prepend models to an image-bearing
// request: it must be enabled and name at least one model.
func (a VisionAdapter) Active() bool { return a.enabled && len(a.models) > 0 }

// NextOrder returns the model order this request adapts to and the state the
// next request continues from. The rotation is the combo rotation: a
// non-round-robin adapter keeps its configured order, and a one-model list has
// nothing to distribute. stickyLimit is fixed at one here because §7.8 defines
// round_robin as alternating, with no sticky count of its own.
func (a VisionAdapter) NextOrder(state RotationState) ([]string, RotationState) {
	strategy := ComboFallback
	if a.roundRobin {
		strategy = ComboRoundRobin
	}
	sortable := make([]string, 0, len(a.models))
	for _, ref := range a.models {
		sortable = append(sortable, ref.String())
	}
	return strategy.NextOrder(sortable, 1, state)
}

// Includes reports whether a model string is one of the adapter's models, which
// is how the data plane recognises a response identity it must strip (§7.8).
func (a VisionAdapter) Includes(ref string) bool {
	target := strings.TrimSpace(ref)
	for _, model := range a.models {
		if model.String() == target {
			return true
		}
	}
	return false
}
