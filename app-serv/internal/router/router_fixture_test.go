// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_fixture_test.go
// @for       The in-memory repositories and fixture builder for the management route tests.
// @uses      internal/domain, internal/handler, internal/registry,
// @reason    The route cases and the fixture stubs are separate declarations; AGENTS.md §1.1 caps a file at 250 lines, so the stubs moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-17
package router

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

func (r *memCatalogRepo) AddCustom(_ context.Context, model domain.CustomModel) error {
	for _, existing := range r.custom {
		if existing.Ref().String() == model.Ref().String() {
			return domain.ErrModelExists
		}
	}
	r.custom[model.ID()] = model
	return nil
}

func (r *memCatalogRepo) RemoveCustom(_ context.Context, id string) error {
	if _, ok := r.custom[id]; !ok {
		return domain.ErrModelNotFound
	}
	delete(r.custom, id)
	return nil
}

func (r *memCatalogRepo) Aliases(context.Context) ([]domain.ModelAlias, error) {
	out := make([]domain.ModelAlias, len(r.aliases))
	copy(out, r.aliases)
	return out, nil
}

func (r *memCatalogRepo) ReplaceAliases(_ context.Context, aliases []domain.ModelAlias) error {
	r.aliases = make([]domain.ModelAlias, len(aliases))
	copy(r.aliases, aliases)
	return nil
}

func (r *memCatalogRepo) Disabled(context.Context) ([]domain.ModelRef, error) {
	out := make([]domain.ModelRef, len(r.disabled))
	copy(out, r.disabled)
	return out, nil
}

func (r *memCatalogRepo) ReplaceDisabled(_ context.Context, refs []domain.ModelRef) error {
	r.disabled = make([]domain.ModelRef, len(refs))
	copy(r.disabled, refs)
	return nil
}

type memComboRepo struct {
	byID   map[string]domain.Combo
	nameID map[string]string
}

func newMemComboRepo() *memComboRepo {
	return &memComboRepo{byID: map[string]domain.Combo{}, nameID: map[string]string{}}
}

func (r *memComboRepo) Create(_ context.Context, combo domain.Combo) error {
	if _, dup := r.nameID[combo.Name()]; dup {
		return domain.ErrComboExists
	}
	r.byID[combo.ID()] = combo
	r.nameID[combo.Name()] = combo.ID()
	return nil
}

func (r *memComboRepo) List(_ context.Context, q repository.PageQuery) ([]domain.Combo, int64, error) {
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

func (r *memComboRepo) GetByID(_ context.Context, id string) (domain.Combo, error) {
	combo, ok := r.byID[id]
	if !ok {
		return domain.Combo{}, domain.ErrComboNotFound
	}
	return combo, nil
}

func (r *memComboRepo) GetByName(_ context.Context, name string) (domain.Combo, error) {
	id, ok := r.nameID[name]
	if !ok {
		return domain.Combo{}, domain.ErrComboNotFound
	}
	return r.byID[id], nil
}

func (r *memComboRepo) ExistsByName(_ context.Context, name string) (bool, error) {
	_, ok := r.nameID[name]
	return ok, nil
}

func (r *memComboRepo) Names(context.Context) ([]string, error) {
	out := make([]string, 0, len(r.nameID))
	for name := range r.nameID {
		out = append(out, name)
	}
	return out, nil
}

func (r *memComboRepo) Update(_ context.Context, combo domain.Combo) error {
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

func (r *memComboRepo) Delete(_ context.Context, id string) error {
	combo, ok := r.byID[id]
	if !ok {
		return domain.ErrComboNotFound
	}
	delete(r.nameID, combo.Name())
	delete(r.byID, id)
	return nil
}

type memAdapterRepo struct {
	adapter domain.VisionAdapter
}

func newMemAdapterRepo() *memAdapterRepo {
	return &memAdapterRepo{adapter: domain.DefaultVisionAdapter()}
}

func (r *memAdapterRepo) Get(context.Context) (domain.VisionAdapter, error) { return r.adapter, nil }

func (r *memAdapterRepo) Save(_ context.Context, adapter domain.VisionAdapter) error {
	r.adapter = adapter
	return nil
}

var (
	_ repository.ModelCatalogRepository  = (*memCatalogRepo)(nil)
	_ repository.ComboRepository         = (*memComboRepo)(nil)
	_ repository.VisionAdapterRepository = (*memAdapterRepo)(nil)
)
