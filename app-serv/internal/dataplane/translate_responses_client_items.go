// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_client_items.go
// @for       The item-level rules the Responses-to-chat translation follows:
//
//	which item becomes which message, and how content and tools map.
//
// @uses      encoding/json, strings, internal/schema.
// @reason    SPEC-API-001 §7.15 serves the Responses wire, and its item array is
//
//	where the two wires differ most: one assistant turn is assembled
//	from several items, a tool result is an item rather than a message,
//	and reasoning attaches to the turn that follows it. Keeping the
//	rules beside the translator (rather than in it) holds both files
//	inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// responsesClientMessages turns the input array into a chat message list.
//
// One assistant turn is assembled from several items: a `function_call` item
// becomes a tool call on an assistant message, and consecutive calls share one
// message because that is the shape the chat wire uses for parallel calls. A
// `reasoning` item carries no message of its own; its text attaches to the
// assistant turn that follows it, which is where the chat wire puts it.
func responsesClientMessages(input schema.ResponsesInput) []schema.ChatMessage {
	if input.Items == nil {
		if input.Text == "" {
			return nil
		}
		return []schema.ChatMessage{{
			Role: schema.RoleUser, Content: schema.MessageContent{Text: input.Text},
		}}
	}

	out := make([]schema.ChatMessage, 0, len(input.Items))
	var pending *schema.ChatMessage
	pendingReasoning := ""
	flush := func() {
		if pending == nil {
			return
		}
		// A turn that gathered no call is dropped: it exists only to carry the
		// calls, and an assistant message with no content is rejected outright by
		// Anthropic. A lone nameless call is what produces one, since the call
		// itself was skipped.
		if len(pending.ToolCalls) > 0 {
			out = append(out, *pending)
		}
		pending = nil
	}

	for _, item := range input.Items {
		switch responsesItemKind(item) {
		case schema.ResponsesItemMessage:
			flush()
			message := schema.ChatMessage{Role: item.Role, Content: responsesClientContent(item.Content)}
			if item.Role == schema.RoleAssistant && pendingReasoning != "" {
				message.Reasoning = pendingReasoning
			}
			pendingReasoning = ""
			out = append(out, message)
		case schema.ResponsesItemFunctionCall:
			if pending == nil {
				pending = &schema.ChatMessage{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{}}
				if pendingReasoning != "" {
					pending.Reasoning = pendingReasoning
					pendingReasoning = ""
				}
			}
			// A nameless call is skipped: the API rejects one, and forwarding it
			// would fail the whole request (reference #444).
			if item.Name == "" {
				continue
			}
			pending.ToolCalls = append(pending.ToolCalls, schema.ToolCall{
				ID: item.CallID, Type: schema.BlockFunction,
				Function: schema.FunctionCall{Name: item.Name, Arguments: responsesArguments(item.Arguments)},
			})
		case schema.ResponsesItemFunctionCallIn:
			flush()
			out = append(out, schema.ChatMessage{
				Role: schema.RoleTool, ToolCallID: item.CallID,
				Content: schema.MessageContent{Text: responsesToolResult(item.Output)},
			})
		case schema.ResponsesItemReasoning:
			if text := responsesReasoningText(item); text != "" {
				if pendingReasoning != "" {
					pendingReasoning += "\n"
				}
				pendingReasoning += text
			}
		}
	}
	flush()
	return out
}

// responsesItemKind reads an item's kind, falling back to the message kind when
// only a role is present: the Droid CLI sends role-based items with no `type`,
// and skipping them would drop the conversation.
func responsesItemKind(item schema.ResponsesItem) string {
	if item.Type != "" {
		return item.Type
	}
	if item.Role != "" {
		return schema.ResponsesItemMessage
	}
	return ""
}

// responsesClientContent maps an item's content onto the chat wire's union.
func responsesClientContent(body schema.ResponsesItemBody) schema.MessageContent {
	if body.Parts == nil {
		return schema.MessageContent{Text: body.Text}
	}
	parts := make([]schema.ContentPart, 0, len(body.Parts))
	for _, part := range body.Parts {
		parts = append(parts, responsesClientPart(part))
	}
	return schema.MessageContent{Parts: parts}
}

// responsesClientPart maps one Responses content part onto its chat spelling.
//
// A part kind the chat wire has no field for keeps its own type name and text,
// which is the closest a typed payload gets to the reference's pass-through: the
// upstream sees a kind it may understand even though the gateway does not model
// it, rather than a part silently rewritten into a text block.
func responsesClientPart(part schema.ResponsesItemPart) schema.ContentPart {
	switch part.Type {
	case schema.ResponsesPartInputText, schema.ResponsesPartOutputText:
		return schema.ContentPart{Type: schema.PartText, Text: part.Text}
	case schema.ResponsesPartInputImage:
		reference := part.ImageURL
		if reference == "" {
			reference = part.FileID
		}
		detail := part.Detail
		if detail == "" {
			detail = "auto"
		}
		return schema.ContentPart{
			Type: schema.PartImageURL, ImageURL: &schema.ImageURL{URL: reference, Detail: detail},
		}
	default:
		return schema.ContentPart{Type: part.Type, Text: part.Text}
	}
}

// responsesReasoningText reads a reasoning item's text, from its summary parts
// when it has them and from its content parts otherwise: providers disagree on
// which member carries it, and the reference reads both.
func responsesReasoningText(item schema.ResponsesItem) string {
	texts := make([]string, 0, len(item.Summary))
	for _, part := range item.Summary {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	if len(texts) == 0 {
		for _, part := range item.Content.Parts {
			if part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
	}
	return strings.Join(texts, "\n")
}

// responsesToolResult renders a tool result as the string the chat wire requires,
// whatever shape the client sent it in: a JSON string is unwrapped, and anything
// else keeps its JSON text so a structured payload survives.
func responsesToolResult(output json.RawMessage) string {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return ""
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err == nil {
			return text
		}
	}
	return string(trimmed)
}

// responsesArguments keeps a call's argument string, defaulting an absent one to
// an empty object: a chat upstream rejects a call with no arguments member.
func responsesArguments(arguments string) string {
	if arguments == "" {
		return "{}"
	}
	return arguments
}
