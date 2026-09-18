// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_response.go
// @for       Non-streamed response translation between the OpenAI and Anthropic
//
//	wire formats, in both directions.
//
// @uses      internal/schema, encoding/json.
// @reason    SPEC-API-001 §7.15 serves both routes, so a client's format and the
//
//	upstream's format are independent: a Claude Code client may be served
//	by an OpenAI provider and the reverse. These functions are pure, so
//	every mapping is testable without a network, and the usage fold lives
//	beside the response it belongs to.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// ClaudeToOpenAIResponse translates an Anthropic messages response into an OpenAI
// completion. The id keeps the upstream's message id so a client correlating its
// own logs with the provider's has one identifier, not two.
func ClaudeToOpenAIResponse(raw []byte, model string, created int64) (schema.ChatCompletionResponse, error) {
	response, ok := decodeObject(raw)
	if !ok {
		return schema.ChatCompletionResponse{}, dataPlaneError(CodeUpstreamError, "the upstream answered with an unreadable body")
	}

	message := schema.ChatMessage{Role: RoleAssistant}
	texts := make([]string, 0, 2)
	for _, raw := range rawBlocks(response) {
		block, ok := decodeObject(raw)
		if !ok {
			continue
		}
		switch stringField(block, "type") {
		case schema.BlockText:
			if text := stringField(block, "text"); text != "" {
				texts = append(texts, text)
			}
		case schema.BlockThinking:
			message.Reasoning += stringField(block, "thinking")
		case schema.BlockToolUse:
			message.ToolCalls = append(message.ToolCalls, schema.ToolCall{
				ID:   stringField(block, "id"),
				Type: schema.BlockFunction,
				Function: schema.FunctionCall{
					Name:      stringField(block, "name"),
					Arguments: string(rawOrObjectField(block, "input")),
				},
			})
		}
	}
	message.Content = schema.MessageContent{Text: joinText(texts)}

	usage := schema.Usage{}
	if usageObject, ok := objectField(response, "usage"); ok {
		usage = ClaudeUsageToOpenAI(claudeUsageFromObject(usageObject))
	}
	return schema.ChatCompletionResponse{
		ID:      responseID(stringField(response, "id"), "chatcmpl-pannelai"),
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []schema.ChatChoice{{
			Index:        0,
			Message:      message,
			FinishReason: openAIFinishReason(stringField(response, "stop_reason"), TargetClaude),
		}},
		Usage: &usage,
	}, nil
}

// OpenAIToClaudeResponse translates an OpenAI completion into an Anthropic
// messages response, which is the path a client on /api/v1/messages takes when
// the resolved provider speaks OpenAI.
func OpenAIToClaudeResponse(raw []byte, model string) (schema.MessagesResponse, error) {
	response, ok := decodeObject(raw)
	if !ok {
		return schema.MessagesResponse{}, dataPlaneError(CodeUpstreamError, "the upstream answered with an unreadable body")
	}

	blocks := make([]schema.Block, 0, 2)
	var finish string
	var usage schema.Usage

	if choices, ok := arrayField(response, "choices"); ok && len(choices) > 0 {
		if choice, ok := decodeObject(choices[0]); ok {
			finish = stringField(choice, "finish_reason")
			if message, ok := objectField(choice, "message"); ok {
				blocks = append(blocks, openAIChoiceBlocks(message)...)
			}
		}
	}
	if usageObject, ok := objectField(response, "usage"); ok {
		if parsed := openAIUsageFromObject(usageObject); parsed != nil {
			usage = *parsed
		}
	}

	return schema.MessagesResponse{
		ID:           responseID(stringField(response, "id"), "msg_pannelai"),
		Type:         "message",
		Role:         RoleAssistant,
		Model:        model,
		Content:      blocks,
		StopReason:   claudeStopReason(finish),
		StopSequence: nil,
		Usage:        OpenAIToClaudeUsage(usage),
	}, nil
}

// openAIChoiceBlocks converts one OpenAI choice message into Anthropic blocks:
// text, reasoning as a thinking block, and each tool call as a tool_use block.
func openAIChoiceBlocks(message object) []schema.Block {
	blocks := make([]schema.Block, 0, 2)
	if text := stringField(message, "content"); text != "" {
		blocks = append(blocks, schema.Block{Type: schema.BlockText, Text: text})
	}
	if thinking := stringField(message, "reasoning_content"); thinking != "" {
		blocks = append(blocks, schema.Block{Type: schema.BlockThinking, Thinking: thinking})
	}
	if calls, ok := arrayField(message, "tool_calls"); ok {
		for _, raw := range calls {
			call, ok := decodeObject(raw)
			if !ok {
				continue
			}
			fn, ok := objectField(call, "function")
			if !ok {
				continue
			}
			blocks = append(blocks, schema.Block{
				Type:  schema.BlockToolUse,
				ID:    responseID(stringField(call, "id"), "toolu_pannelai"),
				Name:  stringField(fn, "name"),
				Input: rawOrObjectField(fn, "arguments"),
			})
		}
	}
	return blocks
}

// rawBlocks reads a response's content array as its raw members.
func rawBlocks(response object) []json.RawMessage {
	blocks, _ := arrayField(response, "content")
	return blocks
}

// rawOrObjectField reads a member that an upstream sends as an object but a
// client may send as a JSON string: a tool's `input` in one direction and
// `arguments` in the other are the same payload in two encodings.
func rawOrObjectField(parent object, key string) json.RawMessage {
	raw, ok := parent[key]
	if !ok {
		return json.RawMessage(`{}`)
	}
	// A string member is a JSON document in its own right for both Anthropic's
	// input and OpenAI's arguments, so it is unwrapped rather than forwarded as a
	// JSON string, which the target would reject.
	var wrapped string
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		return argumentObject(wrapped)
	}
	return raw
}

// responseID keeps an upstream identifier when it is usable and falls back to a
// stable placeholder when it is absent, so a client always has a non-empty id.
func responseID(id, fallback string) string {
	if id == "" {
		return fallback
	}
	return id
}

// joinText joins text blocks with newlines, which is how the reference flattens
// multiple text blocks for a client that reads one string.
func joinText(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	default:
		out := values[0]
		for _, value := range values[1:] {
			out += "\n" + value
		}
		return out
	}
}
