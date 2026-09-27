// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/reasoning_delta.go
// @for       Reading one chat delta's reasoning text across the vendor shapes
//
//	the reference's extractReasoningText answers.
//
// @uses      encoding/json.
// @reason    A forced stream is folded into one completion, and vendors report
//
//	reasoning under three different delta fields: reasoning_content
//	(GLM, Qwen, DeepSeek, Kimi), reasoning (compat layers), and
//	reasoning_details[] (MiniMax reasoning_split, OpenRouter). Porting
//	the reference's one extraction rule
//	(open-sse/translator/concerns/reasoning.js) keeps the fold and the
//	reference from disagreeing about the same delta, and deciding per
//	delta stops a frame that carries two shapes from being counted twice.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package dataplane

import "encoding/json"

// reasoningText extracts a delta's reasoning text with the reference's
// precedence: reasoning_content, then reasoning, then the joined
// reasoning_details entries. A field the delta does not carry contributes
// nothing, so the shapes can mix across one stream.
func reasoningText(delta object) string {
	if text := stringField(delta, "reasoning_content"); text != "" {
		return text
	}
	if text := stringField(delta, "reasoning"); text != "" {
		return text
	}
	details, ok := arrayField(delta, "reasoning_details")
	if !ok {
		return ""
	}
	joined := ""
	for _, raw := range details {
		joined += reasoningDetailText(raw)
	}
	return joined
}

// reasoningDetailText reads one reasoning_details entry, which is either a bare
// string or an object whose text the reference reads from `text` or `content`.
// An entry with neither contributes nothing rather than an error: the reference
// joins whatever text is present and skips the rest.
func reasoningDetailText(raw json.RawMessage) string {
	var bare string
	if err := json.Unmarshal(raw, &bare); err == nil {
		return bare
	}
	entry, ok := decodeObject(raw)
	if !ok {
		return ""
	}
	if text := stringField(entry, "text"); text != "" {
		return text
	}
	return stringField(entry, "content")
}
