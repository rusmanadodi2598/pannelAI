// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/messages_stream.go
// @for       The streamed Anthropic wire: the one event shape, the message
//
//	intro a message_start carries, and the event and delta names.
//
// @uses      encoding/json.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/messages with stream:true as
//
//	SSE, and the event vocabulary is a closed set the translators read.
//	Keeping it beside the request and response contract in messages.go
//	crossed the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-10-03
package schema

import "encoding/json"

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
