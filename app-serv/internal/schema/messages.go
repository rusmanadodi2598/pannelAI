// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/messages.go
// @for       The Anthropic messages wire contract: the /api/v1/messages request,
//
//	its non-streamed response, and the blocks both carry.
//
// @uses      bytes, encoding/json, internal/domain.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/messages on the Anthropic
//
//	wire, and AGENTS.md §2.4 requires a typed, validated contract
//	before any handler logic. Two of its fields are genuine unions
//	(system and message content), so they are resolved once here rather
//	than by every reader downstream.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

import (
	"bytes"
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Anthropic block types, plus the OpenAI function discriminator the translators
// map onto: one closed set means a translated block has exactly one spelling.
const (
	BlockText       = "text"
	BlockImage      = "image"
	BlockDocument   = "document"
	BlockToolUse    = "tool_use"
	BlockToolResult = "tool_result"
	BlockThinking   = "thinking"
	BlockFunction   = "function"
)

// Anthropic stop reasons.
const (
	StopEndTurn      = "end_turn"
	StopMaxTokens    = "max_tokens"
	StopToolUse      = "tool_use"
	StopStopSequence = "stop_sequence"
)

// MessagesRequest is the body of POST /api/v1/messages (Anthropic wire).
type MessagesRequest struct {
	Model         string          `json:"model" validate:"required,min=1,max=200"`
	MaxTokens     *int            `json:"max_tokens,omitempty" validate:"omitempty,gte=1"`
	Messages      []Message       `json:"messages" validate:"required,min=1,dive"`
	System        TextBlocks      `json:"system,omitempty"`
	Stream        bool            `json:"stream"`
	Temperature   *float64        `json:"temperature,omitempty" validate:"omitempty,gte=0,lte=1"`
	TopP          *float64        `json:"top_p,omitempty" validate:"omitempty,gte=0,lte=1"`
	TopK          *int            `json:"top_k,omitempty" validate:"omitempty,gte=1"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Tools         []ToolBlock     `json:"tools,omitempty" validate:"dive"`
	ToolChoice    *ToolChoiceRule `json:"tool_choice,omitempty"`
	Thinking      *ThinkingBlock  `json:"thinking,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// Message is one Anthropic message.
type Message struct {
	Role    string        `json:"role" validate:"required,oneof=user assistant"`
	Content MessageBlocks `json:"content"`
}

// ToolBlock is one Anthropic tool declaration.
type ToolBlock struct {
	Name        string          `json:"name" validate:"required,min=1,max=128"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
	Type        string          `json:"type,omitempty"`
}

// ToolChoiceRule is Anthropic's tool_choice: auto, any, tool, or none.
type ToolChoiceRule struct {
	Type string `json:"type" validate:"required,oneof=auto any tool none"`
	Name string `json:"name,omitempty"`
}

// ThinkingBlock is Anthropic's extended-thinking control.
type ThinkingBlock struct {
	Type         string `json:"type" validate:"required,oneof=enabled disabled"`
	BudgetTokens int    `json:"budget_tokens,omitempty" validate:"omitempty,gte=1024"`
}

// TextBlocks is Anthropic's system field: either a bare string or an array of
// text blocks. It is a declared decode boundary, so the union is resolved here.
type TextBlocks []TextBlock

// TextBlock is one system text block.
type TextBlock struct {
	Type         string          `json:"type" validate:"omitempty,oneof=text"`
	Text         string          `json:"text"`
	CacheControl json.RawMessage `json:"cache_control,omitempty"`
}

// UnmarshalJSON accepts both permitted shapes of the system field.
func (b *TextBlocks) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return err
		}
		*b = TextBlocks{{Type: BlockText, Text: text}}
		return nil
	}
	var blocks []TextBlock
	if err := json.Unmarshal(trimmed, &blocks); err != nil {
		return err
	}
	*b = blocks
	return nil
}

// Text returns the system text with the blocks joined by newlines, which is what
// a target without a system field needs.
func (b TextBlocks) Text() string {
	var buf bytes.Buffer
	for _, block := range b {
		if block.Text == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(block.Text)
	}
	return buf.String()
}

// MessageBlocks is an Anthropic message body: either a bare string or an array
// of content blocks.
type MessageBlocks []Block

// UnmarshalJSON accepts both permitted shapes of a message body.
func (b *MessageBlocks) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return err
		}
		*b = MessageBlocks{{Type: BlockText, Text: text}}
		return nil
	}
	var blocks []Block
	if err := json.Unmarshal(trimmed, &blocks); err != nil {
		return err
	}
	*b = blocks
	return nil
}

// Block is one Anthropic content block. One struct carries every block type
// because a union of near-identical shapes would triple the decode surface for
// no added safety; the Type discriminates which members are meaningful.
type Block struct {
	Type      string          `json:"type" validate:"required"`
	Text      string          `json:"text,omitempty"`
	Source    *MediaSource    `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
}

// MediaSource is an Anthropic image or document source: inline base64 or a URL.
type MediaSource struct {
	Type      string `json:"type" validate:"required,oneof=base64 url"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// MessagesResponse is the non-streamed Anthropic answer.
type MessagesResponse struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	Role         string        `json:"role"`
	Model        string        `json:"model"`
	Content      []Block       `json:"content"`
	StopReason   string        `json:"stop_reason"`
	StopSequence *string       `json:"stop_sequence"`
	Usage        MessagesUsage `json:"usage"`
}

// MessagesUsage is Anthropic's token accounting block.
type MessagesUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// ClaudeStreamEvent is one streamed Anthropic event. One struct carries every
// event type, discriminated by Type, matching how the wire is framed.
type ClaudeStreamEvent struct {
	Type         string          `json:"type"`
	Index        *int            `json:"index,omitempty"`
	Message      *MessagesIntro  `json:"message,omitempty"`
	ContentBlock json.RawMessage `json:"content_block,omitempty"`
	Delta        json.RawMessage `json:"delta,omitempty"`
	Usage        *MessagesUsage  `json:"usage,omitempty"`
	Error        *ErrorDetail    `json:"error,omitempty"`
}

// MessagesIntro is the message object a message_start event carries.
type MessagesIntro struct {
	ID      string        `json:"id"`
	Type    string        `json:"type"`
	Role    string        `json:"role"`
	Model   string        `json:"model"`
	Content []Block       `json:"content"`
	Usage   MessagesUsage `json:"usage"`
}

// Anthropic stream event names.
const (
	EventMessageStart      = "message_start"
	EventMessageDelta      = "message_delta"
	EventMessageStop       = "message_stop"
	EventContentBlockStart = "content_block_start"
	EventContentBlockDelta = "content_block_delta"
	EventContentBlockStop  = "content_block_stop"
	EventPing              = "ping"
	EventError             = "error"
)

// Anthropic delta types.
const (
	DeltaText      = "text_delta"
	DeltaInputJSON = "input_json_delta"
	DeltaThinking  = "thinking_delta"
)

// DecodeMessagesRequest decodes an Anthropic-wire body into its typed contract.
func DecodeMessagesRequest(raw []byte) (MessagesRequest, error) {
	var req MessagesRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return MessagesRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}
