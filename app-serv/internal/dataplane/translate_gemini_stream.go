// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_gemini_stream.go
// @for       One streamed Gemini payload mapped onto the OpenAI chunk vocabulary.
// @uses      internal/schema.
// @reason    The three client stream states already know how to frame an OpenAI chunk, so the Gemini stream needs one mapping into that vocabulary rather than a third framing path per client wire. It lives apart from the answer direction because a frame and a whole answer share only the part reading, and the frame has rules an answer does not: no finish frame at all on some vendors, and usage arriving on its own payload.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

func geminiChunkToOpenAIChunk(payload []byte) []byte {
	chunk, ok := decodeObject(payload)
	if !ok {
		return nil
	}
	parts, reason := geminiAnswerParts(chunk)

	delta := map[string]any{}
	texts := make([]string, 0, 2)
	reasoning := make([]string, 0, 2)
	calls := make([]map[string]any, 0, 2)
	for _, part := range parts {
		switch {
		case part.Thought:
			if part.Text != "" {
				reasoning = append(reasoning, part.Text)
			}
		case part.Text != "":
			texts = append(texts, part.Text)
		}
		if name, arguments, ok := part.functionCall(); ok {
			calls = append(calls, map[string]any{
				"index":    len(calls),
				"id":       geminiCallID(len(calls)),
				"type":     schema.BlockFunction,
				"function": map[string]any{"name": name, "arguments": string(arguments)},
			})
		}
	}
	if len(texts) > 0 {
		delta["content"] = concatGeminiText(texts)
	}
	if len(reasoning) > 0 {
		delta["reasoning_content"] = concatGeminiText(reasoning)
	}
	if len(calls) > 0 {
		delta["tool_calls"] = calls
	}

	out := map[string]any{}
	if metadata, ok := objectField(chunk, "usageMetadata"); ok && len(metadata) > 0 {
		out["usage"] = GeminiUsageToOpenAI(mustJSON(metadata))
	}
	finish := ""
	if reason != "" {
		finish = openAIFinishReason(reason, formatGemini)
	}
	// A frame carrying neither content, a finish, nor usage is the vendor's
	// bookkeeping (a model version header arrives the same way). Forwarding it as
	// an empty delta would give a client a frame that claims an answer.
	if len(delta) == 0 && finish == "" && len(out) == 0 {
		return nil
	}

	choice := map[string]any{"index": 0, "delta": delta, "finish_reason": nil}
	if finish != "" {
		choice["finish_reason"] = finish
	}
	out["object"] = "chat.completion.chunk"
	out["choices"] = []any{choice}
	return mustJSON(out)
}
