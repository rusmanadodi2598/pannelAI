//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/usage_live_frames_integration_test.go
// @for       F4 live evidence: a real data-plane request lighting a node, the
//
//	marker released when it finishes, and an error naming its provider.
//
// @uses      internal/dataplane, internal/domain, internal/handler, internal/router,
//
//	internal/schema, internal/service, bufio, context, encoding/json, net,
//	net/http, net/http/httptest, os, strings, testing, time.
//
// @reason    Draft 013 F4 is closed by "the route answers with a session and the
//
//	live pass records a frame from the gateway itself". Every half of
//	that has a unit test, and none of them proves the two halves meet:
//	the marker is written by a real relay leg over a real socket, the
//	store is the real Redis, and the frame is read off a real
//	connection through the real router. That is the evidence the
//	previous pass could not produce, and it is the difference between
//	"the code should work" and "the frame arrived".
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration -run TestUsageLive ./cmd/app-serv/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestUsageLive_FrameCarriesARealRequestInFlight is the pass's central evidence:
// a real data-plane call, through the real relay leg, over a real upstream, with
// its marker read back off the real live route while the call is still running.
//
// The upstream holds its answer open on a channel, which is what makes the
// interval observable: the frame is read between the request starting and the
// upstream answering, so what it reports is a request genuinely in flight rather
// than a marker written for a call that already finished.
//
// The call runs on its own goroutine and its result travels over a channel
// rather than calling t.Fatalf: a failing assertion on a non-test goroutine does
// not stop the test, and the release below has to run or the upstream would hold
// the connection until the process exits.
func TestUsageLive_FrameCarriesARealRequestInFlight(t *testing.T) {
	release := make(chan struct{})
	upstream := newHoldingUpstream(t, release)
	fixture := newLiveUsageFixture(t, upstream)
	server := httptest.NewServer(liveRouteHandler(t, fixture, false))
	t.Cleanup(server.Close)

	type callResult struct {
		status int
		body   string
	}
	callDone := make(chan callResult, 1)
	go func() {
		status, body, _ := fixture.live.post(t,
			`{"model":"live/live-model","messages":[{"role":"user","content":"ping"}]}`, fixture.live.key, "")
		callDone <- callResult{status: status, body: body}
	}()

	frames := readLiveFramesUntil(t, server, func(frame schema.UsageLiveFrame) bool {
		return len(frame.Active) > 0
	})
	last := frames[len(frames)-1]
	t.Logf("live evidence: frames=%d active=%d provider=%q model=%q started_at=%q recent=%d error_provider=%q",
		len(frames), len(last.Active), last.Active[0].ProviderID, last.Active[0].Model,
		last.Active[0].StartedAt, len(last.Recent), last.ErrorProvider)

	if last.Active[0].ProviderID != "live" {
		t.Fatalf("active provider = %q, want live (the provider the request resolved to)", last.Active[0].ProviderID)
	}
	if last.Active[0].Model != "live-model" {
		t.Fatalf("active model = %q, want live-model", last.Active[0].Model)
	}
	if _, err := time.Parse(time.RFC3339, last.Active[0].StartedAt); err != nil {
		t.Fatalf("started_at = %q, want an RFC3339 instant: %v", last.Active[0].StartedAt, err)
	}

	// Releasing the upstream lets the call finish, which has to remove the
	// marker: a frame that keeps claiming the request is in flight would be the
	// stale-node defect this design exists to prevent.
	close(release)
	result := <-callDone
	t.Logf("live evidence: data-plane status=%d body=%s", result.status, truncate(result.body, 100))
	if result.status != http.StatusOK {
		t.Fatalf("data-plane status = %d, want 200; body = %s", result.status, result.body)
	}

	frames = readLiveFramesUntil(t, server, func(frame schema.UsageLiveFrame) bool {
		return len(frame.Active) == 0 && len(frame.Recent) > 0
	})
	final := frames[len(frames)-1]
	t.Logf("live evidence: after release active=%d recent=%d recent_request=%q status=%q tokens_in=%d tokens_out=%d",
		len(final.Active), len(final.Recent), final.Recent[0].RequestID, final.Recent[0].Status,
		final.Recent[0].TokensIn, final.Recent[0].TokensOut)

	if len(final.Active) != 0 {
		t.Fatalf("active = %+v after the call finished, want none: a finished call must release its marker", final.Active)
	}
	if final.Recent[0].Status != "success" {
		t.Fatalf("recent status = %q, want success", final.Recent[0].Status)
	}
	if final.Recent[0].TokensIn != 7 || final.Recent[0].TokensOut != 3 {
		t.Fatalf("recent tokens = (%d, %d), want the upstream's (7, 3)",
			final.Recent[0].TokensIn, final.Recent[0].TokensOut)
	}
}

// TestUsageLive_FrameNamesTheProviderOfAFailure pins the third fact: a request
// the upstream refuses is recorded as an error, and the frame names its provider
// so the drawing can mark that node.
func TestUsageLive_FrameNamesTheProviderOfAFailure(t *testing.T) {
	upstream := newFailingUpstream(t)
	fixture := newLiveUsageFixture(t, upstream)
	server := httptest.NewServer(liveRouteHandler(t, fixture, false))
	t.Cleanup(server.Close)

	// The failure is recorded synchronously by the request, so no goroutine is
	// needed: the frame below reads a state the call has already left.
	status, body, _ := fixture.live.post(t, `{"model":"live/live-model","messages":[{"role":"user","content":"ping"}]}`, fixture.live.key, "")
	t.Logf("live evidence: refused call status=%d body=%s", status, body)

	frames := readLiveFramesUntil(t, server, func(frame schema.UsageLiveFrame) bool {
		return frame.ErrorProvider != ""
	})
	last := frames[len(frames)-1]
	t.Logf("live evidence: error_provider=%q recent=%d recent_status=%q", last.ErrorProvider, len(last.Recent), last.Recent[0].Status)

	if last.ErrorProvider != "live" {
		t.Fatalf("error_provider = %q, want live", last.ErrorProvider)
	}
	if last.Recent[0].Status != "error" {
		t.Fatalf("recent status = %q, want error", last.Recent[0].Status)
	}
}

// TestUsageLive_IdleConnectionGetsACommentNotAFrame pins the idle rule on a real
// socket: nothing changes, so nothing is claimed, and the connection is held
// open with a comment.
func TestUsageLive_IdleConnectionGetsACommentNotAFrame(t *testing.T) {
	fixture := newLiveUsageFixture(t, newLiveUpstream(t))
	server := httptest.NewServer(liveRouteHandler(t, fixture, false))
	t.Cleanup(server.Close)

	conn, reader := dialLiveRoute(t, server)
	_ = conn
	first := readSSEPayload(t, reader)
	t.Logf("live evidence: first frame = %s", truncate(first, 120))

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading the idle line: %v", err)
	}
	t.Logf("live evidence: idle line = %q", line)
	if !strings.HasPrefix(line, ":") {
		t.Fatalf("idle line = %q, want an SSE comment rather than a frame", line)
	}
	if strings.HasPrefix(line, "data:") {
		t.Fatalf("idle line = %q, want a comment: an unchanged state must not be sent as a frame", line)
	}
}
