// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/outcome.go
// @for       What one relayed call produced: the identity it was attempted
//
//	under, the answer, the accounting, and the latency.
//
// @uses      internal/schema.
// @reason    The outcome crosses the engine's boundary into accounting, logging,
//
//	and the handlers, so its shape is a contract rather than an internal
//	detail. Declaring it in its own file keeps engine.go inside the
//	AGENTS.md §1.1 line budget and gives that contract one home.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// Outcome reports what one relayed call produced, in the form accounting and
// logging need.
type Outcome struct {
	// Format is the wire format the caller's answer is written in.
	Format schema.DataPlaneFormat
	// ProviderID, EndpointID, and Model are the routing identity of the call.
	ProviderID string
	EndpointID string
	Model      string
	// Combo names the combo the request addressed, or "".
	Combo string
	// Body is the non-streamed answer, already in Format. It is nil for a
	// streamed answer, which the sink received frame by frame.
	Body []byte
	// Usage is the accounting the upstream reported, or nil when it reported
	// none, so a caller records nothing rather than a fabricated zero.
	Usage *schema.Usage
	// Streamed reports whether the answer went to a FrameSink.
	Streamed bool
	// FramesWritten reports that a streamed answer already reached the client.
	// The combo walk reads it as a commitment: failing over after the client has
	// bytes would stream a second answer behind the first, and bill a second
	// upstream for one request.
	FramesWritten bool
	// Truncated reports a non-streamed answer that stopped at its own output
	// ceiling with nothing in it for the client to show. It is not a failure:
	// the upstream answered 200 and the credential served the call. The combo
	// walk reads it to try the next member before serving an empty body.
	Truncated bool
	// VisionAdapted reports that the model which answered came from the §7.8
	// vision adapter rather than from anything the request addressed. It is the
	// one fact the gateway holds about an image answer it cannot otherwise
	// prove: whether the picture was actually read is the upstream's business,
	// but which model it was handed to is routing, and routing is known here.
	//
	// It exists because the answer's own model name is deliberately the one the
	// caller sent (SPEC-API-001 §7.6), so a substitution that changes who looked
	// at the image would otherwise leave no trace a client or an operator could
	// query, which is how a blind adapter model came to be serving red images as
	// "gray" for a whole combo without anyone being able to see it happen.
	VisionAdapted bool
	// LatencyMS is the upstream call's duration, measured with the engine's
	// clock.
	LatencyMS int64
}
