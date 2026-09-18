// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_parts.go
// @for       The multimodal and tool-calling members of the OpenAI chat
//
//	contract: content parts, images, tool definitions, and tool_choice.
//
// @uses      bytes, encoding/json, internal/domain.
// @reason    SPEC-API-001 §7.15 routes image-bearing and tool-calling requests, and
//
//	AGENTS.md §1.4 allows the union decoders only at a declared I/O
//	boundary: content is genuinely two shapes on the wire (a string or an
//	array of parts), so resolving it once here is what keeps a
//	type-assertion chain out of the translator.
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

// OpenAI content-part types.
const (
	PartText       = "text"
	PartImageURL   = "image_url"
	PartInputAudio = "input_audio"
	PartFile       = "file"
)

// MessageContent is an OpenAI message body: either a plain string or an array of
// typed parts. It is a declared decode boundary, so the union is resolved once
// here instead of at every reader.
type MessageContent struct {
	Text  string
	Parts []ContentPart
}

// UnmarshalJSON accepts both permitted shapes of a message body.
func (c *MessageContent) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		return json.Unmarshal(trimmed, &c.Text)
	}
	var parts []ContentPart
	if err := json.Unmarshal(trimmed, &parts); err != nil {
		return domain.NewValidationError("message content must be a string or an array of parts")
	}
	c.Parts = parts
	return nil
}

// MarshalJSON writes the content back in the shape it arrived in, so a forwarded
// message is byte-comparable to what the client sent.
func (c MessageContent) MarshalJSON() ([]byte, error) {
	if c.Parts == nil {
		return json.Marshal(c.Text)
	}
	return json.Marshal(c.Parts)
}

// IsEmpty reports whether the message carries no content at all.
func (c MessageContent) IsEmpty() bool {
	return c.Text == "" && len(c.Parts) == 0
}

// TextContent flattens the textual parts, joined by newlines the way the
// reference flattens multimodal input for a text-only target.
func (c MessageContent) TextContent() string {
	if c.Parts == nil {
		return c.Text
	}
	var buf bytes.Buffer
	for _, part := range c.Parts {
		if part.Type == PartText && part.Text != "" {
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			buf.WriteString(part.Text)
		}
	}
	return buf.String()
}

// Images lists the image references in the content, in order.
func (c MessageContent) Images() []ImageURL {
	out := make([]ImageURL, 0, len(c.Parts))
	for _, part := range c.Parts {
		if part.Type == PartImageURL && part.ImageURL != nil && part.ImageURL.URL != "" {
			out = append(out, *part.ImageURL)
		}
	}
	return out
}

// HasImage reports whether the content carries an image.
func (c MessageContent) HasImage() bool { return len(c.Images()) > 0 }

// ContentPart is one entry of a multimodal OpenAI message body.
type ContentPart struct {
	Type     string          `json:"type" validate:"required"`
	Text     string          `json:"text,omitempty"`
	ImageURL *ImageURL       `json:"image_url,omitempty"`
	Audio    json.RawMessage `json:"input_audio,omitempty"`
	File     json.RawMessage `json:"file,omitempty"`
}

// ImageURL is an OpenAI image reference: an http(s) URL or a base64 data URI.
type ImageURL struct {
	URL    string `json:"url" validate:"required"`
	Detail string `json:"detail,omitempty"`
}

// ToolCall is one OpenAI function invocation.
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function"`
}

// FunctionCall is the name and JSON-encoded arguments of a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool is one OpenAI function definition.
type Tool struct {
	Type     string       `json:"type" validate:"required,oneof=function"`
	Function ToolFunction `json:"function"`
}

// ToolFunction is the declaration of a callable function.
type ToolFunction struct {
	Name        string          `json:"name" validate:"required,min=1,max=128"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

// InputSchema returns the parameter schema, falling back to an empty object
// schema so an upstream that requires one always receives a valid shape.
func (t ToolFunction) InputSchema() json.RawMessage {
	if len(bytes.TrimSpace(t.Parameters)) > 0 {
		return t.Parameters
	}
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

// ToolChoice is the parsed tool_choice control.
type ToolChoice struct {
	// Mode is auto, none, required, or tool.
	Mode string
	// Name is set only when Mode is tool.
	Name string
}

// ParseToolChoice reads every spelling this endpoint accepts: the OpenAI string
// form, OpenAI's forced-function object, and the Anthropic-native object a client
// may forward through the OpenAI route. An unrecognised type is refused rather
// than passed upstream, where it would fail as an opaque 400 naming nothing.
func ParseToolChoice(raw json.RawMessage) (ToolChoice, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return ToolChoice{}, nil
	}
	if trimmed[0] == '"' {
		var mode string
		if err := json.Unmarshal(trimmed, &mode); err != nil {
			return ToolChoice{}, domain.NewValidationError("tool_choice must be a string or an object")
		}
		switch mode {
		case "auto", "none", "required":
			return ToolChoice{Mode: mode}, nil
		default:
			return ToolChoice{}, domain.NewValidationError("tool_choice " + mode + " is not supported")
		}
	}
	var shaped struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(trimmed, &shaped); err != nil {
		return ToolChoice{}, domain.NewValidationError("tool_choice must be a string or an object")
	}
	switch shaped.Type {
	case "tool", "any":
		if shaped.Name == "" {
			return ToolChoice{Mode: "required"}, nil
		}
		return ToolChoice{Mode: "tool", Name: shaped.Name}, nil
	case "function":
		if shaped.Function.Name == "" {
			return ToolChoice{}, domain.NewValidationError("tool_choice requires a function name")
		}
		return ToolChoice{Mode: "tool", Name: shaped.Function.Name}, nil
	case "auto", "none":
		return ToolChoice{Mode: shaped.Type}, nil
	default:
		return ToolChoice{}, domain.NewValidationError("tool_choice type " + shaped.Type + " is not supported")
	}
}
