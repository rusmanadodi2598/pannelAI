// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/seams.go
// @for       The interfaces the pipeline talks to at its boundaries: the stream
//
//	sink it writes frames to and the two optional body seams it hands the
//	translated upstream body to.
//
// @uses      internal/reasoning, context.
// @reason    A port belongs to the code that calls it, not to the package that
//
//	implements it: declaring these here is what keeps the engine free of
//	net/http, of the token-saver graph, and of the settings store, and
//	what lets a test stub one seam without building the pipeline behind
//	it. They live in one file so engine.go stays inside the AGENTS.md
//	§1.1 line budget and so "what the engine can be plugged into" is one
//	list rather than three definitions scattered through the pipeline.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/reasoning"
)

// FrameSink receives the frames of a streamed answer. It exists so the data plane
// never touches an http.ResponseWriter (AGENTS.md §1.5 keeps net/http out of the
// service layer) while still flushing each frame as it is produced.
type FrameSink interface {
	// WriteFrame writes one complete SSE frame. A partial write is an error the
	// caller stops the stream on.
	WriteFrame(frame []byte) error
	// Flush pushes what has been written to the client.
	Flush()
}

// TokenSaver is the request-path seam for optional body transforms. The engine
// hands it the already translated upstream wire, so a saver cannot be undone by
// a later format conversion.
type TokenSaver interface {
	Apply(ctx context.Context, body []byte, wire, model string, bypass bool) []byte
}

// ThinkingApplier is the §7.15 request-path seam that writes the resolved
// reasoning control into the already translated upstream body. It is declared
// here so the engine binds to the one method it calls and a test can stub it;
// the reasoning package's applier satisfies it directly, so no adapter is
// written for it.
type ThinkingApplier interface {
	Apply(ctx context.Context, body []byte, call reasoning.Call) []byte
}
