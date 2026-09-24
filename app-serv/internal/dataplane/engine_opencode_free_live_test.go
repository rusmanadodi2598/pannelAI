//go:build integration

// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_opencode_free_live_test.go
// @for       The live free-tier acceptance run: the same three models against
//
//	the real opencode.ai upstream, with no credential configured.
//
// @uses      context, os, testing, internal/schema.
// @reason    The hermetic tests in engine_opencode_free_test.go pin the lane's
//
//	shape against a stand-in built from the measured gate. This file is the
//	other half of the evidence R-35 asks for: the same pipeline against the
//	real upstream, so "the free tier answers" is a measurement rather than
//	an inference. It carries the `integration` build tag because it spends
//	the upstream's anonymous quota and needs the network, so the default
//	`go test ./...` stays hermetic.
//
//	  go test -tags=integration -run TestOpenCodeFreeLive ./internal/dataplane/
//
//	The endpoint it builds stores no key, which is the point: the lane is
//	served by the connector's own public bearer, exactly as the reference
//	serves it through its virtual no-auth connection.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestOpenCodeFreeLive_ThreeModelsAnswerFromTheRealUpstream is the live half of
// the owner's acceptance request: each free model completes a streamed request
// through the whole pipeline with no credential, and the answer carries text.
//
// It reaches the network, so it is tagged `integration`; the hermetic acceptance
// test is the one that runs by default.
func TestOpenCodeFreeLive_ThreeModelsAnswerFromTheRealUpstream(t *testing.T) {
	seen := []openCodeFreeCall{}
	engine := newOpenCodeFreeEngine(t, "https://opencode.ai", &seen)

	for _, model := range openCodeFreeModels {
		t.Run(model, func(t *testing.T) {
			raw := []byte(`{"model":"opencode/` + model + `","messages":[{"role":"user","content":"Reply with the single word: pong"}],"stream":true}`)
			decoded, err := schema.DecodeChatRequest(raw)
			if err != nil {
				t.Fatalf("decoding the client body: %v", err)
			}
			request := Request{
				Route: RouteChatCompletions, ClientFormat: schema.FormatOpenAI,
				Model: decoded.Model, Chat: &decoded, Raw: raw, Stream: decoded.Stream,
			}

			// The free tier is shared and answers 429 once it has seen enough
			// anonymous traffic from one address, which is a property of the
			// upstream's capacity rather than of this pipeline. A bounded retry
			// is what a real client does with that answer, and it keeps the
			// assertion intact: the test still requires an answer, and reports
			// how many attempts the lane needed. A skip would hide a genuine
			// regression behind the same message.
			var outcome Outcome
			sink := &recordingSink{}
			for attempt := 1; attempt <= openCodeFreeLiveAttempts; attempt++ {
				sink = &recordingSink{}
				outcome, err = engine.Relay(context.Background(), request, sink)
				if err == nil || !isOpenCodeFreeRateLimit(err) {
					break
				}
				if attempt < openCodeFreeLiveAttempts {
					time.Sleep(openCodeFreeLiveBackoff)
				}
			}
			if err != nil {
				t.Fatalf("Relay(%s) error = %v after %d attempts, want the live free tier to answer",
					model, err, openCodeFreeLiveAttempts)
			}
			if outcome.ProviderID != "opencode" || outcome.EndpointID != "ep_free" {
				t.Fatalf("routed to %s/%s, want opencode/ep_free", outcome.ProviderID, outcome.EndpointID)
			}
			if !outcome.Streamed {
				t.Fatal("the answer must arrive as a stream: the live tier refuses a non-streamed request")
			}
			if text := freeSinkText(sink); text == "" {
				t.Fatal("the stream carried no answer text")
			}
		})
	}
}

// openCodeFreeLiveAttempts and openCodeFreeLiveBackoff bound the retry a live
// run makes against the shared free tier. Three attempts over a minute is enough
// for a burst to clear, and a longer wait would make the run look hung rather
// than rate-limited.
const (
	openCodeFreeLiveAttempts = 3
	openCodeFreeLiveBackoff  = 20 * time.Second
)

// isOpenCodeFreeRateLimit reports whether a live failure is the shared lane
// refusing further anonymous traffic. The free tier answers 429 once it has seen
// enough requests from one address, which is a property of the upstream's
// capacity rather than of this pipeline.
func isOpenCodeFreeRateLimit(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "RATE_LIMITED") ||
		strings.Contains(message, "Rate limit exceeded") ||
		strings.Contains(message, "Too Many Requests")
}
