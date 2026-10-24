// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/stream_terminal_test.go
// @for       That each client format's state recognises its own upstream terminal event before Finish synthesizes one.
// @uses      testing.
// @reason    M12's truncation flag is only as good as this signal: a false positive logs every healthy stream, and a false negative hides a cut answer. The four formats close with different events, so each state is exercised on its own before Finish runs.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package dataplane

import "testing"

// TestStreamTerminalSeen_PerFormat pins the marker each state treats as the
// upstream's own finish, and that a content frame alone is not one.
func TestStreamTerminalSeen_PerFormat(t *testing.T) {
	t.Run("openai content then finish", func(t *testing.T) {
		state := NewStreamState("", "m", 0, false)
		state.Frames(TargetOpenAI, []byte(`{"choices":[{"index":0,"delta":{"content":"hi"}}]}`))
		if state.terminalSeen() {
			t.Fatal("a content chunk counted as a finish")
		}
		state.Frames(TargetOpenAI, []byte(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`))
		if !state.terminalSeen() {
			t.Fatal("the upstream finish frame was not recognised")
		}
	})

	t.Run("claude message_stop", func(t *testing.T) {
		state := NewClaudeStreamState("", "m")
		state.Frames(TargetClaude, []byte(`{"type":"message_delta","delta":{"content":"hi"}}`))
		if state.terminalSeen() {
			t.Fatal("a content delta counted as a finish")
		}
		state.Frames(TargetClaude, []byte(`{"type":"message_stop"}`))
		if !state.terminalSeen() {
			t.Fatal("message_stop was not recognised")
		}
	})

	t.Run("responses completed", func(t *testing.T) {
		state := NewResponsesStreamState("", "m", 0)
		state.Frames(TargetResponses, []byte(`{"type":"response.output_text.delta","delta":"hi"}`))
		if state.terminalSeen() {
			t.Fatal("a content delta counted as a finish")
		}
		state.Frames(TargetResponses, []byte(`{"type":"response.completed","response":{"status":"completed"}}`))
		if !state.terminalSeen() {
			t.Fatal("response.completed was not recognised")
		}
	})

	t.Run("gemini finish reason", func(t *testing.T) {
		state := NewStreamState("", "m", 0, false)
		state.Frames(TargetGemini, []byte(`{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`))
		if state.terminalSeen() {
			t.Fatal("a Gemini content chunk counted as a finish")
		}
		state.Frames(TargetGemini, []byte(`{"candidates":[{"finishReason":"STOP"}]}`))
		if !state.terminalSeen() {
			t.Fatal("the Gemini finish reason was not recognised")
		}
	})
}
