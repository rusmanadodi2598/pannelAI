// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_gemini_parts.go
// @for       The part, image, tool, and function-result conversion the
//
//	OpenAI-to-Gemini direction needs, plus Gemini's name rules.
//
// @uses      internal/schema, encoding/json, strings.
// @reason    Gemini's part shapes are the bulk of the direction's work and none of
//
//	them is reachable from the request builder without them: a tool
//	result has to be paired by call id with the name its functionCall
//	declared, and a name has to satisfy Gemini's character rules. Keeping
//	these beside the request translation is what makes the pairing
//	auditable in one place.
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

// geminiMaxFunctionNameLen is Gemini's function-name ceiling. Gemini also
// requires a name to start with a letter or underscore and to contain only
// [a-zA-Z0-9_.:-]; the reference sanitizes rather than rejecting
// (openai-to-gemini.js, sanitizeGeminiFunctionName), because the name is the
// client's, not the gateway's.
const geminiMaxFunctionNameLen = 64

// geminiTurn converts one non-system message into a Gemini turn, returning false
// when the message produced nothing to send.
func geminiTurn(message schema.ChatMessage, calls map[string]string) (GeminiContent, bool) {
	switch message.Role {
	case schema.RoleAssistant:
		parts := make([]GeminiPart, 0, 2+len(message.ToolCalls))
		if text := message.Content.TextContent(); text != "" {
			parts = append(parts, GeminiPart{Text: text})
		}
		for _, call := range message.ToolCalls {
			if call.Function.Name == "" {
				continue
			}
			parts = append(parts, GeminiPart{FunctionCall: &GeminiFunctionCall{
				ID:   call.ID,
				Name: sanitizeGeminiFunctionName(call.Function.Name),
				Args: argumentMap(call.Function.Arguments),
			}})
		}
		if len(parts) == 0 {
			return GeminiContent{}, false
		}
		return GeminiContent{Role: RoleModel, Parts: parts}, true
	case schema.RoleTool:
		part, ok := geminiFunctionResponse(message.ToolCallID, message.Content.TextContent(), calls)
		if !ok {
			return GeminiContent{}, false
		}
		return GeminiContent{Role: RoleUser, Parts: []GeminiPart{part}}, true
	default:
		parts := geminiParts(message.Content)
		if len(parts) == 0 {
			return GeminiContent{}, false
		}
		return GeminiContent{Role: RoleUser, Parts: parts}, true
	}
}

// geminiParts converts an OpenAI user body into Gemini parts: text, inline base64
// images, and remote image references.
func geminiParts(content schema.MessageContent) []GeminiPart {
	if content.Parts == nil {
		if content.Text == "" {
			return nil
		}
		return []GeminiPart{{Text: content.Text}}
	}
	parts := make([]GeminiPart, 0, len(content.Parts))
	for _, part := range content.Parts {
		switch {
		case part.Type == schema.PartText && part.Text != "":
			parts = append(parts, GeminiPart{Text: part.Text})
		case part.Type == schema.PartImageURL && part.ImageURL != nil:
			if converted, ok := geminiImage(part.ImageURL.URL); ok {
				parts = append(parts, converted)
			}
		}
	}
	return parts
}

// geminiImage converts an image reference: a data URI becomes inlineData, and an
// http(s) URL becomes fileData, which is Gemini's remote-reference shape.
func geminiImage(reference string) (GeminiPart, bool) {
	if mediaType, payload, ok := splitDataURI(reference); ok {
		return GeminiPart{InlineData: &GeminiInlineData{MIMEType: mediaType, Data: payload}}, true
	}
	if isRemoteURL(reference) {
		return GeminiPart{FileData: &GeminiFileData{FileURI: reference, MIMEType: "image/*"}}, true
	}
	return GeminiPart{}, false
}

// geminiFunctionResponse builds the functionResponse part for one tool call, using
// the name the matching functionCall declared so the upstream can pair them. A
// response that is not an object is wrapped under `result`, because Gemini's
// response field is an object.
func geminiFunctionResponse(callID, body string, calls map[string]string) (GeminiPart, bool) {
	if callID == "" {
		return GeminiPart{}, false
	}
	result := argumentMap(body)
	if _, ok := result["result"]; !ok {
		result = map[string]any{"result": argumentValue(body)}
	}
	return GeminiPart{FunctionResponse: &GeminiFunctionResp{
		ID:       callID,
		Name:     sanitizeGeminiFunctionName(calls[callID]),
		Response: result,
	}}, true
}

// geminiCallNames maps every tool-call id in the conversation to the name its
// functionCall declared, which is what a functionResponse has to repeat.
func geminiCallNames(messages []schema.ChatMessage) map[string]string {
	names := make(map[string]string, 4)
	for _, message := range messages {
		for _, call := range message.ToolCalls {
			if call.ID != "" && call.Function.Name != "" {
				names[call.ID] = call.Function.Name
			}
		}
	}
	return names
}

// geminiDeclarations converts OpenAI tool declarations into Gemini ones, applying
// Gemini's name rules so the upstream accepts them instead of rejecting the whole
// request.
func geminiDeclarations(tools []schema.Tool) []GeminiFunctionDecl {
	out := make([]GeminiFunctionDecl, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != schema.BlockFunction || tool.Function.Name == "" {
			continue
		}
		out = append(out, GeminiFunctionDecl{
			Name:        sanitizeGeminiFunctionName(tool.Function.Name),
			Description: tool.Function.Description,
			Parameters:  tool.Function.InputSchema(),
		})
	}
	return out
}

// sanitizeGeminiFunctionName applies Gemini's rules: the first character must be a
// letter or underscore, the rest may be [a-zA-Z0-9_.:-], and the whole name is at
// most 64 characters. An invalid character becomes an underscore, which keeps the
// request servable where rejecting it would fail a call over a decoration.
func sanitizeGeminiFunctionName(name string) string {
	if name == "" {
		return "_unknown"
	}
	var out strings.Builder
	out.Grow(len(name))
	for i := 0; i < len(name) && out.Len() < geminiMaxFunctionNameLen; i++ {
		ch := name[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z':
			out.WriteByte(ch)
		case ch >= '0' && ch <= '9':
			if i == 0 {
				out.WriteByte('_')
			}
			out.WriteByte(ch)
		case ch == '_' || ch == '.' || ch == ':' || ch == '-':
			if i == 0 && ch != '_' {
				out.WriteByte('_')
			}
			out.WriteByte(ch)
		default:
			out.WriteByte('_')
		}
	}
	if out.Len() == 0 {
		return "_unknown"
	}
	return out.String()
}

// argumentMap decodes a tool argument string into a map. An unparseable or
// non-object payload becomes an empty map, because a vector argument would be
// rejected where an empty object is merely empty.
func argumentMap(arguments string) map[string]any {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return map[string]any{}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil || parsed == nil {
		return map[string]any{}
	}
	return parsed
}

// argumentValue decodes a tool result body into the value a functionResponse
// carries: an object or array stays as it is, and a scalar or unparseable string is
// kept verbatim so no result is silently lost.
func argumentValue(body string) any {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return body
	}
	return parsed
}
