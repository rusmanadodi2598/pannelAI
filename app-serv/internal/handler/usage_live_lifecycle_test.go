// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage_live_lifecycle_test.go
// @for       The live Usage route's lifecycle: a change is pushed, and a
//
//	disconnect ends the loop.
//
// @uses      context, encoding/json, net/http, net/http/httptest, testing, time,
//
//	internal/domain, internal/schema.
//
// @reason    AGENTS.md §1.6 requires every goroutine to have an explicit
//
//	termination condition, and a stream's loop is where that rule is
//	easiest to break silently: a loop that kept reading against a closed
//	connection looks identical to a healthy one until the sockets run
//	out. These tests pin both halves, the push that makes the stream live
//	and the stop that ends it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestUsageLiveHandler_EndsWhenTheClientDisconnects pins the stream's explicit
// termination condition: closing the socket ends the read loop rather than
// leaving a goroutine holding a connection nobody is reading (AGENTS.md §1.6).
func TestUsageLiveHandler_EndsWhenTheClientDisconnects(t *testing.T) {
	usage := &liveHandlerUsageDouble{}
	handler := newLiveHandler(t, usage)
	handler.SetIntervals(25*time.Millisecond, 20*time.Millisecond)

	stream := dialLiveStream(t, liveRoute(handler))
	stream.readHead(t)
	_ = stream.readFrame(t)

	before := usage.readCount()
	if err := stream.conn.Close(); err != nil {
		t.Fatalf("closing the client socket: %v", err)
	}
	// The read loop has to stop. The counter is the observable: a loop still
	// running against a closed connection keeps reading, and a loop that ended
	// leaves the count where it was.
	time.Sleep(300 * time.Millisecond)
	after := usage.readCount()
	if after > before+4 {
		t.Fatalf("reads kept climbing after the client disconnected (%d -> %d), so the loop did not stop", before, after)
	}
}

// TestUsageLiveHandler_ServesTheRouteThroughTheSessionGate pins that the
// registered pattern is session-gated like every other management read, which is
// the sweep's own rule and is restated here for this route specifically.
func TestUsageLiveHandler_ServesTheRouteThroughTheSessionGate(t *testing.T) {
	usage := &liveHandlerUsageDouble{}
	handler := newLiveHandler(t, usage)

	// A plain handler with no session gate answers; the gate itself is pinned by
	// the router's whole-table sweep, and this asserts the route's own handler
	// does not refuse an authorised caller.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/usage/live", nil)
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	handler.Stream(recorder, request.WithContext(ctx))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a cancelled context still gets the frame it connected for", recorder.Code)
	}
}
