// Command app-serv wires the P1 data-plane dependencies.
//
// @file      cmd/app-serv/media_router.go
// @for       Adapting the dataplane engine's selection seam to media services.
// @uses      internal/dataplane, context.
// @reason    Media calls need endpoint selection and circuit accounting but do not
//
// need the chat engine's request translation. Keeping this three-method
// adapter separate leaves the main composition file focused on construction
// and below the source-file line warning.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// mediaRouter adapts the engine to the three questions a media call asks. The
// adapter exists because the engine's selector is reached through a method:
// taking the engine itself would put the resolver and the wire translators in
// the way of every media call.
type mediaRouter struct{ engine *dataplane.Engine }

func (r mediaRouter) Select(ctx context.Context, providerID string) (dataplane.Selection, error) {
	return r.engine.Selector().Select(ctx, providerID)
}

func (r mediaRouter) RecordSuccess(ctx context.Context, selection dataplane.Selection) error {
	return r.engine.RecordSuccess(ctx, selection)
}

func (r mediaRouter) RecordFailure(ctx context.Context, selection dataplane.Selection, reason string) error {
	return r.engine.RecordFailure(ctx, selection, reason)
}
