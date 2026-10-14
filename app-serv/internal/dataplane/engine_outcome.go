// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_outcome.go
// @for       The health decision a finished leg makes: record the credential as served, and record nothing when the caller stopped listening.
// @uses      internal/domain, internal/logx, context, errors.
// @reason    The walk's accounting was decided inside the relay loop, and one of those decisions was wrong: a stream the client abandoned stops quietly (pump returns nil), which the loop read as a served call and cleared a park a real upstream failure had just earned. Both rules now live together where the difference between "answered" and "still being read" can be stated once. It is also what keeps engine_relay.go inside the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"errors"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
)

// recordServedOutcome applies an answered call to the credential's health. A stream
// the client stopped reading records neither a success nor a failure: the frames that
// arrived proved the credential can answer, the cancellation proved nothing, and a park
// another request earned has to stay where it is.
//
// A bookkeeping failure is logged and dropped, as the failure path drops it: a 200 the
// caller can read outranks a health row that did not land.
func (e *Engine) recordServedOutcome(ctx context.Context, outcome *Outcome, selection Selection) {
	if outcome.StreamAborted {
		return
	}
	if err := e.selector.RecordSuccess(ctx, selection); err != nil {
		logx.Degraded(e.logger, "key health could not be recorded after a served call", err,
			"endpoint_id", outcome.EndpointID, "key_id", selection.Key.ID())
	}
}

// isClientCancellation reports whether a read failed because the caller left rather than
// because the upstream refused. Recording a hang-up as an upstream error would let one
// client's browsing habits park an account for every other caller.
func isClientCancellation(err error) bool {
	return errors.Is(err, context.Canceled)
}
