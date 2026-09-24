// Command app-serv wires the P1 management and data-plane dependencies.
//
// @file      cmd/app-serv/catalog_wiring.go
// @for       The model catalog service's slice of the composition root: one
//
//	constructor call over the repositories and seams the read needs.
//
// @uses      internal/repository, internal/registry, internal/service.
// @reason    The catalog is the first service in the management graph — the
//
//	combo and vision services resolve their model references through
//	it — and it reads the runtime overlay like the rest of the graph,
//	because a model an operator registers under a custom node must
//	reach the catalog or the node's models are routable but invisible.
//	The endpoint roll-up it takes is the seam `?active=true` answers
//	through, and it is the same one-query read the provider list
//	already uses (draft 025 F4). Holding this constructor here keeps
//	management_wiring.go a plain sequence under the AGENTS.md §1.1
//	line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package main

import (
	"fmt"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// buildCatalogService constructs the §7.6 catalog service over the runtime
// overlay and the model repositories, carrying the endpoint candidate reader the
// `active` filter is answered through.
func buildCatalogService(
	index *runtimeProviderIndex,
	catalogRepo repository.ModelCatalogRepository,
	comboRepo repository.ComboRepository,
	active service.ActiveProviderSet,
) (*service.ModelCatalogService, error) {
	catalog, err := service.NewModelCatalogService(service.ModelCatalogServiceDeps{
		Index: index, Repo: catalogRepo, Combos: comboRepo, Active: active,
	})
	if err != nil {
		return nil, fmt.Errorf("management wiring: model catalog: %w", err)
	}
	return catalog, nil
}
