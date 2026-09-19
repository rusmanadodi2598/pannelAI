// Package schema holds the typed contracts each route decodes into.
//
// @file      internal/schema/responses.go
// @for       The OpenAI Responses request contract: its item vocabulary, the
//
//	input union, and the fields a Responses-only body may carry.
//
// @uses      encoding/json, internal/domain.
// @reason    SPEC-API-001 §7.15 makes POST /api/v1/responses a P3 deliverable,
//
//	and the wire differs from the chat completions contract in three
//	ways that have to be decoded rather than guessed: `input` is a
//	union of a bare string and an item array, an item's content is a
//	union of a bare string and a part array, and an item's kind is
//	carried by `type` with a role fallback the CLI tools rely on.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"bytes"
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Responses item kinds the gateway reads. One spelling each.
const (
	ResponsesItemMessage        = "message"
	ResponsesItemFunctionCall   = "function_call"
	ResponsesItemFunctionCallIn = "function_call_output"
	ResponsesItemReasoning      = "reasoning"
)

// Responses content-part kinds the gateway reads.
const (
	ResponsesPartInputText  = "input_text"
	ResponsesPartOutputText = "output_text"
	ResponsesPartInputImage = "input_image"
)

// ResponsesRequest is the body of POST /api/v1/responses.
//
// The fields the gateway does not model are declared so a decode does not fail
// on them: a coding agent sends `include`, `prompt_cache_key`, `store`, and a
// `reasoning` block, and refusing a body for carrying a field the upstream
// understands would make the route unusable for the tools it exists to serve.
type ResponsesRequest struct {
	Model           string              `json:"model" validate:"required,min=1,max=200"`
	Input           ResponsesInput      `json:"input"`
	Instructions    string              `json:"instructions,omitempty"`
	Stream          bool                `json:"stream,omitempty"`
	Tools           []ResponsesToolDecl `json:"tools,omitempty" validate:"dive"`
	Temperature     *float64            `json:"temperature,omitempty"`
	TopP            *float64            `json:"top_p,omitempty"`
	MaxOutputTokens *int                `json:"max_output_tokens,omitempty" validate:"omitempty,gte=1"`
	Store           *bool               `json:"store,omitempty"`
	Include         []string            `json:"include,omitempty"`
	PromptCacheKey  string              `json:"prompt_cache_key,omitempty"`
	Reasoning       json.RawMessage     `json:"reasoning,omitempty"`
}

// ResponsesInput is the `input` union: a bare string stands for one user turn,
// and an array carries the conversation's items.
type ResponsesInput struct {
	// Text is the bare-string form, which is one user message's content.
	Text string
	// Items is the array form.
	Items []ResponsesItem
}

// UnmarshalJSON accepts both permitted shapes of `input`.
func (i *ResponsesInput) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		return json.Unmarshal(trimmed, &i.Text)
	}
	if trimmed[0] != '[' {
		return domain.NewValidationError("input must be a string or an array of items")
	}
	return json.Unmarshal(trimmed, &i.Items)
}

// MarshalJSON writes the input back in the shape it arrived in, so a forwarded
// body stays byte-comparable to what the client sent.
func (i ResponsesInput) MarshalJSON() ([]byte, error) {
	if i.Items == nil {
		return json.Marshal(i.Text)
	}
	return json.Marshal(i.Items)
}

// ResponsesItem is one entry of the input array. One struct carries every item
// kind, discriminated by Type, matching how the wire frames it.
type ResponsesItem struct {
	Type      string             `json:"type,omitempty"`
	Role      string             `json:"role,omitempty"`
	Content   ResponsesItemBody  `json:"content,omitempty"`
	CallID    string             `json:"call_id,omitempty"`
	Name      string             `json:"name,omitempty"`
	Arguments string             `json:"arguments,omitempty"`
	Output    json.RawMessage    `json:"output,omitempty"`
	Summary   []ResponsesSummary `json:"summary,omitempty"`
	Status    string             `json:"status,omitempty"`
	ID        string             `json:"id,omitempty"`
}

// ResponsesItemBody is an item's `content` union: a bare string or an array of
// parts. It mirrors MessageContent, which cannot be reused here because the
// Responses part names and the string-valued `image_url` differ from OpenAI's.
type ResponsesItemBody struct {
	Text  string
	Parts []ResponsesItemPart
}

// UnmarshalJSON accepts both permitted shapes of an item's content.
func (b *ResponsesItemBody) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		return json.Unmarshal(trimmed, &b.Text)
	}
	if trimmed[0] != '[' {
		return domain.NewValidationError("an item's content must be a string or an array of parts")
	}
	return json.Unmarshal(trimmed, &b.Parts)
}

// MarshalJSON writes the content back in the shape it arrived in.
func (b ResponsesItemBody) MarshalJSON() ([]byte, error) {
	if b.Parts == nil {
		return json.Marshal(b.Text)
	}
	return json.Marshal(b.Parts)
}

// IsEmpty reports whether the body carries no content at all.
func (b ResponsesItemBody) IsEmpty() bool { return b.Text == "" && len(b.Parts) == 0 }

// ResponsesItemPart is one part of an item's content. `image_url` is a bare
// string here rather than OpenAI's object, which is the wire's own shape.
type ResponsesItemPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// ResponsesSummary is one entry of a reasoning item's summary.
type ResponsesSummary struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// ResponsesToolDecl is one tool declaration. The Responses API flattens OpenAI's
// `function` wrapper, so a declaration that still carries one is accepted too: a
// client may send either, and both mean the same tool.
type ResponsesToolDecl struct {
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
	Function    *ToolFunction   `json:"function,omitempty"`
}

// DecodeResponsesRequest decodes a Responses-wire body into its typed contract.
func DecodeResponsesRequest(raw []byte) (ResponsesRequest, error) {
	var req ResponsesRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return ResponsesRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}
