// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage_live.go
// @for       The live Usage stream: one SSE connection carrying the gateway's
//
//	in-flight set, its recent requests, and its last error provider.
//
// @uses      internal/schema, internal/service, context, encoding/json,
//
//	log/slog, net/http, runtime/debug, time.
//
// @reason    SPEC-API-001 §7.12 and SPEC-UI-001 §6.5 give /usage one stream, and
//
//	§4 fixes its framing: text/event-stream, `X-Accel-Buffering: no`, and
//	the status line committed with the first frame so a failure before
//	that point is still an ordinary HTTP error. The sink is the one the
//	data plane already streams through, so this route inherits the
//	Flusher forwarding draft 010 F5 fixed rather than re-deriving it.
//
//	The loop is deliberately a read-and-compare rather than a poll: the
//	first frame is the whole state, and every later frame is sent only
//	when the state differs from the last one sent, so a quiet gateway
//	costs one bounded read per interval and writes nothing. A keepalive
//	comment is what holds an idle connection open through a proxy
//	without claiming a change happened.
//
//	TERMINATION (AGENTS.md §1.6)
//
//	The loop ends on exactly four conditions, all explicit: the client
//	disconnected (ctx done), the stream reached its stated maximum
//	lifetime, a write failed, or the service refused a read. A panic
//	inside the loop is recovered and ends that one connection rather
//	than the process.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// The stream's own cadences, as defaults a test may shorten. The read interval is
// short enough that a change reaches the screen promptly and long enough that a
// quiet gateway costs one bounded read every second; the keepalive is under the
// 25 seconds draft 010 §10.3 names, which is the interval an idle proxy needs to
// see to keep the connection open.
const (
	usageLiveReadInterval      = time.Second
	usageLiveKeepaliveInterval = 20 * time.Second
	// usageLiveMaxLifetime is the stream's explicit end. A connection that lives
	// forever is a connection nobody re-authenticates, so the stream ends on its
	// own terms and the panel's bounded retry schedule reconnects.
	usageLiveMaxLifetime = 30 * time.Minute
)

// usageLiveKeepalive is the comment an idle stream sends. It is an SSE comment
// rather than a frame, so a client counting frames is not told a change happened.
var usageLiveKeepalive = []byte(": ping\n\n")

// UsageLiveHandler serves GET /api/v1/usage/live (§7.12).
type UsageLiveHandler struct {
	live           *service.UsageLiveService
	readInterval   time.Duration
	keepaliveEvery time.Duration
	clock          func() time.Time
}

// NewUsageLiveHandler returns the route's handler.
func NewUsageLiveHandler(live *service.UsageLiveService) *UsageLiveHandler {
	return &UsageLiveHandler{
		live:           live,
		readInterval:   usageLiveReadInterval,
		keepaliveEvery: usageLiveKeepaliveInterval,
		clock:          time.Now,
	}
}

// SetIntervals shortens the stream's cadences so a test observes a keepalive and
// a change without waiting for the production intervals. It is not part of the
// production path.
func (h *UsageLiveHandler) SetIntervals(read, keepalive time.Duration) {
	if h == nil {
		return
	}
	if read > 0 {
		h.readInterval = read
	}
	if keepalive > 0 {
		h.keepaliveEvery = keepalive
	}
}

// Stream serves the route.
//
// The first frame is sent before the loop and before any timer exists, so the
// response is committed with real state rather than with an empty promise, and a
// client that connected between changes still draws the right picture.
func (h *UsageLiveHandler) Stream(w http.ResponseWriter, r *http.Request) {
	sink := newSSESink(w)

	frame, err := h.live.Snapshot(r.Context())
	if err != nil {
		// Nothing has been written, so this is still an ordinary HTTP error the
		// client can act on rather than a success status carrying a failure.
		slog.Warn("reading the first live frame failed", "error", err)
		schema.WriteError(w, err)
		return
	}
	if !h.writeFrame(sink, frame) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), usageLiveMaxLifetime)
	defer cancel()

	last := encodeFrame(frame)
	readTicker := time.NewTicker(h.readInterval)
	defer readTicker.Stop()
	keepaliveTicker := time.NewTicker(h.keepaliveEvery)
	defer keepaliveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			// The client disconnected, or the stream reached its own end. Both
			// are terminations rather than failures.
			return
		case <-readTicker.C:
			if !h.publish(ctx, sink, &last) {
				return
			}
		case <-keepaliveTicker.C:
			if !h.writeBytes(sink, usageLiveKeepalive) {
				return
			}
		}
	}
}

// publish reads the current state and writes a frame only when it differs from
// the last one sent, reporting whether the stream may continue. It takes the
// last frame by pointer because it is the one thing the loop carries across
// iterations.
//
// Comparing the encoded frames rather than each field is deliberate: the frame is
// the unit the client receives, so "did the client's picture change" is exactly
// "do these bytes differ", and a field-by-field comparison would be a second
// answer to the same question that could disagree with the wire.
func (h *UsageLiveHandler) publish(ctx context.Context, sink *sseSink, last *[]byte) bool {
	frame, err := h.live.Snapshot(ctx)
	if err != nil {
		// A read that failed is stated rather than answered with an empty frame:
		// "nothing is running" is a claim a failed read cannot support (R-36).
		slog.Warn("reading a live frame failed; ending the stream", "error", err)
		return false
	}
	encoded := encodeFrame(frame)
	if bytes.Equal(encoded, *last) {
		return true
	}
	if !h.writeFrame(sink, frame) {
		return false
	}
	*last = encoded
	return true
}

// writeFrame encodes and writes one frame under the panic boundary.
func (h *UsageLiveHandler) writeFrame(sink *sseSink, frame schema.UsageLiveFrame) bool {
	return h.writeBytes(sink, dataplane.Frame(encodeFrame(frame)))
}

// writeBytes writes and flushes one complete SSE frame, recovering a panic in
// the encoding path so one bad frame ends one connection rather than the process.
func (h *UsageLiveHandler) writeBytes(sink *sseSink, payload []byte) bool {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("panic while writing a live frame",
				"panic", recovered, "stack", string(debug.Stack()))
		}
	}()
	if err := sink.WriteFrame(payload); err != nil {
		// The client is gone; a failed write is how a disconnect usually
		// surfaces, so it ends the stream quietly.
		return false
	}
	sink.Flush()
	return true
}

// encodeFrame renders the frame's JSON. An encode failure cannot happen for a
// frame built from strings and integers, and the caller treats an empty result as
// an unchanged frame rather than sending a broken one.
func encodeFrame(frame schema.UsageLiveFrame) []byte {
	payload, err := json.Marshal(frame)
	if err != nil {
		slog.Error("encoding a live frame failed", "error", err)
		return nil
	}
	return payload
}
