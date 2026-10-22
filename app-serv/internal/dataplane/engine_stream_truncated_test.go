// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_stream_truncated_test.go
// @for       The stream that ends on a clean EOF before the upstream's finish event.
// @uses      context, net/http, net/http/httptest, testing, internal/domain.
// @reason    A clean EOF is indistinguishable from a finished answer at the transport, and the gateway synthesizes the finish the client needs. The credential did not finish the answer, so the leg is logged and kept out of the served-success path without parking a provider that may simply close oddly.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package dataplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestRelay_StreamWithoutATerminalEvent pins the three shapes a clean close can
// have: no terminal event, an upstream finish frame, and the [DONE] marker.
func TestRelay_StreamWithoutATerminalEvent(t *testing.T) {
	content := `data: {"id":"cc1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hi"}}]}` + "\n\n"
	finish := `data: {"id":"cc1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n"

	cases := []struct {
		name string
		body string
		want bool
	}{
		{name: "no terminal event", body: content, want: true},
		{name: "an upstream finish frame", body: content + finish, want: false},
		{name: "the done marker", body: content + "data: [DONE]\n\n", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			repo := newMemEndpointRepo()
			repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-1", "alpha")}
			engine := newRelayEngine(t, server.URL, repo, nil)
			request := relayRequest("alpha/m")
			request.Stream = true
			sink := &recordingSink{}

			outcome, err := engine.Relay(context.Background(), request, sink)
			if err != nil {
				t.Fatalf("Relay() error = %v, want the clean close served", err)
			}
			if outcome.StreamTruncated != tc.want {
				t.Fatalf("StreamTruncated = %v, want %v", outcome.StreamTruncated, tc.want)
			}
			if !outcome.FramesWritten {
				t.Fatal("FramesWritten = false, want the content frame delivered")
			}
			if len(sink.frames) == 0 || string(sink.frames[len(sink.frames)-1]) != SSEDone {
				t.Fatalf("last frame = %q, want the synthesized terminal marker", sink.frames)
			}
		})
	}
}
