// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_gemini_stream_test.go
// @for       One streamed Gemini payload mapped onto the OpenAI chunk vocabulary.
// @uses      strings, testing.
// @reason    A stream is where this format can answer successfully and still be wrong: the vendor sends no terminal marker, reports usage on its own frames, and sometimes sends a frame carrying neither content nor a finish. Each of those is a rule the mapping has to get right for a client to see one answer and one bill, so they are pinned here rather than left to the answer direction's tests.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package dataplane

import (
	"strings"
	"testing"
)

func TestGeminiChunkToOpenAIChunk_CarriesDeltaFinishAndUsage(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantIn  []string
		wantNil bool
	}{
		{
			name:    "text delta",
			payload: `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`,
			wantIn:  []string{`"content":"hi"`, "chat.completion.chunk"},
		},
		{
			name:    "thought delta",
			payload: `{"candidates":[{"content":{"parts":[{"text":"hmm","thought":true}]}}]}`,
			wantIn:  []string{`"reasoning_content":"hmm"`},
		},
		{
			name:    "function call delta",
			payload: `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"ls","args":{"p":"/tmp"}}}]}}]}`,
			wantIn:  []string{`"tool_calls"`, `"name":"ls"`, `"arguments":"{\"p\":\"/tmp\"}"`, `"index":0`},
		},
		{
			name:    "finish frame",
			payload: `{"candidates":[{"finishReason":"STOP"}]}`,
			wantIn:  []string{`"finish_reason":"stop"`},
		},
		{
			// The vendor reports usage on its own frames; forwarding it is what
			// keeps a streamed answer billed at all.
			name:    "usage frame",
			payload: `{"usageMetadata":{"promptTokenCount":9,"candidatesTokenCount":2,"totalTokenCount":11}}`,
			wantIn:  []string{`"usage"`, `"prompt_tokens":9`, `"completion_tokens":2`},
		},
		{
			name:    "a frame with nothing to show",
			payload: `{"candidates":[{"content":{"parts":[]}}]}`,
			wantNil: true,
		},
		{
			name:    "metadata frame the vendor sends beside the answer",
			payload: `{"modelVersion":"gemini-2.5-flash"}`,
			wantNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunk := geminiChunkToOpenAIChunk([]byte(tc.payload))
			if tc.wantNil {
				if chunk != nil {
					t.Fatalf("chunk = %s, want nil so no empty frame reaches the client", chunk)
				}
				return
			}
			if chunk == nil {
				t.Fatalf("chunk = nil, want the frame rendered")
			}
			for _, want := range tc.wantIn {
				if !strings.Contains(string(chunk), want) {
					t.Fatalf("chunk = %s, want it to carry %s", chunk, want)
				}
			}
		})
	}
}

// TestGeminiChunkToOpenAIChunk_FramesFeedTheOpenAIState proves the mapping lands
// in the vocabulary the existing states already frame, rather than a shape only
// its own test reads.
func TestGeminiChunkToOpenAIChunk_FramesFeedTheOpenAIState(t *testing.T) {
	state := NewStreamState("chatcmpl-x", "gemini-2.5-flash", 1700000000, false)
	chunk := geminiChunkToOpenAIChunk([]byte(`{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`))
	if chunk == nil {
		t.Fatal("no chunk produced")
	}

	frames := state.openAIFrames(chunk)
	if len(frames) != 1 {
		t.Fatalf("frames = %d, want the delta forwarded", len(frames))
	}
	body := string(frames[0])
	if !strings.Contains(body, `"content":"hi"`) || !strings.Contains(body, `"model":"gemini-2.5-flash"`) {
		t.Fatalf("frame = %s, want the delta text and the state's model name", body)
	}
}
