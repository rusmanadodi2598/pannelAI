// Command app-serv wires the media and embeddings planes.
//
// @file      cmd/app-serv/dataplane_media_wiring.go
// @for       Builds the media and embeddings services over the engine they route
//
//	through.
//
// @uses      internal/dataplane, internal/router, internal/service.
// @reason    The media and embeddings planes ask the engine only for resolution
//
//	and selection, so they take narrow ports rather than the engine itself
//	(register G20: a refusal has to be recordable without it). Building
//	them here keeps the shared instance rules visible, one caller and one
//	quota counter for every plane, and keeps dataplane_wiring.go inside
//	its AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// buildMediaPlanes assembles the embeddings and media call services over one
// shared media caller and the engine's own resolver and selector.
func buildMediaPlanes(in dataPlaneInputs, engine *dataplane.Engine, quotas *service.QuotaCounter) (*service.EmbeddingsService, *service.MediaCallService, *dataplane.MediaTransport, error) {
	// The package's own HTTP implementation over the guarded client, which
	// already carries the §1.7 pool limits and the §1.6 deadlines. One instance
	// serves every media call, so the routes share a connection pool rather than
	// opening one each.
	caller := dataplane.NewMediaTransport(in.Client)

	embeddings, err := service.NewEmbeddingsService(service.EmbeddingsServiceDeps{
		// The embeddings use case asks the engine only for resolution and
		// selection, so it takes the same narrow ports the media service does
		// (register G20: a refusal has to be recordable without the engine).
		Resolver:       engine.Resolver(),
		Router:         mediaRouter{engine: engine},
		Caller:         caller,
		Overrides:      in.MediaOverrides,
		Usage:          in.Usage,
		Logs:           in.Logs,
		Quotas:         quotas,
		RequestID:      router.RequestIDFrom,
		ActiveRequests: in.ActiveRequests,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	media, err := service.NewMediaCallService(service.MediaCallServiceDeps{
		Index:          in.MediaIndex,
		Router:         mediaRouter{engine: engine},
		Caller:         caller,
		Overrides:      in.MediaOverrides,
		Usage:          in.Usage,
		Logs:           in.Logs,
		Quotas:         quotas,
		RequestID:      router.RequestIDFrom,
		ActiveRequests: in.ActiveRequests,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return embeddings, media, caller, nil
}
