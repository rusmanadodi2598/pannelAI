// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_claude_blocks.go
// @for       The message, tool, and image conversion the OpenAI-to-Anthropic
//
//	direction applies.
//
// @uses      internal/schema, encoding/json, strings.
// @reason    Anthropic enforces two ordering rules the OpenAI shape cannot
//
//	express — a tool_result stands alone in its own user turn and a turn
//	ends at its tool_use — and both need the same block vocabulary and
//	the same tool-name mapping. Keeping the conversion beside those rules
//	is what makes the ordering auditable in one place.
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

// claudeMessages converts a conversation into Anthropic turns, applying both
// ordering rules: tool results get their own turn, and a turn carrying tool_use
// ends at that block.
func claudeMessages(messages []schema.ChatMessage) []ClaudeMessage {
	out := make([]ClaudeMessage, 0, len(messages))
	pendingRole := ""
	pending := make([]schema.Block, 0, 2)

	flush := func() {
		if pendingRole == "" || len(pending) == 0 {
			pending = pending[:0]
			return
		}
		out = append(out, ClaudeMessage{Role: pendingRole, Content: pending})
		pending = make([]schema.Block, 0, 2)
	}

	for _, message := range messages {
		if message.Role == schema.RoleSystem || message.Role == schema.RoleDeveloper {
			continue // hoisted into the top-level system field by the caller
		}
		role := RoleAssistant
		if message.Role == schema.RoleUser || message.Role == schema.RoleTool {
			role = RoleUser
		}
		blocks := claudeBlocks(message)
		results := blocksOfType(blocks, schema.BlockToolResult)
		rest := blocksExcept(blocks, schema.BlockToolResult)

		if len(results) > 0 {
			// The tool_result turn follows the tool_use turn immediately, so the
			// assistant turn is closed first.
			flush()
			out = append(out, ClaudeMessage{Role: RoleUser, Content: results})
		}
		if len(rest) == 0 {
			continue
		}
		if pendingRole != role {
			flush()
			pendingRole = role
		}
		pending = append(pending, rest...)
		if role == RoleAssistant && hasBlock(rest, schema.BlockToolUse) {
			// Text after a tool_use in one turn is what Anthropic refuses; closing
			// the turn here keeps the ordering valid without losing the call.
			flush()
		}
	}
	flush()
	return out
}

// claudeBlocks converts one OpenAI message into Anthropic blocks.
func claudeBlocks(message schema.ChatMessage) []schema.Block {
	switch message.Role {
	case schema.RoleTool:
		return []schema.Block{{
			Type:      schema.BlockToolResult,
			ToolUseID: message.ToolCallID,
			Content:   mustJSON(message.Content.TextContent()),
		}}
	case schema.RoleAssistant:
		blocks := make([]schema.Block, 0, 2+len(message.ToolCalls))
		// Whitespace-only content is dropped rather than forwarded: Anthropic
		// rejects an empty text block, and a client that pads a turn with spaces
		// means "no text here", not "a turn made of spaces".
		if text := message.Content.TextContent(); strings.TrimSpace(text) != "" {
			blocks = append(blocks, schema.Block{Type: schema.BlockText, Text: text})
		}
		for _, call := range message.ToolCalls {
			if call.Function.Name == "" {
				continue
			}
			blocks = append(blocks, schema.Block{
				Type:  schema.BlockToolUse,
				ID:    call.ID,
				Name:  ClaudeToolPrefix + call.Function.Name,
				Input: argumentObject(call.Function.Arguments),
			})
		}
		return blocks
	default:
		return claudeUserBlocks(message.Content)
	}
}

// claudeUserBlocks converts a user message body, mapping images onto Anthropic
// image blocks and preserving a client-forwarded tool_result unchanged.
func claudeUserBlocks(content schema.MessageContent) []schema.Block {
	if content.Parts == nil {
		if content.Text == "" {
			return nil
		}
		return []schema.Block{{Type: schema.BlockText, Text: content.Text}}
	}
	blocks := make([]schema.Block, 0, len(content.Parts))
	for _, part := range content.Parts {
		switch part.Type {
		case schema.PartText:
			if part.Text != "" {
				blocks = append(blocks, schema.Block{Type: schema.BlockText, Text: part.Text})
			}
		case schema.PartImageURL:
			if block, ok := claudeImage(part.ImageURL); ok {
				blocks = append(blocks, block)
			}
		case schema.BlockToolResult:
			blocks = append(blocks, schema.Block{
				Type:      schema.BlockToolResult,
				ToolUseID: part.Text,
				Content:   part.File,
			})
		}
	}
	return blocks
}

// claudeImage converts an OpenAI image reference into an Anthropic image block: a
// data URI becomes inline base64, and an http(s) URL stays a URL the upstream
// fetches for itself.
func claudeImage(image *schema.ImageURL) (schema.Block, bool) {
	if image == nil || image.URL == "" {
		return schema.Block{}, false
	}
	if mediaType, payload, ok := splitDataURI(image.URL); ok {
		return schema.Block{Type: schema.BlockImage, Source: &schema.MediaSource{
			Type: "base64", MediaType: mediaType, Data: payload,
		}}, true
	}
	if isRemoteURL(image.URL) {
		return schema.Block{Type: schema.BlockImage, Source: &schema.MediaSource{Type: "url", URL: image.URL}}, true
	}
	return schema.Block{}, false
}

// claudeTools converts OpenAI tool declarations into Anthropic ones. An
// undeclared parameter schema becomes an empty object schema, because Anthropic
// requires input_schema to be present and an absent one is a 400.
func claudeTools(tools []schema.Tool) []ClaudeTool {
	out := make([]ClaudeTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != schema.BlockFunction || tool.Function.Name == "" {
			continue
		}
		out = append(out, ClaudeTool{
			Name:        ClaudeToolPrefix + tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.InputSchema(),
		})
	}
	return out
}

// claudeToolChoice maps the parsed OpenAI choice onto Anthropic's closed set:
// OpenAI's `required` is Anthropic's `any`, and anything else would be a 400.
func claudeToolChoice(choice schema.ToolChoice) *ClaudeToolChoice {
	switch choice.Mode {
	case "required":
		return &ClaudeToolChoice{Type: "any"}
	case "tool":
		return &ClaudeToolChoice{Type: "tool", Name: ClaudeToolPrefix + choice.Name}
	case "none":
		return &ClaudeToolChoice{Type: "none"}
	default:
		return &ClaudeToolChoice{Type: "auto"}
	}
}

// argumentObject returns a JSON object an upstream will accept for tool input:
// an unparseable or absent argument string becomes an empty object rather than
// invalid JSON, which Anthropic would reject with an opaque 400.
func argumentObject(arguments string) json.RawMessage {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(trimmed)
}

// blocksOfType returns the blocks carrying one type.
func blocksOfType(blocks []schema.Block, blockType string) []schema.Block {
	out := make([]schema.Block, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == blockType {
			out = append(out, block)
		}
	}
	return out
}

// blocksExcept returns every block but those carrying the given type.
func blocksExcept(blocks []schema.Block, blockType string) []schema.Block {
	out := make([]schema.Block, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != blockType {
			out = append(out, block)
		}
	}
	return out
}

// hasBlock reports whether any block carries the type.
func hasBlock(blocks []schema.Block, blockType string) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}
