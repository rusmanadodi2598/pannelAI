// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_stream_usage_test.go
// @for       The accounting a streamed call reports on its outcome, so the usage
//
//	row records the numbers the upstream sent (draft 021 F5).
//
// @uses      context, testing.
// @reason    A streamed call logged 0/0 tokens while the wire carried the
//
//	upstream's usage, because the relay leg wrote the usage into the
//	outcome and the caller overwrote it with the nil the stream branch
//	returned. The free-tier stand-in reports usage on its finish frame, so
//	one relayed call is enough to pin the property end to end.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package dataplane

import (
	"context"
	"testing"
)

// TestOpenCodeFree_StreamedCallReportsItsUsage pins F5: a streamed call must hand
// the caller the usage the upstream reported, which is what the accounting pair
// and the quota counters read.
func TestOpenCodeFree_StreamedCallReportsItsUsage(t *testing.T) {
	seen := []openCodeFreeCall{}
	server := openCodeFreeUpstream(t, &seen)
	engine := newOpenCodeFreeEngine(t, server.URL, &seen)

	sink := &recordingSink{}
	outcome, err := engine.Relay(context.Background(), openCodeFreeRequest(t, openCodeFreeModels[0]), sink)
	if err != nil {
		t.Fatalf("Relay error = %v, want the free tier to answer", err)
	}
	if !outcome.Streamed {
		t.Fatal("the answer must arrive as a stream: the free tier refuses a non-streamed request")
	}
	if outcome.Usage == nil {
		t.Fatal("a streamed call must report the usage the upstream sent, not nil")
	}
	if outcome.Usage.PromptTokens != 7 || outcome.Usage.CompletionTokens != 2 {
		t.Fatalf("usage = %d/%d, want the stand-in's 7/2",
			outcome.Usage.PromptTokens, outcome.Usage.CompletionTokens)
	}
}
