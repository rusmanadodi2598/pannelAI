// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_answer.go
// @for       Reading a Responses API answer: its output items, finish status, and accounting.
// @uses      encoding/json, strings, internal/schema.
// @reason    Both client directions read one decoded form, so the item readers and the usage fold are one concern. Keeping them beside the answer translator (rather than in it) holds both files inside the AGENTS.md §1.1 budget. The finish reason lives here too because the stream and the body have to agree about one answer, including whether a tool call was closed before the ceiling stopped it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// ItemReasoning is the output item that carries a model's reasoning summary.
const ItemReasoning = "reasoning"

// responsesTextParts reads the output_text parts of one message item.
func responsesTextParts(item object) []string {
	parts, _ := arrayField(item, "content")
	out := make([]string, 0, len(parts))
	for _, raw := range parts {
		part, ok := decodeObject(raw)
		if !ok {
			continue
		}
		if stringField(part, "type") != ItemOutputText {
			continue
		}
		if text := stringField(part, "text"); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// responsesReasoningParts reads a reasoning item's summary, falling back to its
// content parts when the upstream reported the text there.
func responsesReasoningParts(item object) []string {
	if texts := textFields(item, "summary"); len(texts) > 0 {
		return texts
	}
	return textFields(item, "content")
}

// textFields reads the text member of every entry of an array member.
func textFields(parent object, key string) []string {
	entries, _ := arrayField(parent, key)
	out := make([]string, 0, len(entries))
	for _, raw := range entries {
		entry, ok := decodeObject(raw)
		if !ok {
			continue
		}
		if text := stringField(entry, "text"); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// responsesCall maps one function_call item onto the OpenAI tool call shape. A
// call with no name is dropped: the client could not dispatch it, and reporting
// a nameless call would look like a call it can make.
func responsesCall(item object) (schema.ToolCall, bool) {
	name := stringField(item, "name")
	if name == "" {
		return schema.ToolCall{}, false
	}
	arguments := stringField(item, "arguments")
	if arguments == "" {
		arguments = "{}"
	}
	return schema.ToolCall{
		ID:       stringField(item, "call_id"),
		Type:     schema.BlockFunction,
		Function: schema.FunctionCall{Name: name, Arguments: arguments},
	}, true
}

// ResponsesCeilingReason is how a Responses answer names a stop on its output
// ceiling. One spelling, because the reader and the writer of a stream both use it.
const ResponsesCeilingReason = "max_output_tokens"

// responsesStoppedAtCeiling reports whether the answer says it ran out of output.
// One spelling for the reason, because the stream and the body-side reader ask the
// same question of the same field.
func responsesStoppedAtCeiling(response object) bool {
	if stringField(response, "status") != schema.ResponsesStatusIncomplete {
		return false
	}
	details, ok := objectField(response, "incomplete_details")
	return ok && stringField(details, "reason") == ResponsesCeilingReason
}

// responsesFinishReason maps the answer's status onto the OpenAI finish reason.
// A call the ceiling cut before its arguments closed is reported as a length stop,
// never as a dispatchable call: measured 2026-10-16 on the free tier, an answer
// that spent its ceiling on thinking carried a call name with no arguments and was
// reported as `tool_calls`, leaving a coding client with a call it could not run
// and no signal that anything was missing. A call the answer closed survives the
// ceiling stop, because dropping it would discard a request the client can make.
//
// The stream and the fold read the ceiling off the same function so they cannot
// disagree about one answer.
func responsesFinishReason(response object, hasCalls, openCall bool) string {
	ceiling := responsesStoppedAtCeiling(response)
	if ceiling && openCall {
		return FinishLength
	}
	if hasCalls {
		return FinishToolCalls
	}
	if ceiling {
		return FinishLength
	}
	return FinishStop
}

// responsesCallLeftOpen reports whether one call item was cut before its arguments
// were complete. A body carries no item lifecycle, so the arguments themselves are
// the evidence: absent, blank, or not a JSON object means the call never closed.
func responsesCallLeftOpen(item object) bool {
	raw, present := item["arguments"]
	if !present {
		return true
	}
	var arguments string
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return false
	}
	if strings.TrimSpace(arguments) == "" {
		return true
	}
	var parsed map[string]json.RawMessage
	return json.Unmarshal([]byte(arguments), &parsed) != nil
}

// responsesUsageFromObject reads the Responses accounting block. input_tokens
// already includes the cached tokens, so the cached split is reported as a
// detail rather than added to the prompt count (reference comment,
// open-sse/translator/response/openai-responses.js).
func responsesUsageFromObject(usage object) *schema.Usage {
	parsed := schema.Usage{
		PromptTokens:     intField(usage, "input_tokens"),
		CompletionTokens: intField(usage, "output_tokens"),
	}
	parsed.TotalTokens = parsed.PromptTokens + parsed.CompletionTokens

	if details, ok := objectField(usage, "input_tokens_details"); ok {
		if cached := intField(details, "cached_tokens"); cached > 0 {
			parsed.PromptTokensDetails = &schema.PromptTokensDetails{
				CachedTokens:         cached,
				CacheReadInputTokens: cached,
			}
		}
	}
	if details, ok := objectField(usage, "output_tokens_details"); ok {
		if reasoning := intField(details, "reasoning_tokens"); reasoning > 0 {
			parsed.CompletionTokensDetail = &schema.CompletionDetail{ReasoningTokens: reasoning}
		}
	}
	return &parsed
}
