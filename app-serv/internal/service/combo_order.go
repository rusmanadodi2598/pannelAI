// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_order.go
// @for       The ordered model list one request routes through, the rotation
//
//	wiring that produces it, and the reference validation a combo write
//	runs (SPEC-API-001 §7.7).
//
// @uses      internal/domain, internal/repository, context, log/slog.
// @reason    §7.7 makes write time the only moment a combo's references can be
//
//	checked — a router cannot dereference a stale name — and makes the
//	round-robin order the one part of a combo a client observes request
//	by request. Both concern the path from a stored combo to a served
//	request, which is why they sit together, apart from the CRUD
//	surface.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Order returns the model order one request should try for a combo.
//
// A round-robin combo advances the Redis-backed state, so two concurrent
// requests land on different models; every other strategy keeps the stored
// order, which is the priority order the operator arranged. A rotation store
// that is absent or failing falls back to the stored order rather than failing
// the request: distribution is an optimisation, and losing it must not lose the
// request. The failure is logged, because a rotation that silently stopped
// rotating is behaviour an operator needs to see.
func (s *ComboService) Order(ctx context.Context, combo domain.Combo) ([]string, error) {
	refs := combo.Refs()
	if !combo.Strategy().UsesRotation() || len(refs) < 2 || s.rotation == nil {
		return refs, nil
	}
	order, err := s.rotation.Next(ctx, combo.Name(), refs, combo.StickyLimit())
	if err != nil {
		slog.Warn("combo rotation unavailable; serving the stored order",
			"combo", combo.Name(), "error", err)
		return refs, nil
	}
	return order, nil
}

// validateRefs dereferences every model ref and the judge one level deep (§7.7).
//
// "One dereference level" is the rule that makes write-time validation total: a
// ref may be a model, a combo name, or an alias, and an alias's own target may
// not be another alias. Without that limit, validation would have to walk a
// chain that a later write could turn into a cycle.
func (s *ComboService) validateRefs(ctx context.Context, name string, draft ComboDraft) error {
	view, err := newReferenceView(s.catalog, ctx)
	if err != nil {
		return err
	}
	names, err := s.repo.Names(ctx)
	if err != nil {
		return err
	}
	comboNames := make(map[string]struct{}, len(names))
	for _, name := range names {
		comboNames[name] = struct{}{}
	}
	aliases, err := s.catalog.Aliases(ctx)
	if err != nil {
		return err
	}
	aliasTargets := make(map[string]string, len(aliases))
	for _, alias := range aliases {
		aliasTargets[alias.Alias()] = alias.Target()
	}

	// resolve answers whether one reference names something the data plane
	// could route and serve. A provider/model reference is canonicalized through
	// the same alias-then-id lookup the router performs, so a member spelled with
	// a registry alias (`cc/claude-...`) or a node prefix (`oczen/...`) validates
	// exactly when the router would route it (draft 024 §3.2).
	//
	// An alias member is judged by its target, because that is what the router
	// serves: an alias to a media model or to an untranslatable provider is
	// refused exactly like the direct reference to it.
	//
	// A member naming the combo itself is refused outright: a self-reference is
	// the one-cycle a write can produce without touching the database, and the
	// runtime guard is a backstop for stored rows, not the first line.
	resolve := func(ref string) error {
		if strings.EqualFold(strings.TrimSpace(ref), name) {
			return domain.NewValidationError("a combo cannot list itself")
		}
		if _, ok := comboNames[ref]; ok {
			return nil
		}
		if target, ok := aliasTargets[ref]; ok {
			// An alias may target a combo as well as a model, and a combo is
			// served by its members rather than by a provider of its own: it is
			// accepted here exactly as the direct combo name above is.
			if _, isCombo := comboNames[target]; isCombo {
				return nil
			}
			if strings.EqualFold(strings.TrimSpace(target), name) {
				return domain.NewValidationError("a combo cannot list itself")
			}
			ref = target
		}
		parsed, err := domain.ParseModelRef(ref)
		if err != nil {
			return domain.ErrComboModelRef
		}
		return view.chatServable(parsed)
	}
	for _, model := range draft.Models {
		if err := resolve(model.Ref()); err != nil {
			return err
		}
	}
	if draft.JudgeModel != "" {
		return resolve(draft.JudgeModel)
	}
	return nil
}

// resetRotation clears a combo's rotation state after a change that invalidates
// it: a counter that addressed the old model list would otherwise keep serving
// the wrong model. The failure is discarded deliberately — the combo itself was
// stored successfully, and a stale counter costs one window of skew rather than
// a refused write.
func (s *ComboService) resetRotation(ctx context.Context, comboName string) {
	if s.rotation == nil {
		return
	}
	resetter, ok := s.rotation.(interface {
		Reset(ctx context.Context, comboKey string) error
	})
	if !ok {
		return
	}
	if err := resetter.Reset(ctx, comboName); err != nil {
		slog.Warn("combo rotation state was not reset", "combo", comboName, "error", err)
	}
}

// refsOf extracts the references of a draft's model list.
func refsOf(models []domain.ComboModel) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		out = append(out, model.Ref())
	}
	return out
}

// sameRefs compares two ordered reference lists.
func sameRefs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
