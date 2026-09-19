// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_items.go
// @for       The Responses API payload types and the item builders the request
//
//	translator writes them with.
//
// @uses      internal/schema, encoding/json.
// @reason    The item vocabulary is a shape of its own: three item types, two
//
//	content-part types, and the rules that decide which one a chat member
//	becomes. Keeping it beside the request translator (rather than in it)
//	holds both files inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// The Responses API item and content-part names this translator emits. One
// spelling each, because the format has no synonyms.
const (
	ItemMessage         = "message"
	ItemFunctionCall    = "function_call"
	ItemFunctionCallOut = "function_call_output"
	ItemInputText       = "input_text"
	ItemOutputText      = "output_text"
	ItemInputImage      = "input_image"
)

// ResponsesRequest is the body the translator builds for a Responses API
// upstream.
type ResponsesRequest struct {
	Model        string          `json:"model"`
	Input        []ResponsesItem `json:"input"`
	Instructions string          `json:"instructions"`
	Stream       bool            `json:"stream"`
	Store        bool            `json:"store"`
	Temperature  *float64        `json:"temperature,omitempty"`
	MaxTokens    *int            `json:"max_tokens,omitempty"`
	TopP         *float64        `json:"top_p,omitempty"`
	Tools        []ResponsesTool `json:"tools,omitempty"`
}

// ResponsesItem is one entry of the input array. One struct carries every item
// type, discriminated by Type, matching how the wire frames it.
type ResponsesItem struct {
	Type      string             `json:"type"`
	Role      string             `json:"role,omitempty"`
	Content   []ResponsesContent `json:"content,omitempty"`
	CallID    string             `json:"call_id,omitempty"`
	Name      string             `json:"name,omitempty"`
	Arguments string             `json:"arguments,omitempty"`
	Output    string             `json:"output,omitempty"`
}

// ResponsesContent is one content part of a message item.
type ResponsesContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// ResponsesTool is one function declaration in the Responses vocabulary, which
// flattens OpenAI's `function` wrapper into the tool itself.
type ResponsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      *bool           `json:"strict,omitempty"`
}

// responsesMessageItem builds the message item one chat message stands for, or
// reports false when it carries no content: an assistant turn with only tool
// calls has none, and its calls travel as their own items.
func responsesMessageItem(message schema.ChatMessage) (ResponsesItem, bool) {
	content := make([]ResponsesContent, 0, len(message.Content.Parts)+1)
	if message.Content.Parts == nil {
		if message.Content.Text != "" {
			content = append(content, ResponsesContent{
				Type: responsesTextType(message.Role), Text: message.Content.Text,
			})
		}
	} else {
		for _, part := range message.Content.Parts {
			content = append(content, responsesContent(part, message.Role))
		}
	}
	if len(content) == 0 {
		return ResponsesItem{}, false
	}
	return ResponsesItem{Type: ItemMessage, Role: message.Role, Content: content}, true
}

// responsesContent maps one OpenAI content part onto its Responses spelling. A
// part type the Responses API has no field for is carried as its JSON text
// rather than dropped, which is the reference's fallback and keeps the model
// aware of it.
func responsesContent(part schema.ContentPart, role string) ResponsesContent {
	switch part.Type {
	case schema.PartText:
		return ResponsesContent{Type: responsesTextType(role), Text: part.Text}
	case schema.PartImageURL:
		if part.ImageURL == nil {
			return ResponsesContent{Type: responsesTextType(role), Text: ""}
		}
		detail := part.ImageURL.Detail
		if detail == "" {
			detail = "auto"
		}
		return ResponsesContent{Type: ItemInputImage, ImageURL: part.ImageURL.URL, Detail: detail}
	default:
		encoded, err := json.Marshal(part)
		if err != nil {
			return ResponsesContent{Type: responsesTextType(role), Text: part.Text}
		}
		return ResponsesContent{Type: responsesTextType(role), Text: string(encoded)}
	}
}

// responsesTextType names the content-part type a role's text travels as: the
// Responses API splits input from output text by who wrote it.
func responsesTextType(role string) string {
	if role == schema.RoleAssistant {
		return ItemOutputText
	}
	return ItemInputText
}

// responsesCallItems builds one function_call item per tool call. A call with no
// name is skipped: the Responses API rejects a nameless call, and forwarding one
// would fail the whole request (reference #444).
func responsesCallItems(calls []schema.ToolCall) []ResponsesItem {
	items := make([]ResponsesItem, 0, len(calls))
	for _, call := range calls {
		if call.Function.Name == "" {
			continue
		}
		arguments := call.Function.Arguments
		if arguments == "" {
			arguments = "{}"
		}
		items = append(items, ResponsesItem{
			Type:      ItemFunctionCall,
			CallID:    clampResponsesCallID(call.ID),
			Name:      call.Function.Name,
			Arguments: arguments,
		})
	}
	return items
}

// responsesToolOutput renders a tool result as the string the Responses API
// requires, whatever shape the client sent it in. Parts are concatenated with
// nothing between them, which is how the reference joins them: a tool result is
// usually one payload and a separator would corrupt a structured one.
func responsesToolOutput(content schema.MessageContent) string {
	if content.Parts == nil {
		return content.Text
	}
	out := ""
	for _, part := range content.Parts {
		if part.Text != "" {
			out += part.Text
			continue
		}
		if encoded, err := json.Marshal(part); err == nil {
			out += string(encoded)
		}
	}
	return out
}

// responsesTools converts the tool declarations, flattening OpenAI's function
// wrapper and ensuring the schema carries a properties member.
func responsesTools(tools []schema.Tool) []ResponsesTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]ResponsesTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, ResponsesTool{
			Type:        schema.BlockFunction,
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  tool.Function.InputSchema(),
			Strict:      tool.Function.Strict,
		})
	}
	return out
}

// clampResponsesCallID applies the Responses API's call_id length limit.
func clampResponsesCallID(id string) string {
	if len(id) > ResponsesCallIDMaxLen {
		return id[:ResponsesCallIDMaxLen]
	}
	return id
}
