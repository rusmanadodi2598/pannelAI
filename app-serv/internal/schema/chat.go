// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat.go
// @for       The OpenAI-wire chat request and the body reader the data plane
//
//	decodes it from.
//
// @uses      bytes, encoding/json, io, net/http, internal/domain.
// @reason    SPEC-API-001 §7.15 serves CLI tools the OpenAI wire format, and
//
//	AGENTS.md §2.4 requires a typed, validated contract before any
//	handler logic. Unknown fields are deliberately accepted here,
//	unlike the management plane: this endpoint proxies a third-party
//	protocol whose field set grows without notice, so refusing an
//	unknown key would break a client over a field the gateway was only
//	going to pass along.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// DataPlaneFormat names the wire format a data plane request or response is
// written in (SPEC-API-001 §7.15: OpenAI and Anthropic in P1).
type DataPlaneFormat string

const (
	// FormatOpenAI is the OpenAI chat completions wire format.
	FormatOpenAI DataPlaneFormat = "openai"
	// FormatAnthropic is the Anthropic messages wire format.
	FormatAnthropic DataPlaneFormat = "anthropic"
	// FormatOpenAIResponses is the OpenAI Responses wire format, which POST
	// /api/v1/responses serves. It is a client wire of its own rather than a
	// variant of FormatOpenAI: its request is an item array, not a message
	// array, and its answer is a response object, not a choice list.
	FormatOpenAIResponses DataPlaneFormat = "openai-responses"
)

// ReadBody reads a bounded request body. The raw bytes are kept because a
// same-format upstream target is forwarded verbatim: re-serialising a decoded
// DTO would silently drop every field the gateway does not model.
func ReadBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, domain.NewValidationError("request body is required")
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, MaxBodyBytes+1))
	if err != nil {
		return nil, domain.NewValidationError("request body could not be read")
	}
	if len(raw) > MaxBodyBytes {
		return nil, domain.NewValidationError("request body is too large")
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, domain.NewValidationError("request body is required")
	}
	return raw, nil
}

// ChatRequest is the body of POST /api/v1/chat/completions. Optional members
// carry omitempty so the type can also be marshalled into an upstream OpenAI
// body: sending an explicit null is not the same request as omitting a field.
type ChatRequest struct {
	Model               string          `json:"model" validate:"required,min=1,max=200"`
	Messages            []ChatMessage   `json:"messages" validate:"required,min=1,dive"`
	Stream              bool            `json:"stream"`
	StreamOptions       *StreamOptions  `json:"stream_options,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty" validate:"omitempty,gte=0,lte=2"`
	TopP                *float64        `json:"top_p,omitempty" validate:"omitempty,gte=0,lte=1"`
	MaxTokens           *int            `json:"max_tokens,omitempty" validate:"omitempty,gte=1"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty" validate:"omitempty,gte=1"`
	Stop                json.RawMessage `json:"stop,omitempty"`
	Tools               []Tool          `json:"tools,omitempty" validate:"dive"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
	ResponseFormat      *ResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
}

// StreamOptions is the OpenAI streaming control block; IncludeUsage is what
// makes the gateway emit a final usage chunk (SPEC-API-001 §4).
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// IncludeUsage reports whether the caller asked for a usage chunk.
func (r ChatRequest) IncludeUsage() bool {
	return r.StreamOptions != nil && r.StreamOptions.IncludeUsage
}

// MaxOutputTokens reports the requested output ceiling, whichever of the two
// OpenAI spellings the client used. Zero means the client asked for none, which
// is distinct from asking for the smallest possible answer.
func (r ChatRequest) MaxOutputTokens() int {
	if r.MaxTokens != nil && *r.MaxTokens > 0 {
		return *r.MaxTokens
	}
	if r.MaxCompletionTokens != nil && *r.MaxCompletionTokens > 0 {
		return *r.MaxCompletionTokens
	}
	return 0
}

// StopSequences reads the `stop` field in either permitted shape: a single
// string or an array of them. An unparseable value yields none, which is the
// same request as one that omitted the field.
func (r ChatRequest) StopSequences() []string {
	trimmed := bytes.TrimSpace(r.Stop)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		var single string
		if err := json.Unmarshal(trimmed, &single); err != nil || single == "" {
			return nil
		}
		return []string{single}
	}
	var many []string
	if err := json.Unmarshal(trimmed, &many); err != nil {
		return nil
	}
	return many
}

// ResponseFormat is OpenAI's response_format control.
type ResponseFormat struct {
	Type       string           `json:"type" validate:"required,oneof=text json_object json_schema"`
	JSONSchema *JSONSchemaField `json:"json_schema,omitempty"`
}

// JSONSchemaField carries the named schema a json_schema response format
// declares. The schema itself stays raw JSON so no part of it is lost.
type JSONSchemaField struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict,omitempty"`
	Schema json.RawMessage `json:"schema"`
}

// ChatMessage is one OpenAI message.
type ChatMessage struct {
	Role       string         `json:"role" validate:"required"`
	Content    MessageContent `json:"content"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Refusal    string         `json:"refusal,omitempty"`
	Reasoning  string         `json:"reasoning_content,omitempty"`
}

// ChatRoles are the roles the OpenAI wire format defines.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
	RoleDeveloper = "developer"
)

// DecodeChatRequest decodes an OpenAI-wire body into its typed contract.
func DecodeChatRequest(raw []byte) (ChatRequest, error) {
	var req ChatRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return ChatRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}

// jsonErrorTail keeps a bounded English tail of a codec error so the envelope
// never echoes a large or driver-specific message.
func jsonErrorTail(err error) string {
	msg := err.Error()
	if len(msg) > 128 {
		return msg[len(msg)-128:]
	}
	return msg
}
