// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/combo.go
// @for       The Combo aggregate root: an ordered model list a client addresses
//
//	as one model string (SPEC-API-001 §7.7).
//
// @uses      internal/domain (ComboModel, ComboStrategy, AppError
//
//	constructors), strings, time.
//
// @reason    A combo is a model string the data plane resolves, so its shape —
//
//	which strategies may carry a judge, which ones need a sticky
//	limit, and that it names at least one model — is a routing rule
//	rather than a request-shape rule. Keeping it here means the panel
//	and the data plane cannot disagree about what a valid combo is.
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

// ComboIDPrefix is the type prefix for a combo id (SPEC-API-001 §4).
const ComboIDPrefix = "cmb_"

// Combo is an ordered model list with a strategy. Fields are unexported on
// purpose (AGENTS.md §2.2): strategy rules are enforced by Update, so no caller
// can leave a combo in a shape the router cannot execute.
type Combo struct {
	id          string
	name        string
	strategy    ComboStrategy
	stickyLimit int
	judgeModel  string
	models      []ComboModel
	createdAt   time.Time
	updatedAt   time.Time
}

// NewCombo is the only constructor for a new combo.
//
// stickyLimit is normalized for the strategy: round_robin requires at least one
// request per model, and the strategies that ignore it store the default rather
// than whatever was sent, so a combo that switches strategy does not silently
// inherit a number nothing reads.
func NewCombo(id, name string, strategy ComboStrategy, stickyLimit int, judgeModel string, models []ComboModel, now time.Time) (Combo, error) {
	validatedName, err := validateComboName(name)
	if err != nil {
		return Combo{}, err
	}
	shape, err := validateComboShape(strategy, stickyLimit, judgeModel, models)
	if err != nil {
		return Combo{}, err
	}
	instant := now.UTC()
	return Combo{
		id:          id,
		name:        validatedName,
		strategy:    strategy,
		stickyLimit: shape.stickyLimit,
		judgeModel:  shape.judgeModel,
		models:      shape.models,
		createdAt:   instant,
		updatedAt:   instant,
	}, nil
}

// RehydrateCombo rebuilds a stored row. For the repository load path only;
// never use it to create a combo: it skips the strategy rules on purpose,
// because a row that violates them was written by an older build and must still
// load so the API can report and repair it.
func RehydrateCombo(id, name string, strategy ComboStrategy, stickyLimit int, judgeModel string, models []ComboModel, createdAt, updatedAt time.Time) Combo {
	return Combo{
		id:          id,
		name:        name,
		strategy:    strategy,
		stickyLimit: stickyLimit,
		judgeModel:  judgeModel,
		models:      models,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

// Accessors expose state without allowing mutation.
func (c Combo) ID() string              { return c.id }
func (c Combo) Name() string            { return c.name }
func (c Combo) Strategy() ComboStrategy { return c.strategy }
func (c Combo) StickyLimit() int        { return c.stickyLimit }
func (c Combo) JudgeModel() string      { return c.judgeModel }
func (c Combo) CreatedAt() time.Time    { return c.createdAt }
func (c Combo) UpdatedAt() time.Time    { return c.updatedAt }

// Models returns a copy of the ordered list, so a caller cannot reorder the
// aggregate's own collection.
func (c Combo) Models() []ComboModel {
	out := make([]ComboModel, len(c.models))
	copy(out, c.models)
	return out
}

// ModelCount reports how many models the combo tries.
func (c Combo) ModelCount() int { return len(c.models) }

// Refs returns the ordered model strings: what the router walks on the fallback
// path and what the round-robin rotation rotates.
func (c Combo) Refs() []string {
	out := make([]string, 0, len(c.models))
	for _, m := range c.models {
		out = append(out, m.ref)
	}
	return out
}

// HasRef reports whether the combo already names this reference, which is how
// the service tells "this alias points at this combo" from a coincidental match.
func (c Combo) HasRef(ref string) bool {
	target := strings.TrimSpace(ref)
	for _, m := range c.models {
		if m.ref == target {
			return true
		}
	}
	return false
}

// Update applies the PATCH of §7.7 to the whole aggregate. Every field is
// replaced by the caller's value, because the panel edits the model list as a
// set and a partial merge would leave the ordering half-applied.
func (c *Combo) Update(name string, strategy ComboStrategy, stickyLimit int, judgeModel string, models []ComboModel, now time.Time) error {
	validatedName, err := validateComboName(name)
	if err != nil {
		return err
	}
	shape, err := validateComboShape(strategy, stickyLimit, judgeModel, models)
	if err != nil {
		return err
	}
	c.name = validatedName
	c.strategy = strategy
	c.stickyLimit = shape.stickyLimit
	c.judgeModel = shape.judgeModel
	c.models = shape.models
	c.updatedAt = now.UTC()
	return nil
}

// NextOrder returns the model order one request uses and the rotation state the
// next request continues from, so the service never reimplements the
// distribution rule the aggregate owns.
func (c Combo) NextOrder(state RotationState) ([]string, RotationState) {
	return c.strategy.NextOrder(c.Refs(), c.stickyLimit, state)
}
