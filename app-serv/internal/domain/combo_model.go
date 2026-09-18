// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/combo_model.go
// @for       A combo's model entry and the shape rules its strategy imposes.
// @uses      internal/domain (ComboStrategy, AppError constructors), sort,
//
//	strings.
//
// @reason    SPEC-API-001 §7.7 gives each strategy a different required field
//
//	set, and the three rules have to be enforced in one place or a
//	create and a patch will drift. Keeping the entry type and the
//	rules together — away from the aggregate's mutation surface —
//	makes the rule table readable on one screen.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"sort"
	"strings"
)

// comboNameMax bounds a combo name, which is also a model string a client types.
const comboNameMax = 120

// ComboModel is one entry of a combo's ordered list: a reference and the
// priority the list sorts by. Priority is carried explicitly rather than
// implied by slice position because the panel reorders rows by editing it.
type ComboModel struct {
	ref      string
	priority int
}

// NewComboModel validates one entry. The reference is shape-checked here; the
// service resolves it against the catalog and the combo table, because one
// level of dereference is a cross-aggregate question.
func NewComboModel(ref string, priority int) (ComboModel, error) {
	validated, err := validateComboRef(ref)
	if err != nil {
		return ComboModel{}, err
	}
	if priority < 0 {
		return ComboModel{}, NewValidationError("model priority must be zero or greater")
	}
	return ComboModel{ref: validated, priority: priority}, nil
}

// RehydrateComboModel rebuilds a stored entry. For the repository load path
// only, which must not re-run shape validation on a row that already exists.
func RehydrateComboModel(ref string, priority int) ComboModel {
	return ComboModel{ref: ref, priority: priority}
}

// Ref reports the model string this entry names: a provider/model reference, a
// combo name, or an alias.
func (m ComboModel) Ref() string { return m.ref }

// Priority reports the sort key.
func (m ComboModel) Priority() int { return m.priority }

// comboShape is the validated projection of a combo's configurable fields.
type comboShape struct {
	stickyLimit int
	judgeModel  string
	models      []ComboModel
}

// validateComboShape is the single place the strategy rules live (SPEC-API-001
// §7.7):
//
//   - fusion requires judge_model, and a sticky limit is rejected because
//     fusion fans out rather than rotating;
//   - fallback and round_robin reject judge_model, because nothing would call
//     the judge;
//   - round_robin requires sticky_limit >= 1, since zero consecutive requests
//     per model is not a rotation;
//   - every strategy refuses an empty model list: a combo with nothing to try
//     is a routing failure waiting to happen, not a configuration.
//
// A sticky limit sent for a strategy that ignores it is normalized rather than
// rejected: the panel hides the control, so the value is not a user statement.
func validateComboShape(strategy ComboStrategy, stickyLimit int, judgeModel string, models []ComboModel) (comboShape, error) {
	if !strategy.IsValid() {
		return comboShape{}, NewValidationError("invalid strategy: " + string(strategy))
	}
	judge, err := normalizeComboRef(judgeModel)
	if err != nil {
		return comboShape{}, err
	}
	if strategy == ComboFusion {
		if judge == "" {
			return comboShape{}, NewValidationError("strategy fusion requires judge_model")
		}
		if stickyLimit != 0 {
			return comboShape{}, NewValidationError("strategy fusion does not accept sticky_limit")
		}
	} else if judge != "" {
		return comboShape{}, NewValidationError("strategy " + string(strategy) + " does not accept judge_model")
	}
	if strategy == ComboRoundRobin {
		if stickyLimit < 1 {
			return comboShape{}, NewValidationError("strategy round_robin requires sticky_limit of at least 1")
		}
	} else {
		stickyLimit = 1
	}
	if len(models) == 0 {
		return comboShape{}, NewValidationError("a combo requires at least one model")
	}
	ordered, err := orderComboModels(models)
	if err != nil {
		return comboShape{}, err
	}
	return comboShape{stickyLimit: stickyLimit, judgeModel: judge, models: ordered}, nil
}

// normalizeComboRef shape-checks an optional reference. An empty value stays
// empty, which is how "no judge" is spelled.
func normalizeComboRef(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return validateComboRef(value)
}

// orderComboModels validates every reference's shape, rejects a duplicated
// reference, and sorts by priority. A duplicate cannot mean anything the router
// could execute, and a stable sort keeps equal priorities in the order the
// caller sent, so the panel's manual ordering survives a round-trip.
//
// The shape is re-checked here rather than trusted from NewComboModel: the
// aggregate is the mutation boundary (AGENTS.md §2.2), so a combo assembled from
// literals must be refused exactly like one built through the entry constructor.
// A reference carrying whitespace would otherwise reach the router and be parsed
// as two different model strings.
func orderComboModels(models []ComboModel) ([]ComboModel, error) {
	ordered := make([]ComboModel, len(models))
	copy(ordered, models)
	for i, m := range ordered {
		validated, err := validateComboRef(m.ref)
		if err != nil {
			return nil, err
		}
		ordered[i].ref = validated
	}
	sort.SliceStable(ordered, func(a, b int) bool { return ordered[a].priority < ordered[b].priority })
	seen := make(map[string]struct{}, len(ordered))
	for _, m := range ordered {
		if _, dup := seen[m.ref]; dup {
			return nil, NewValidationError("model reference is listed twice: " + m.ref)
		}
		seen[m.ref] = struct{}{}
	}
	return ordered, nil
}

// validateComboName normalizes a combo name. It is a model string a client
// types, so it may not contain a slash (that form is a provider/model
// reference) or whitespace, and it keeps the character set the reference's
// VALID_NAME_REGEX accepted.
func validateComboName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", NewValidationError("name is required")
	}
	if len(trimmed) > comboNameMax {
		return "", NewValidationError("name must be at most 120 characters")
	}
	if !isComboName(trimmed) {
		return "", NewValidationError("name may only contain letters, numbers, dot, dash, and underscore")
	}
	return trimmed, nil
}

// isComboName reports whether a string can be a combo name and therefore a
// model string: no slash and no character outside the accepted set.
func isComboName(value string) bool {
	if value == "" || strings.Contains(value, "/") {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// Combo sentinels, carrying the §8 codes.
var (
	ErrComboNotFound   = NewNotFoundError("combo not found")
	ErrComboExists     = NewConflictError("a combo with this name already exists")
	ErrComboReferenced = NewConflictError("an alias still references this combo")
	ErrComboModelRef   = NewValidationError("model reference does not resolve to a model, combo, or alias")
)
