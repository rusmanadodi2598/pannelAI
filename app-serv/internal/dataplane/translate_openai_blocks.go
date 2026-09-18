// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_openai_blocks.go
// @for       The message, block, and image conversion the Anthropic-to-OpenAI
//
//	direction applies.
//
// @uses      internal/schema, encoding/json, strings.
// @reason    OpenAI pairs a tool call with a separate `tool` message and
//
//	Anthropic nests the result inside the calling turn, so this file
//	holds the split that makes a Claude Code conversation expressible on
//	an OpenAI provider. It is separated from the request builder so the
//	block rules are auditable on their own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// openAIMessages converts one Anthropic message, splitting a tool_result block
// into the separate `tool` messages OpenAI requires and folding tool_use blocks
// into the assistant message's tool_calls.
func openAIMessages(message schema.Message) []schema.ChatMessage {
	results := make([]schema.ChatMessage, 0, 1)
	rest := make([]schema.Block, 0, len(message.Content))
	for _, block := range message.Content {
		if block.Type != schema.BlockToolResult {
			rest = append(rest, block)
			continue
		}
		results = append(results, schema.ChatMessage{
			Role:       schema.RoleTool,
			ToolCallID: block.ToolUseID,
			Content:    schema.MessageContent{Text: toolResultText(block)},
		})
	}

	out := make([]schema.ChatMessage, 0, len(results)+1)
	out = append(out, results...)
	if len(rest) == 0 {
		return out
	}

	role := RoleAssistant
	if message.Role == RoleUser {
		role = RoleUser
	}
	converted := schema.ChatMessage{Role: role}
	texts := make([]string, 0, len(rest))
	parts := make([]schema.ContentPart, 0, len(rest))
	images := 0

	for _, block := range rest {
		switch block.Type {
		case schema.BlockText:
			if block.Text != "" {
				texts = append(texts, block.Text)
				parts = append(parts, schema.ContentPart{Type: schema.PartText, Text: block.Text})
			}
		case schema.BlockImage:
			if part, ok := openAIImagePart(block.Source); ok {
				parts = append(parts, part)
				images++
			}
		case schema.BlockToolUse:
			converted.ToolCalls = append(converted.ToolCalls, schema.ToolCall{
				ID:   block.ID,
				Type: schema.BlockFunction,
				Function: schema.FunctionCall{
					Name:      strings.TrimPrefix(block.Name, ClaudeToolPrefix),
					Arguments: string(block.Input),
				},
			})
		case schema.BlockThinking:
			converted.Reasoning = block.Thinking
		}
	}

	// A text-only message stays a plain string, which is what an upstream expects
	// and what makes the two directions round-trip comparably. The image count,
	// not the part count, decides: an image part carries no text and would
	// otherwise be collapsed away.
	switch {
	case images == 0 && len(texts) == len(parts) && len(texts) > 0:
		converted.Content = schema.MessageContent{Text: strings.Join(texts, "\n")}
	case len(parts) > 0:
		converted.Content = schema.MessageContent{Parts: parts}
	}
	out = append(out, converted)
	return out
}

// openAIImagePart converts an Anthropic image source into an OpenAI image part: a
// base64 source becomes a data URI, and a URL stays a URL.
func openAIImagePart(source *schema.MediaSource) (schema.ContentPart, bool) {
	if source == nil {
		return schema.ContentPart{}, false
	}
	switch source.Type {
	case "base64":
		if source.Data == "" {
			return schema.ContentPart{}, false
		}
		return schema.ContentPart{
			Type:     schema.PartImageURL,
			ImageURL: &schema.ImageURL{URL: encodeDataURI(source.MediaType, source.Data)},
		}, true
	case "url":
		if source.URL == "" {
			return schema.ContentPart{}, false
		}
		return schema.ContentPart{
			Type:     schema.PartImageURL,
			ImageURL: &schema.ImageURL{URL: source.URL},
		}, true
	default:
		return schema.ContentPart{}, false
	}
}

// toolResultText flattens a tool_result body into the string OpenAI's tool message
// carries: a bare string stays as it is, a block array is joined by newlines, and
// anything else keeps its JSON so no result is silently lost.
func toolResultText(block schema.Block) string {
	if len(block.Content) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(block.Content, &text); err == nil {
		return text
	}
	var content []schema.Block
	if err := json.Unmarshal(block.Content, &content); err == nil {
		joined := make([]string, 0, len(content))
		for _, part := range content {
			if part.Type == schema.BlockText && part.Text != "" {
				joined = append(joined, part.Text)
			}
		}
		if len(joined) > 0 {
			return strings.Join(joined, "\n")
		}
	}
	return string(block.Content)
}
