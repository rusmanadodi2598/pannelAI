// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/operator_marks.go
// @for       The operator's own marks on an account or a key: its tags, and the disable with the window it runs out at.
// @uses      time, internal/domain (Tag, error constructors).
// @reason    A bulk action names its rows by tag and parks them for a window rather than forever, and both facts are the operator's decision, which no health path may overwrite. They are grouped in one transport type so a stored row gains one Rehydrate argument, the way EndpointParity and KeyRotationState each did, and the rules stay on the roots' methods.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-16
package domain

import "time"

// OperatorState carries the operator's marks of a stored row to its aggregate. It
// transports a read and holds no rule, like the two state carriers before it.
type OperatorState struct {
	Tags          []Tag
	DisabledUntil *time.Time
}

// DisabledWindowDefault is how long an operator's park holds an account when the
// provider names no reset cadence: most free-tier and event allowances here return on a
// daily cycle, so a shorter park would release an account back into a wall the vendor
// has not finished rebuilding.
const DisabledWindowDefault = 24 * time.Hour

// TagSet is the tag collection an aggregate root owns. It is a named type so both roots
// share the add and remove rules below instead of each carrying a copy that can drift
// from the other.
type TagSet []Tag

// Add marks one tag onto the set and answers false when it was already there, so a
// caller can tell a change from a no-op without diffing the set itself. The bound is
// enforced on add rather than on write so a row cannot accumulate past what the
// containment index is sized for.
func (t *TagSet) Add(raw string) (bool, error) {
	tag, err := NewTag(raw)
	if err != nil {
		return false, err
	}
	for _, held := range *t {
		if held == tag {
			return false, nil
		}
	}
	if len(*t) >= MaxTagsPerRow {
		return false, NewValidationError("a row may hold at most 16 tags")
	}
	*t = append(*t, tag)
	return true, nil
}

// Remove drops one tag and answers whether it was there.
func (t *TagSet) Remove(raw string) (bool, error) {
	tag, err := NewTag(raw)
	if err != nil {
		return false, err
	}
	for i, held := range *t {
		if held != tag {
			continue
		}
		*t = append((*t)[:i], (*t)[i+1:]...)
		return true, nil
	}
	return false, nil
}

// Copy returns the set as its own slice, so a read cannot hand out the aggregate's
// storage to a caller that would append to it.
func (t TagSet) Copy() []Tag {
	out := make([]Tag, len(t))
	copy(out, t)
	return out
}

// parkWindow answers what a disable written at `now` records as its deadline: a zero or
// already-elapsed window is a permanent switch, stored with no deadline rather than one
// a reader would immediately treat as expired.
func parkWindow(until, now time.Time) *time.Time {
	if until.IsZero() || !until.After(now) {
		return nil
	}
	return &until
}
