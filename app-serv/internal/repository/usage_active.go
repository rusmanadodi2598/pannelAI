// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/usage_active.go
// @for       The in-flight marker store: record one call while it runs, remove
//
//	it when it ends, and read the bounded live set.
//
// @uses      internal/domain, context.
// @reason    SPEC-UI-001 §6.5 makes the drawing's live state come from the
//
//	gateway rather than from a poll, and the gateway is the only party
//	that knows a request is between two instants. The port is three
//	methods over one key because that is the whole lifecycle: a marker
//	is written before the outbound call, removed after it, and read by
//	the stream. Keeping it here rather than in the service keeps Redis
//	out of `service` (AGENTS.md §1.5).
//
//	The read is deliberately bounded and takes its own clock, so the
//	staleness rule belongs to one place and a caller cannot ask for an
//	unbounded scan of a shared keyspace (§1.7).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package repository

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ActiveRequestStore is the in-flight marker store the live Usage stream reads.
//
// Every method is a no-op on a nil store, which is the documented behaviour for
// a deployment that wired no broker: the data plane keeps serving and the
// drawing shows no active provider rather than the process refusing to boot.
type ActiveRequestStore interface {
	// Start records one marker for a call that is about to run.
	Start(ctx context.Context, marker domain.ActiveRequest) error

	// Finish removes one marker. It takes the marker rather than its id because
	// the store keys entries by the marker's own encoded form: removal is then
	// one exact deletion with no lookup, and a marker that is already gone is
	// not an error, because the staleness window or a restart may have removed
	// it and the call it described has ended either way.
	Finish(ctx context.Context, marker domain.ActiveRequest) error

	// Active returns at most limit markers no older than domain.ActiveRequestStaleAfter,
	// oldest first, and prunes what it found stale so the keyspace cannot grow
	// without a reader.
	Active(ctx context.Context, now time.Time, limit int) ([]domain.ActiveRequest, error)
}
