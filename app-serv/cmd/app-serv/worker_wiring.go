// Command app-serv wires the background workers to the process lifecycle.
//
// @file      cmd/app-serv/worker_wiring.go
// @for       Starts the quota flush and the log retention workers, each with a
//
//	panic boundary and a termination condition.
//
// @uses      internal/service, context, log/slog, runtime/debug.
// @reason    AGENTS.md §1.6 requires every goroutine to recover from a panic and
//
//	to stop with the process, and SPEC-API-001 §6 gives both workers
//	their schedule. They live in one file because their shape is
//	identical — run until ctx is cancelled, supervise the panic — so a
//	third worker has a pattern to follow rather than a new place to
//	invent one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package main

import (
	"context"
	"log/slog"
	"runtime/debug"
)

// runWorkers starts every background worker the management graph returned. A nil
// worker is a wiring gap the constructor already reported, so it is skipped here
// rather than crashed on at shutdown.
func runWorkers(ctx context.Context, deps managementDeps) {
	if deps.QuotaFlusher != nil {
		go runSupervised("quota flusher", func() { deps.QuotaFlusher.Run(ctx) })
	}
	if deps.LogRetention != nil {
		go runSupervised("log retention", func() { deps.LogRetention.Run(ctx) })
	}
}

// runSupervised runs one worker body with the §1.6 panic boundary. The worker's
// own Run returns when ctx is cancelled, which is the explicit termination
// condition; this wrapper only keeps a panic in one worker from killing the
// process the way an unrecovered goroutine would.
func runSupervised(name string, body func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("background worker panic recovered",
				"worker", name, "panic", recovered, "stack", string(debug.Stack()))
		}
	}()
	body()
}
