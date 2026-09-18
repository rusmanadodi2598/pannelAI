// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/stream.go
// @for       SSE framing and the idle guard that bounds a silent stream.
// @uses      bytes, io, net/http, sync, time.
// @reason    SPEC-API-001 §4 fixes SSE as the streaming mechanism, requires
//
//	X-Accel-Buffering: no, and gives a stream no total deadline but a
//	300s idle read. Framing is a pure function and the guard is a
//	reader decorator, so both are testable without a network, and a
//	stalled upstream cannot hold a client connection open forever.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"
)

// SSEDone is the terminal frame every OpenAI-compatible stream ends with.
const SSEDone = "data: [DONE]\n\n"

// Frame renders one SSE data frame. The blank line is the event terminator, so
// emitting it is what makes a client flush what it has.
func Frame(payload []byte) []byte {
	var buf bytes.Buffer
	buf.Grow(len(payload) + 8)
	buf.WriteString("data: ")
	buf.Write(payload)
	buf.WriteString("\n\n")
	return buf.Bytes()
}

// EventFrame renders one named SSE event, which is the framing Anthropic's
// messages stream uses (`event: message_delta`).
func EventFrame(event string, payload []byte) []byte {
	var buf bytes.Buffer
	buf.Grow(len(payload) + len(event) + 16)
	buf.WriteString("event: ")
	buf.WriteString(event)
	buf.WriteString("\ndata: ")
	buf.Write(payload)
	buf.WriteString("\n\n")
	return buf.Bytes()
}

// DataPlaneHeaders sets the headers every SSE response carries.
//
// X-Accel-Buffering: no is required, not cosmetic (SPEC-API-001 §4): an nginx
// front end buffers a proxied response by default, which turns a live stream into
// one delayed blob at the end. Heartbeat comments are deliberately absent because
// §4 disables them by default.
func DataPlaneHeaders(headers http.Header) {
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
}

// idleGuard fails a read that stays silent longer than the deadline, and cancels
// the outbound request when it does.
//
// It exists because a stream has no total deadline: a provider that opens a
// stream and then dies would otherwise hold the client's connection until the
// client gave up, leaving the gateway with a goroutine and a socket for a request
// nobody is waiting for. Every read of at least one byte resets the window, so a
// legitimately long answer is never cut off — only a genuine silence is.
type idleGuard struct {
	body    io.ReadCloser
	timeout time.Duration
	cancel  func()

	// mu guards fired and timer. The timer fires on its own goroutine while a
	// read may be in flight, so both fields need the lock: a bare flag would be
	// a data race the race detector catches under load, not in a quiet test.
	mu    sync.Mutex
	timer *time.Timer
	fired bool
}

// newIdleGuard wraps a streamed body. cancel is called when the window expires,
// which closes the connection the blocked read is on.
func newIdleGuard(body io.ReadCloser, timeout time.Duration, cancel func()) io.ReadCloser {
	if timeout <= 0 || cancel == nil {
		return body
	}
	guard := &idleGuard{body: body, timeout: timeout, cancel: cancel}
	guard.timer = time.AfterFunc(timeout, guard.expire)
	return guard
}

// expire cancels the outbound request. It runs on the timer's goroutine, so it
// takes the lock before touching state and calls cancel outside it.
func (g *idleGuard) expire() {
	g.mu.Lock()
	g.fired = true
	g.mu.Unlock()
	g.cancel()
}

// expired reports whether the window already elapsed.
func (g *idleGuard) expired() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.fired
}

// Read forwards one read and resets the idle window on progress.
func (g *idleGuard) Read(p []byte) (int, error) {
	if g.expired() {
		return 0, io.ErrUnexpectedEOF
	}
	n, err := g.body.Read(p)
	if n > 0 {
		g.mu.Lock()
		if !g.fired {
			g.timer.Reset(g.timeout)
		}
		g.mu.Unlock()
	}
	if g.expired() {
		// The window expired while this read was in flight, so the bytes are
		// discarded: they belong to a request already reported as timed out, and
		// mixing them into a re-framed stream would emit a half-frame.
		return 0, io.ErrUnexpectedEOF
	}
	return n, err
}

// Close releases the timer and the upstream body.
func (g *idleGuard) Close() error {
	g.mu.Lock()
	if g.timer != nil {
		g.timer.Stop()
	}
	g.mu.Unlock()
	g.cancel()
	return g.body.Close()
}
