// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_timeouts.go
// @for       The deadlines one outbound attempt runs under: which one applies,
//
//	and the registry overrides that replace them.
//
// @uses      time, internal/registry.
// @reason    SPEC-API-001 §4 fixes the deadlines and AGENTS.md §1.6 requires
//
//	every outbound call to carry one. They are read at two different
//	moments (the retry loop decides, the attempt applies), so they live
//	together here rather than at either call site, which is also what keeps
//	transport_call.go inside the AGENTS.md §1.1 split warning.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package dataplane

import (
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// attemptDeadline reports the deadline one attempt runs under, or zero for none.
// The client's own shape decides: a client that asked for a stream has no total
// cap, because a long answer is not a failure and the idle read is what detects a
// dead upstream, while a client that asked for one body stays bounded by the
// total timeout even when the provider only answers a stream the gateway folds
// (SPEC-API-001 §4; AGENTS.md §1.6 requires the bound).
func attemptDeadline(clientStream bool, entry registry.Provider) time.Duration {
	if clientStream {
		return 0
	}
	return totalTimeout(entry)
}

// totalTimeout reports the total deadline for a non-streamed call, honouring a
// registry override.
func totalTimeout(entry registry.Provider) time.Duration {
	if ms := entry.Transport.TimeoutMS; ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return TotalTimeout
}

// idleTimeout reports how long a streamed body may stay silent, honouring a
// registry override.
func idleTimeout(entry registry.Provider) time.Duration {
	if ms := entry.Transport.StallTimeoutMS; ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return IdleTimeout
}
