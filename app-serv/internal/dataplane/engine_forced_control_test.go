// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_forced_control_test.go
// @for       The control cases around a forced stream: a client that asked for
//
//	SSE still receives SSE, and a provider that forces nothing is
//	untouched.
//
// @uses      testing, context, internal/registry.
// @reason    The fold must only run for a client that asked for one body from a
//
//	provider that refuses to send one. A test that pins the opposite case
//	is what keeps a future change from folding a stream a client is
//	reading live, which would turn a live answer into one delayed blob.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-21
package dataplane

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestRelay_ForcedStreamKeepsAStreamingClientStreaming is the benign control: a
// client that asked for SSE still receives SSE, so the fold never runs for it.
func TestRelay_ForcedStreamKeepsAStreamingClientStreaming(t *testing.T) {
	var sawStream bool
	server := newStreamingUpstream(t, responsesStreamBody, &sawStream)
	defer server.Close()

	entry := registry.Provider{
		ID: "forced-responses", Priority: 1, Category: "free", PassthroughModels: true,
		Transport: registry.Transport{BaseURL: server.URL, Format: registry.FormatOpenAIResponses},
	}
	engine := newForcedStreamEngine(t, server.URL, entry, &forcedStreamConnector{
		Base: provider.Base{ID: entry.ID, Auth: "no_auth", Format: registry.FormatOpenAIResponses},
		url:  server.URL,
	})

	sink := &recordingSink{}
	in := relayRequest("forced-responses/muse")
	in.Stream = true

	outcome, err := engine.Relay(context.Background(), in, sink)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if !outcome.Streamed {
		t.Fatal("Outcome.Streamed = false, want the client's own stream")
	}
	if outcome.Body != nil {
		t.Fatalf("Outcome.Body = %s, want nil for a streamed answer", outcome.Body)
	}
	if len(sink.frames) == 0 {
		t.Fatal("the sink received no frames, want the upstream stream forwarded")
	}
}

// recordingSink collects the frames a streamed answer produced.
type recordingSink struct {
	frames [][]byte
}

func (s *recordingSink) WriteFrame(frame []byte) error {
	s.frames = append(s.frames, frame)
	return nil
}

func (s *recordingSink) Flush() {}
