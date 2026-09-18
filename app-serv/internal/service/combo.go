// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo.go
// @for       The combo lifecycle: create, list, inspect, update, and delete
//
//	(SPEC-API-001 §7.7).
//
// @uses      internal/domain, internal/repository, context, strings, time.
// @reason    §7.7 makes a combo a model string the data plane resolves, which
//
//	puts two rules here that the aggregate cannot own alone: every ref
//	must dereference one level to a model, a combo, or an alias at
//	write time, and a delete must refuse while an alias still points at
//	the combo. Both span aggregates, so the service is their home.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ComboDraft is the validated input of a create or a full update: the wire
// fields, already parsed into domain types.
type ComboDraft struct {
	Name        string
	Strategy    domain.ComboStrategy
	StickyLimit int
	JudgeModel  string
	Models      []domain.ComboModel
}

// ComboService implements SPEC-API-001 §7.7.
type ComboService struct {
	repo     repository.ComboRepository
	catalog  *ModelCatalogService
	rotation repository.ComboRotationStore
	clock    func() time.Time
}

// ComboServiceDeps holds the collaborators the service needs.
type ComboServiceDeps struct {
	Repo     repository.ComboRepository
	Catalog  *ModelCatalogService
	Rotation repository.ComboRotationStore
}

// NewComboService validates deps and returns a ready service.
//
// Rotation may be nil: the management plane never needs it, and a deployment
// without Redis still has to serve combo CRUD. Order reports an error in that
// case rather than returning an order it cannot justify.
func NewComboService(deps ComboServiceDeps) (*ComboService, error) {
	if deps.Repo == nil {
		return nil, domain.NewValidationError("combo service requires a combo repository")
	}
	if deps.Catalog == nil {
		return nil, domain.NewValidationError("combo service requires the model catalog service")
	}
	return &ComboService{repo: deps.Repo, catalog: deps.Catalog, rotation: deps.Rotation, clock: time.Now}, nil
}

// Create registers a new combo (§7.7 POST).
func (s *ComboService) Create(ctx context.Context, draft ComboDraft) (domain.Combo, error) {
	if err := s.validateRefs(ctx, draft); err != nil {
		return domain.Combo{}, err
	}
	now := s.clock()
	combo, err := domain.NewCombo(domain.ComboIDPrefix+domain.NewULID(now), draft.Name,
		draft.Strategy, draft.StickyLimit, draft.JudgeModel, draft.Models, now)
	if err != nil {
		return domain.Combo{}, err
	}
	if err := s.repo.Create(ctx, combo); err != nil {
		return domain.Combo{}, err
	}
	return combo, nil
}

// List returns one page of combos.
func (s *ComboService) List(ctx context.Context, page, perPage int) ([]domain.Combo, int64, error) {
	return s.repo.List(ctx, repository.PageQuery{Page: page, PerPage: perPage})
}

// Get returns one combo by id.
func (s *ComboService) Get(ctx context.Context, id string) (domain.Combo, error) {
	return s.repo.GetByID(ctx, id)
}

// ByName returns one combo by the name a client sends as a model string.
func (s *ComboService) ByName(ctx context.Context, name string) (domain.Combo, error) {
	return s.repo.GetByName(ctx, name)
}

// Update applies the full-field PATCH of §7.7.
//
// A rename is checked against the stored name first, so a PATCH that keeps the
// name does not collide with itself. The rotation state is reset when the model
// list or the strategy changes: a sticky index that addressed the old list
// would otherwise keep serving the wrong model, which is the one failure a
// round-robin combo can produce that reads like a routing bug.
func (s *ComboService) Update(ctx context.Context, id string, draft ComboDraft) (domain.Combo, error) {
	combo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Combo{}, err
	}
	if err := s.validateRefs(ctx, draft); err != nil {
		return domain.Combo{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(draft.Name), combo.Name()) {
		taken, err := s.repo.ExistsByName(ctx, strings.TrimSpace(draft.Name))
		if err != nil {
			return domain.Combo{}, err
		}
		if taken {
			return domain.Combo{}, domain.ErrComboExists
		}
	}
	rotationChanged := combo.Strategy() != draft.Strategy || !sameRefs(combo.Refs(), refsOf(draft.Models))
	if err := combo.Update(draft.Name, draft.Strategy, draft.StickyLimit, draft.JudgeModel, draft.Models, s.clock()); err != nil {
		return domain.Combo{}, err
	}
	if err := s.repo.Update(ctx, combo); err != nil {
		return domain.Combo{}, err
	}
	if rotationChanged {
		s.resetRotation(ctx, combo.Name())
	}
	return combo, nil
}

// Delete removes a combo, refusing while an alias still points at its name
// (§7.7: the reference must be cleaned first, and the answer is CONFLICT).
func (s *ComboService) Delete(ctx context.Context, id string) error {
	combo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	aliases, err := s.catalog.Aliases(ctx)
	if err != nil {
		return err
	}
	for _, alias := range aliases {
		if alias.Target() == combo.Name() {
			return domain.NewConflictError("alias " + alias.Alias() + " still references combo " + combo.Name())
		}
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.resetRotation(ctx, combo.Name())
	return nil
}
