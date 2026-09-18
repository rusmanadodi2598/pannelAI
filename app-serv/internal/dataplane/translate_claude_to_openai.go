// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_claude_to_openai.go
// @for       Anthropic messages request to OpenAI chat request translation.
// @uses      internal/schema, strings.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/messages to Claude Code while
//
//	the resolved provider may speak OpenAI, so this direction is what
//	makes the Anthropic route provider-agnostic. It is a pure function
//	(the reference's claude-to-openai.js) so it is testable without a
//	network.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// ClaudeToOpenAI translates an Anthropic messages request into an OpenAI chat
// request. upstreamModel is the id the upstream expects.
//
// Anthropic's top-level system field becomes a leading system message because
// OpenAI has no equivalent field; a tool_result block becomes its own `tool`
// message because OpenAI pairs a tool call with a separate message rather than
// nesting the result inside the calling turn.
func ClaudeToOpenAI(req schema.MessagesRequest, upstreamModel string, stream bool) schema.ChatRequest {
	out := schema.ChatRequest{
		Model:       upstreamModel,
		Stream:      stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        stopArray(req.StopSequences),
	}
	if max := maxTokensOf(req.MaxTokens); max > 0 {
		out.MaxTokens = &max
	}
	if text := req.System.Text(); text != "" {
		out.Messages = append(out.Messages, schema.ChatMessage{
			Role: schema.RoleSystem, Content: schema.MessageContent{Text: text},
		})
	}
	for _, message := range req.Messages {
		out.Messages = append(out.Messages, openAIMessages(message)...)
	}
	for _, tool := range req.Tools {
		if tool.Name == "" {
			continue
		}
		out.Tools = append(out.Tools, schema.Tool{
			Type: schema.BlockFunction,
			Function: schema.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	if req.ToolChoice != nil {
		out.ToolChoice = openAIToolChoice(req.ToolChoice)
	}
	return out
}

// openAIToolChoice maps Anthropic's tool_choice onto the OpenAI spelling:
// Anthropic's `any` is OpenAI's `required`, and a named Anthropic tool is
// OpenAI's forced-function object.
func openAIToolChoice(choice *schema.ToolChoiceRule) []byte {
	switch choice.Type {
	case "any":
		return []byte(`"required"`)
	case "none":
		return []byte(`"none"`)
	case "tool":
		return mustJSON(map[string]any{
			"type":     schema.BlockFunction,
			"function": map[string]string{"name": choice.Name},
		})
	default:
		return []byte(`"auto"`)
	}
}

// maxTokensOf returns the value of a possibly-absent ceiling.
func maxTokensOf(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

// stopArray renders stop sequences for the OpenAI `stop` field, which accepts an
// array or a single string. An empty list is omitted rather than sent as [].
func stopArray(sequences []string) []byte {
	if len(sequences) == 0 {
		return nil
	}
	return mustJSON(sequences)
}

// NormalizeClaudeTx strips the billing header a Claude Code client injects into
// its system prompt.
//
// The reference removes it before forwarding (claude-to-openai.js,
// stripAnthropicBillingHeader) because it is a client-side accounting header, not
// part of the conversation: forwarding it changes what the upstream sees and, for
// an Anthropic upstream, double-reports the session.
func NormalizeClaudeTx(text string) string {
	const marker = "x-anthropic-billing-header:"
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(strings.ToLower(trimmed), marker) {
		return text
	}
	if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
		return strings.TrimLeft(trimmed[newline+1:], "\r\n")
	}
	return ""
}
