// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_stream_commit_test.go
// @for       The commitment a stream makes once its first frame has reached the client.
// @uses      atomic, context, net/http, net/http/httptest, strings, testing, internal/domain
// @reason    A mid-stream upstream failure is failover-worthy, and walking to the next combo member after the client already has bytes appends a second answer to the stream it is reading and bills a second upstream for one request. This pins that the walk stops where the client began.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// streamThenBreakUpstream answers one chunk the client can read, then sends a
// single event past the reader's ceiling, which ends the stream with an
// upstream-class failure: the class the combo walk would otherwise fail over from.
func streamThenBreakUpstream(t *testing.T, calls *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"id":"cc1","object":"chat.completion.chunk","choices":` +
			`[{"index":0,"delta":{"content":"first"}}]}` + "\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte(`data: {"filler":"` + strings.Repeat("y", maxEventBytes+64) +
			`"}` + "\n\n"))
	}))
}

func TestRelay_CommittedStreamDoesNotWalkToTheNextMember(t *testing.T) {
	var calls atomic.Int64
	server := streamThenBreakUpstream(t, &calls)
	defer server.Close()

	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-1", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-2", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"pair": comboRow("pair", "alpha/m", "beta/m"),
	})

	request := relayRequest("pair")
	request.Stream = true
	sink := &recordingSink{}

	outcome, err := engine.Relay(context.Background(), request, sink)
	if err == nil {
		t.Fatal("Relay() reported success although the upstream broke mid-stream")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1: a second member streamed behind the answer the client is already reading", got)
	}
	if !outcome.FramesWritten {
		t.Fatal("Outcome.FramesWritten = false, although the first frame reached the sink")
	}
	if len(sink.frames) == 0 {
		t.Fatal("the sink recorded no frame, so the committed path was never exercised")
	}
}
