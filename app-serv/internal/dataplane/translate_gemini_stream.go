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

// geminiStream is the half of one Gemini stream's translation the frames alone
// cannot know: how many calls the stream has already shown the client.
//
// A streamed OpenAI delta is accumulated by tool-call index, so a second call
// arriving in a later frame has to be reported under the next index rather than
// 0 again, and under an id the client can tell apart.
type geminiStream struct {
	calls int
	// sawCall marks a stream that already reported a call, because the vendor's
	// finish frame arrives separately and says STOP: a client that reads a plain
	// stop after a tool call never dispatches it.
	sawCall bool
}

// chunk maps one streamed Gemini payload onto the OpenAI chunk vocabulary, given
// the creation second the stream reports. The zero value is the state of a stream
// that has shown no call yet.
func (g *geminiStream) chunk(payload []byte, created int64) []byte {
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
		if call, ok := part.call(created, g.calls); ok {
			g.calls++
			calls = append(calls, map[string]any{
				"index":    g.calls - 1,
				"id":       call.id,
				"type":     schema.BlockFunction,
				"function": map[string]any{"name": call.name, "arguments": string(call.arguments)},
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
		g.sawCall = true
	}

	out := map[string]any{}
	if metadata, ok := geminiUsage(chunk); ok {
		out["usage"] = GeminiUsageToOpenAI(metadata)
	}
	finish := ""
	if reason != "" {
		finish = openAIFinishReason(reason, formatGemini)
		if g.sawCall && finish == FinishStop {
			finish = FinishToolCalls
		}
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
