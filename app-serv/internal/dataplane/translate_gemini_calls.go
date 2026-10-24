// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_gemini_calls.go
// @for       The tool-call half of the OpenAI-to-Gemini direction: ids that stay unique across a whole history, and results paired with the call that asked for them.
// @uses      encoding/json, fmt, strings, internal/schema.
// @reason    A call and its result are one agreement written in two messages, and Gemini checks the agreement instead of ignoring it: it validates functionCall ids across the whole history and rejects the entire request with 400 INVALID_ARGUMENT when one repeats, while an OpenAI tool_call_id is only unique inside its own assistant turn. The reference (openai-to-gemini.js) carries the same rule as a walk with state, which is what a per-message lookup cannot be.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-18
package dataplane

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// geminiCall is one tool call as the upstream will read it: the id written on the
// wire, and the name this call's own declaration carried.
type geminiCall struct {
	id   string
	name string
}

// geminiHistory is the state one request's message walk keeps so a call and its
// result agree: Gemini rejects the whole request when a functionCall id repeats,
// while an OpenAI tool_call_id is only unique inside its own assistant turn.
//
// Each call takes the next free id and its result is written under that same id,
// named by the call it answers rather than by the last call that held the id.
type geminiHistory struct {
	declared  map[string][]geminiCall
	answered  map[string]int
	used      map[string]bool
	signature string
}

func newGeminiHistory(signature string) *geminiHistory {
	return &geminiHistory{
		declared:  make(map[string][]geminiCall, 4),
		answered:  make(map[string]int, 4),
		used:      make(map[string]bool, 8),
		signature: signature,
	}
}

// declare records one assistant tool call and returns the id and name its
// functionCall part goes out with.
func (h *geminiHistory) declare(originalID, name string) geminiCall {
	call := geminiCall{id: h.free(originalID), name: name}
	if originalID != "" {
		h.declared[originalID] = append(h.declared[originalID], call)
	}
	return call
}

// respond returns the next unanswered call the given id stands for. Results are
// consumed in order so a repeated id pairs each answer with its own call rather
// than handing both to the first.
func (h *geminiHistory) respond(originalID string) (geminiCall, bool) {
	calls, seen := h.declared[originalID], h.answered[originalID]
	if len(calls) == 0 || seen >= len(calls) {
		return geminiCall{}, false
	}
	h.answered[originalID] = seen + 1
	return calls[seen], true
}

// free returns an id no earlier call in this request took. Two calls sharing one
// id must end up with two different ids, so each takes the next free slot instead
// of a memoised answer.
func (h *geminiHistory) free(id string) string {
	if id == "" {
		return id
	}
	if !h.used[id] {
		h.used[id] = true
		return id
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", id, n)
		if !h.used[candidate] {
			h.used[candidate] = true
			return candidate
		}
	}
}

// geminiTurn converts one non-system message into a Gemini turn, returning false
// when the message produced nothing to send.
func geminiTurn(message schema.ChatMessage, history *geminiHistory) (GeminiContent, bool) {
	switch message.Role {
	case schema.RoleAssistant:
		return geminiModelTurn(message, history)
	case schema.RoleTool:
		part, ok := geminiFunctionResponse(message.ToolCallID, message.Content.TextContent(), history)
		if !ok {
			return GeminiContent{}, false
		}
		return GeminiContent{Role: RoleUser, Parts: []GeminiPart{part}}, true
	default:
		parts := geminiParts(message.Content)
		if len(parts) == 0 {
			return GeminiContent{}, false
		}
		return GeminiContent{Role: RoleUser, Parts: parts}, true
	}
}

// geminiModelTurn renders an assistant message: its answer text, then one
// functionCall part per tool call.
func geminiModelTurn(message schema.ChatMessage, history *geminiHistory) (GeminiContent, bool) {
	parts := make([]GeminiPart, 0, 2+len(message.ToolCalls))
	if text := message.Content.TextContent(); text != "" {
		parts = append(parts, GeminiPart{Text: text})
	}
	signed := false
	for _, call := range message.ToolCalls {
		if call.Function.Name == "" {
			continue
		}
		declared := history.declare(call.ID, sanitizeGeminiFunctionName(call.Function.Name))
		part := GeminiPart{FunctionCall: &GeminiFunctionCall{
			ID:   declared.id,
			Name: declared.name,
			Args: argumentMap(call.Function.Arguments),
		}}
		// One signature per model turn is what the backend reads as the thinking
		// that produced the calls; the siblings in the same turn stay unsigned.
		if !signed && history.signature != "" {
			part.ThoughtSignature = history.signature
			signed = true
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return GeminiContent{}, false
	}
	return GeminiContent{Role: RoleModel, Parts: parts}, true
}

// geminiFunctionResponse builds the functionResponse part for one tool result,
// naming and pairing it with the call the history says asked for it.
//
// The result travels under `result`, which is the reference's shape
// (openai-to-gemini.js wraps every parsed body: `response: { result: parsed }`).
// A body that already carries a `result` member is kept as it is instead of being
// nested a second time, and an unparseable or scalar body is carried verbatim so no
// result is silently lost.
func geminiFunctionResponse(callID, body string, history *geminiHistory) (GeminiPart, bool) {
	if callID == "" {
		return GeminiPart{}, false
	}
	call, ok := history.respond(callID)
	if !ok {
		// A result whose call is not in the history still has to travel: the name
		// is the id the client sent, because an id is opaque to this gateway and
		// reading a tool name out of its shape would invent one.
		call = geminiCall{id: history.free(callID), name: callID}
	}
	result := argumentMap(body)
	if _, ok := result["result"]; !ok {
		result = map[string]any{"result": argumentValue(body)}
	}
	return GeminiPart{FunctionResponse: &GeminiFunctionResp{
		ID:       call.id,
		Name:     sanitizeGeminiFunctionName(call.name),
		Response: result,
	}}, true
}

// argumentValue decodes a tool result body into the value a functionResponse
// carries: an object or array stays as it is, and a scalar or unparseable string is
// kept verbatim so no result is silently lost.
func argumentValue(body string) any {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return body
	}
	return parsed
}
