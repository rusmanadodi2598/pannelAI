// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_answer.go
// @for       Reading a Responses API answer: its output items, finish status, and
//
//	accounting.
//
// @uses      internal/schema.
// @reason    Both client directions read one decoded form, so the item readers and
//
//	the usage fold are one concern. Keeping them beside the answer
//	translator (rather than in it) holds both files inside the AGENTS.md
//	§1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

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

// responsesFinishReason maps the answer's status onto the OpenAI finish reason.
// An answer cut short by the output ceiling is a length stop, and a tool call
// always reports as one, because the client must dispatch it.
func responsesFinishReason(response object, hasCalls bool) string {
	if hasCalls {
		return FinishToolCalls
	}
	if stringField(response, "status") == "incomplete" {
		if details, ok := objectField(response, "incomplete_details"); ok {
			if stringField(details, "reason") == "max_output_tokens" {
				return FinishLength
			}
		}
	}
	return FinishStop
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
