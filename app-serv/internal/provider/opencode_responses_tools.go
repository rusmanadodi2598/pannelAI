// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_tools.go
// @for       The tool declarations the OpenCode Responses wire will accept.
// @uses      encoding/json, strings.
// @reason    /inference/v1/responses reads a tool name as `^[a-zA-Z0-9_-]+$` and answers anything else, an MCP name like `mcp:fs:read` is the common case, with 400 invalid_request_error. Renaming the declaration is only half the job: a `tool_choice` still naming the old spelling points at a tool that no longer exists, which the same backend also refuses, so the two decisions belong to the one pass that knows the surviving names.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package provider

import (
	"encoding/json"
	"strings"
)

// normalizeOpenCodeTools flattens a chat-shaped tool declaration onto the flat
// shape the Responses wire reads, drops a declaration that names no function,
// fills in an object schema that carries no properties because a strict backend
// refuses one, and renames a name the backend cannot spell. The reference applies
// the same rules (executors/opencode.js:310-338).
func normalizeOpenCodeTools(body map[string]json.RawMessage) {
	tools, ok := decodeOpenCodeItems(body["tools"])
	if !ok {
		return
	}
	valid := make(map[string]bool, len(tools))
	kept := make([]json.RawMessage, 0, len(tools))
	for _, raw := range tools {
		tool, ok := decodeOpenCodeBody(raw)
		if !ok {
			continue
		}
		nested, hasNested := decodeOpenCodeBody(tool["function"])
		name := strings.TrimSpace(stringMember(tool, "name"))
		if name == "" && hasNested {
			name = strings.TrimSpace(stringMember(nested, "name"))
		}
		if name == "" {
			continue
		}
		name = sanitizeOpenCodeToolName(name)
		flat := map[string]json.RawMessage{
			"type": mustRawString("function"),
			"name": mustRawString(name),
		}
		if description := openCodeToolDescription(tool, nested); description != "" {
			flat["description"] = mustRawString(description)
		}
		flat["parameters"] = openCodeToolParameters(tool, nested)
		valid[name] = true
		kept = append(kept, mustRaw(flat))
	}
	body["tools"] = mustRawList(kept)
	dropUnknownOpenCodeToolChoice(body, valid)
}

// sanitizeOpenCodeToolName replaces every character outside the wire's alphabet
// with an underscore and then bounds the result to the length the wire accepts.
// The order matters: truncating first would count bytes of a multi-byte name
// against a limit the upstream counts in the characters it has left. After the
// replacement the name is pure ASCII, which is what makes the byte slice below
// correct rather than rune-splitting.
func sanitizeOpenCodeToolName(name string) string {
	cleaned := []rune(name)
	for index, value := range cleaned {
		if !isOpenCodeNameChar(value) {
			cleaned[index] = '_'
		}
	}
	if len(cleaned) > openCodeMaxToolNameLen {
		return string(cleaned[:openCodeMaxToolNameLen])
	}
	return string(cleaned)
}

// isOpenCodeNameChar reports whether a character is one the Responses wire spells
// in a tool name: the reference's [a-zA-Z0-9_-].
func isOpenCodeNameChar(value rune) bool {
	switch {
	case value >= 'a' && value <= 'z',
		value >= 'A' && value <= 'Z',
		value >= '0' && value <= '9',
		value == '_', value == '-':
		return true
	default:
		return false
	}
}

// dropUnknownOpenCodeToolChoice removes a choice that names a tool the request no
// longer declares. It is read from the top-level `name`, which is where the
// reference reads it; a choice that is absent, `auto`, or names a surviving tool
// is left alone.
func dropUnknownOpenCodeToolChoice(body map[string]json.RawMessage, valid map[string]bool) {
	choice, ok := decodeOpenCodeBody(body["tool_choice"])
	if !ok || stringMember(choice, "type") != "function" {
		return
	}
	if name := strings.TrimSpace(stringMember(choice, "name")); name != "" && valid[name] {
		return
	}
	delete(body, "tool_choice")
}

// openCodeToolDescription reads a tool's description from either shape.
func openCodeToolDescription(tool, nested map[string]json.RawMessage) string {
	if description := stringMember(tool, "description"); description != "" {
		return description
	}
	return stringMember(nested, "description")
}

// openCodeToolParameters reads a tool's parameter schema from either shape,
// filling in an empty properties map when the schema declares an object without
// one, because a strict backend refuses `{type:"object"}` alone.
func openCodeToolParameters(tool, nested map[string]json.RawMessage) json.RawMessage {
	raw := tool["parameters"]
	if len(raw) == 0 && nested != nil {
		raw = nested["parameters"]
	}
	parameters, ok := decodeOpenCodeBody(raw)
	if !ok {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	if stringMember(parameters, "type") == "object" {
		if _, present := parameters["properties"]; !present {
			parameters["properties"] = json.RawMessage(`{}`)
		}
	}
	return mustRaw(parameters)
}
