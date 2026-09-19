// Command app-serv wires the §7.10 media provider graph.
//
// @file      cmd/app-serv/media_wiring.go
// @for       Builds the media provider service behind its handler.
// @uses      internal/handler, internal/repository/postgres, internal/service,
//
//	pgxpool.
//
// @reason    AGENTS.md §1.5 makes the composition root wiring only, and §1.1
//
//	keeps management_wiring.go inside its line budget. The index handed in
//	is the runtime overlay, not the embedded registry: §7.10's detail route
//	accepts the same provider ids §7.4 does, so a custom node must resolve
//	here too.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// buildMediaProviders assembles the media provider service over the registry
// index, the stored overrides, and the endpoint counter the list reports. The
// service is returned alongside its handler because the data plane reads the
// same stored overrides through it: one reader, so the panel's save and the
// route's answer cannot disagree about which base URL is in force.
func buildMediaProviders(
	pool *pgxpool.Pool,
	index service.ProviderIndex,
	counts service.EndpointCounterByProvider,
) (*service.MediaProviderService, *handler.MediaProviderHandler, error) {
	svc, err := service.NewMediaProviderService(service.MediaProviderServiceDeps{
		Index: index, Repo: postgres.NewMediaOverrideRepository(pool), Counts: counts,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("media wiring: %w", err)
	}
	return svc, handler.NewMediaProviderHandler(svc), nil
}
