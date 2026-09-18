// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/model_stub_test.go
// @for       In-memory catalog, combo, and adapter repositories plus the fixture (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/domain, internal/registry, internal/repository,
// @reason    The handlers are exercised through the real service layer over the
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// stubCatalogRepo is an in-memory ModelCatalogRepository.
type stubCatalogRepo struct {
	custom   map[string]domain.CustomModel
	aliases  []domain.ModelAlias
	disabled []domain.ModelRef
}

func newStubCatalogRepo() *stubCatalogRepo {
	return &stubCatalogRepo{custom: map[string]domain.CustomModel{}, disabled: []domain.ModelRef{}}
}

func (r *stubCatalogRepo) Custom(context.Context) ([]domain.CustomModel, error) {
	out := make([]domain.CustomModel, 0, len(r.custom))
	for _, model := range r.custom {
		out = append(out, model)
	}
	return out, nil
}

func (r *stubCatalogRepo) AddCustom(_ context.Context, model domain.CustomModel) error {
	for _, existing := range r.custom {
		if existing.Ref().String() == model.Ref().String() {
			return domain.ErrModelExists
		}
	}
	r.custom[model.ID()] = model
	return nil
}

func (r *stubCatalogRepo) RemoveCustom(_ context.Context, id string) error {
	if _, ok := r.custom[id]; !ok {
		return domain.ErrModelNotFound
	}
	delete(r.custom, id)
	return nil
}

func (r *stubCatalogRepo) Aliases(context.Context) ([]domain.ModelAlias, error) {
	out := make([]domain.ModelAlias, len(r.aliases))
	copy(out, r.aliases)
	return out, nil
}

func (r *stubCatalogRepo) ReplaceAliases(_ context.Context, aliases []domain.ModelAlias) error {
	r.aliases = make([]domain.ModelAlias, len(aliases))
	copy(r.aliases, aliases)
	return nil
}

func (r *stubCatalogRepo) Disabled(context.Context) ([]domain.ModelRef, error) {
	out := make([]domain.ModelRef, len(r.disabled))
	copy(out, r.disabled)
	return out, nil
}

func (r *stubCatalogRepo) ReplaceDisabled(_ context.Context, refs []domain.ModelRef) error {
	r.disabled = make([]domain.ModelRef, len(refs))
	copy(r.disabled, refs)
	return nil
}

// stubComboRepo is an in-memory ComboRepository.
type stubComboRepo struct {
	byID   map[string]domain.Combo
	nameID map[string]string
}

func newStubComboRepo() *stubComboRepo {
	return &stubComboRepo{byID: map[string]domain.Combo{}, nameID: map[string]string{}}
}

func (r *stubComboRepo) Create(_ context.Context, combo domain.Combo) error {
	if _, dup := r.nameID[combo.Name()]; dup {
		return domain.ErrComboExists
	}
	r.byID[combo.ID()] = combo
	r.nameID[combo.Name()] = combo.ID()
	return nil
}

func (r *stubComboRepo) List(_ context.Context, q repository.PageQuery) ([]domain.Combo, int64, error) {
	all := make([]domain.Combo, 0, len(r.byID))
	for _, combo := range r.byID {
		all = append(all, combo)
	}
	start := q.Offset()
	if start >= len(all) {
		return []domain.Combo{}, int64(len(all)), nil
	}
	end := start + q.PerPage
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], int64(len(all)), nil
}

func (r *stubComboRepo) GetByID(_ context.Context, id string) (domain.Combo, error) {
	combo, ok := r.byID[id]
	if !ok {
		return domain.Combo{}, domain.ErrComboNotFound
	}
	return combo, nil
}

func (r *stubComboRepo) GetByName(_ context.Context, name string) (domain.Combo, error) {
	id, ok := r.nameID[name]
	if !ok {
		return domain.Combo{}, domain.ErrComboNotFound
	}
	return r.byID[id], nil
}

func (r *stubComboRepo) ExistsByName(_ context.Context, name string) (bool, error) {
	_, ok := r.nameID[name]
	return ok, nil
}

func (r *stubComboRepo) Names(context.Context) ([]string, error) {
	out := make([]string, 0, len(r.nameID))
	for name := range r.nameID {
		out = append(out, name)
	}
	return out, nil
}

func (r *stubComboRepo) Update(_ context.Context, combo domain.Combo) error {
	existing, ok := r.byID[combo.ID()]
	if !ok {
		return domain.ErrComboNotFound
	}
	if owner, taken := r.nameID[combo.Name()]; taken && owner != combo.ID() {
		return domain.ErrComboExists
	}
	delete(r.nameID, existing.Name())
	r.byID[combo.ID()] = combo
	r.nameID[combo.Name()] = combo.ID()
	return nil
}
