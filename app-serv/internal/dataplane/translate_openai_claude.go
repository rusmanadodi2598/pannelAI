// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_openai_claude.go
// @for       OpenAI chat request to Anthropic messages request translation.
// @uses      internal/schema, encoding/json, strings.
// @reason    SPEC-API-001 §7.15 makes translation the data plane's core job, and
//
//	the reference implements it as a pure function
//	(open-sse/translator/request/openai-to-claude.js) precisely so it is
//	testable without a network. This is that function: no clock, no I/O,
//	no package state, so a test drives every rule directly.
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

// ClaudeToolPrefix is prepended to a tool name when the upstream needs a
// namespaced name. The reference ships an empty prefix as its current default
// (`CLAUDE_OAUTH_TOOL_PREFIX = ""` in openai-to-claude.js) because a non-empty
// prefix is a detectable client fingerprint; the response direction strips the
// same value, so raising this one constant keeps both halves in step.
const ClaudeToolPrefix = ""

// DefaultClaudeMaxTokens is the ceiling applied when the client asked for none.
// Claude's API requires max_tokens, and the reference's runtime default is 64000
// (open-sse/config/runtimeConfig.js).
const DefaultClaudeMaxTokens = 64000

// ClaudeThinkingEnabled is the thinking type the translator emits when a client
// asked for reasoning effort.
const ClaudeThinkingEnabled = "enabled"

// ClaudeRequest is the Anthropic messages payload the translator produces.
type ClaudeRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	Stream        bool               `json:"stream"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	System        []schema.TextBlock `json:"system,omitempty"`
	Messages      []ClaudeMessage    `json:"messages"`
	Tools         []ClaudeTool       `json:"tools,omitempty"`
	ToolChoice    *ClaudeToolChoice  `json:"tool_choice,omitempty"`
}

// ClaudeMessage is one Anthropic message. Its content is always a block array:
// Anthropic accepts a bare string, but emitting the array form is what keeps a
// tool_result and its text in one shape across every message.
type ClaudeMessage struct {
	Role    string         `json:"role"`
	Content []schema.Block `json:"content"`
}

// ClaudeTool is one Anthropic tool declaration.
type ClaudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ClaudeToolChoice is Anthropic's tool_choice. It accepts only auto, any, tool,
// and none, so an OpenAI type is never passed through unmapped — that is a 400
// from the upstream, not a behaviour difference.
type ClaudeToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// OpenAIToClaude translates an OpenAI chat request into an Anthropic messages
// request. upstreamModel is the id the upstream expects.
//
// System messages are hoisted into Anthropic's top-level system field because the
// messages array cannot carry them: a system role there is rejected, not ignored.
// Tool results are emitted in their own user turn immediately after the assistant
// turn that called the tool, which is Claude's ordering requirement, and a turn
// that carries tool_use ends there rather than trailing text Claude would reject.
func OpenAIToClaude(req schema.ChatRequest, upstreamModel string, stream bool) ClaudeRequest {
	out := ClaudeRequest{
		Model:         upstreamModel,
		MaxTokens:     adjustClaudeMaxTokens(req),
		Stream:        stream,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.StopSequences(),
		Messages:      claudeMessages(req.Messages),
		Tools:         claudeTools(req.Tools),
	}

	system := make([]string, 0, 2)
	for _, message := range req.Messages {
		if message.Role == schema.RoleSystem || message.Role == schema.RoleDeveloper {
			if text := message.Content.TextContent(); text != "" {
				system = append(system, text)
			}
		}
	}
	system = append(system, responseFormatInstruction(req.ResponseFormat)...)
	if len(system) > 0 {
		out.System = []schema.TextBlock{{Type: schema.BlockText, Text: strings.Join(system, "\n")}}
	}

	if len(req.ToolChoice) > 0 {
		if choice, err := schema.ParseToolChoice(req.ToolChoice); err == nil && choice.Mode != "" {
			out.ToolChoice = claudeToolChoice(choice)
		}
	}
	return out
}

// ClaudeUsageToOpenAI folds an Anthropic usage block into the OpenAI accounting
// block.
//
// prompt_tokens is input + cache_read + cache_creation because OpenAI's
// prompt_tokens counts every prompt-side token, and Claude reports the cached
// ones separately (open-sse/translator/concerns/usage.js, the `claude` extractor).
// Reporting only input_tokens would understate the prompt on every cached turn.
func ClaudeUsageToOpenAI(usage schema.MessagesUsage) schema.Usage {
	prompt := usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
	total := prompt + usage.OutputTokens
	out := schema.Usage{PromptTokens: prompt, CompletionTokens: usage.OutputTokens, TotalTokens: total}
	if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 {
		out.PromptTokensDetails = &schema.PromptTokensDetails{
			CachedTokens:         usage.CacheReadInputTokens,
			CacheCreationTokens:  usage.CacheCreationInputTokens,
			CacheReadInputTokens: usage.CacheReadInputTokens,
		}
	}
	return out
}

// OpenAIToClaudeUsage is the inverse fold, for a response translated back into
// the Anthropic envelope.
//
// input_tokens excludes the cached parts because OpenAI's prompt_tokens already
// includes them: subtracting is what makes a round trip report the same input
// count the upstream did (open-sse/translator/response/openai-to-claude.js).
func OpenAIToClaudeUsage(usage schema.Usage) schema.MessagesUsage {
	out := schema.MessagesUsage{OutputTokens: usage.CompletionTokens}
	if usage.PromptTokensDetails != nil {
		out.CacheReadInputTokens = usage.PromptTokensDetails.CachedTokens
		out.CacheCreationInputTokens = usage.PromptTokensDetails.CacheCreationTokens
	}
	out.InputTokens = usage.PromptTokens - out.CacheReadInputTokens - out.CacheCreationInputTokens
	if out.InputTokens < 0 {
		// A malformed upstream that reported cached tokens above the prompt total
		// still yields a non-negative count: Anthropic's field is unsigned, and a
		// negative value would be rejected rather than merely wrong.
		out.InputTokens = 0
	}
	return out
}

// adjustClaudeMaxTokens applies the reference's max_tokens rule
// (formats/maxTokens.js): the default when absent, never above the ceiling.
func adjustClaudeMaxTokens(req schema.ChatRequest) int {
	maxTokens := req.MaxOutputTokens()
	if maxTokens <= 0 || maxTokens > DefaultClaudeMaxTokens {
		return DefaultClaudeMaxTokens
	}
	return maxTokens
}

// responseFormatInstruction turns a structured-output request into a system
// instruction, which is what the reference does for every target with no
// response_format of its own (openai-to-claude.js).
func responseFormatInstruction(format *schema.ResponseFormat) []string {
	if format == nil {
		return nil
	}
	switch format.Type {
	case "json_schema":
		if format.JSONSchema == nil || len(format.JSONSchema.Schema) == 0 {
			return nil
		}
		pretty := format.JSONSchema.Schema
		if indented, err := json.MarshalIndent(format.JSONSchema.Schema, "", "  "); err == nil {
			pretty = indented
		}
		return []string{"You must respond with valid JSON that strictly follows this JSON schema:\n```json\n" +
			string(pretty) + "\n```\nRespond ONLY with the JSON object, no other text."}
	case "json_object":
		return []string{"You must respond with valid JSON. Respond ONLY with a JSON object, no other text."}
	default:
		return nil
	}
}
