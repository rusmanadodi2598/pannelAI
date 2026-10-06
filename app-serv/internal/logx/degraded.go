// Package logx writes the structured lines the services degrade on purpose.
//
// @file      internal/logx/degraded.go
// @for       One place that records a deliberate fail-open with the context needed to trace it.
// @uses      log/slog
// @reason    AGENTS.md §1.6 requires a structured log on every error path. Several components choose to swallow a bookkeeping failure to keep serving the request, which is correct behaviour and untraceable unless the decision is written down with the object it concerned. Dataplane, token-saver and reasoning all needed the same rule, so it lives here rather than in three copies that can drift.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-04
package logx

import "log/slog"

// Of answers the logger a component writes to, defaulting to the process logger
// so a wiring that passes none still records its degradations.
func Of(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

// Degraded records one fail-open. A nil error writes nothing, because the
// majority of these call sites run on the happy path and a log line per served
// request is noise, not evidence. The cause is carried as text under `reason`:
// the caller has already decided not to fail, so the value's only job is to
// explain the decision to whoever reads the line.
func Degraded(logger *slog.Logger, event string, err error, args ...any) {
	if err == nil {
		return
	}
	Of(logger).Warn(event, append(args, "reason", err.Error())...)
}
