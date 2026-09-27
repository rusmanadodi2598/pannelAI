// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_openai_usage_test.go
// @for       Table-driven tests for the one-delivery usage rule on the OpenAI
//
//	passthrough stream.
//
// @uses      strings, testing.
// @reason    SPEC-API-001 §4 fixes the usage chunk stream_options.include_usage
//
//	asks for, and draft 034 F1 measured the passthrough forwarding an
//	upstream frame that already carries usage while Finish appended a
//	second copy of the same numbers. A client that sums every usage
//	object on the wire would double its bill, so the boundary cases
//	(usage on the finish frame, early usage, a restatement, null, empty,
//	and an unasking client) are pinned here against the client-visible
//	bytes with no network and no clock (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package dataplane

import (
	"strings"
	"testing"
)

// TestOpenAIStream_UsageIsDeliveredOnce pins the one-delivery rule: the numbers
// reach the client exactly once, either on a forwarded upstream frame or on the
// gateway's own chunk, and never on both. Accounting stays intact either way.
func TestOpenAIStream_UsageIsDeliveredOnce(t *testing.T) {
	const numbers = `"prompt_tokens":7,"completion_tokens":2,"total_tokens":9`
	content := `{"id":"up-1","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`
	contentUsage := `{"id":"up-1","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}],"usage":{` + numbers + `}}`
	finish := `{"id":"up-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`
	finishUsage := `{"id":"up-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{` + numbers + `}}`

	cases := []struct {
		name             string
		includeUsage     bool
		chunks           []string
		wantUsageObjects int
		wantSynthetic    bool
		wantRecorded     bool
	}{
		{
			// The register 034 §3 shape: the upstream's finish frame carries the
			// numbers, so the forwarded frame is the delivery and Finish adds
			// nothing.
			name:             "usage on the finish frame is the one delivery",
			includeUsage:     true,
			chunks:           []string{content, finishUsage},
			wantUsageObjects: 1, wantSynthetic: false, wantRecorded: true,
		},
		{
			// A restatement is upstream's own pair of frames; the gateway
			// forwards both verbatim and adds no third.
			name:             "a restated usage adds no gateway chunk",
			includeUsage:     true,
			chunks:           []string{contentUsage, finishUsage},
			wantUsageObjects: 2, wantSynthetic: false, wantRecorded: true,
		},
		{
			// An early usage frame already delivered the numbers; the later
			// finish frame must not provoke a second delivery.
			name:             "usage on an early frame is not repeated at Finish",
			includeUsage:     true,
			chunks:           []string{contentUsage, finish},
			wantUsageObjects: 1, wantSynthetic: false, wantRecorded: true,
		},
		{
			// A client that did not ask gets only what the upstream sent.
			name:             "an unasking client still sees only the upstream's frame",
			includeUsage:     false,
			chunks:           []string{content, finishUsage},
			wantUsageObjects: 1, wantSynthetic: false, wantRecorded: true,
		},
		{
			// No upstream reported anything, so no fabricated usage appears.
			name:             "an upstream that reported nothing gets no fabricated chunk",
			includeUsage:     true,
			chunks:           []string{content, finish},
			wantUsageObjects: 0, wantSynthetic: false, wantRecorded: false,
		},
		{
			// A null member is the upstream saying it has no numbers, not zeros.
			name:             "a null usage stays absent",
			includeUsage:     true,
			chunks:           []string{content, `{"id":"up-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}`},
			wantUsageObjects: 0, wantSynthetic: false, wantRecorded: false,
		},
		{
			// An empty object is not numbers on the wire either, so the chunk
			// rule stays with Finish, which publishes the zero block it read.
			name:             "an empty usage object is not a delivery",
			includeUsage:     true,
			chunks:           []string{content, `{"id":"up-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{}}`},
			wantUsageObjects: 1, wantSynthetic: true, wantRecorded: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "model-x", 1700000000, tc.includeUsage)
			var frames [][]byte
			for _, payload := range tc.chunks {
				frames = append(frames, state.Frames(TargetOpenAI, []byte(payload))...)
			}
			frames = append(frames, state.Finish()...)
			rendered := frameText(frames)

			assertSSEStream(t, rendered)
			if got := strings.Count(rendered, `"total_tokens"`); got != tc.wantUsageObjects {
				t.Fatalf("usage objects = %d, want %d: %s", got, tc.wantUsageObjects, rendered)
			}
			if got := strings.Contains(rendered, `"choices":[]`); got != tc.wantSynthetic {
				t.Fatalf("empty-choices usage chunk present = %v, want %v: %s", got, tc.wantSynthetic, rendered)
			}
			if got := state.Usage() != nil; got != tc.wantRecorded {
				t.Fatalf("state records usage = %v, want %v", got, tc.wantRecorded)
			}
		})
	}
}
