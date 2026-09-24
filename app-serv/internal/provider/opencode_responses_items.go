// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_items.go
// @for       The input-item and tool corrections the Responses wire needs before
//
//	a request leaves for the OpenCode upstream.
//
// @uses      encoding/json, strings.
// @reason    The strict Responses backends refuse four shapes outright: a tool
//
//	declaration still wrapped in OpenAI's `function` object, an object
//	schema without a properties map, a call_id past 64 characters, and an
//	`arguments` or `output` that is not a string (#393). Each correction is
//	a rename or a clamp rather than a removal of the client's content,
//	which is the narrow licence this connector has to rewrite a
//	same-format body. The file is split from opencode_responses.go for the
//	AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"encoding/json"
	"strings"
)

// normalizeOpenCodeTools flattens a chat-shaped tool declaration onto the flat
// shape the Responses wire reads, drops a declaration that names no function,
// and fills in an object schema that carries no properties because a strict
// backend refuses one. The reference applies the same rules
// (executors/opencode-go.js:47-80).
func normalizeOpenCodeTools(body map[string]json.RawMessage) {
	tools, ok := decodeOpenCodeItems(body["tools"])
	if !ok {
		return
	}
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
		if len(name) > openCodeMaxToolNameLen {
			name = name[:openCodeMaxToolNameLen]
		}
		flat := map[string]json.RawMessage{
			"type": mustRawString("function"),
			"name": mustRawString(name),
		}
		if description := openCodeToolDescription(tool, nested); description != "" {
			flat["description"] = mustRawString(description)
		}
		flat["parameters"] = openCodeToolParameters(tool, nested)
		kept = append(kept, mustRaw(flat))
	}
	body["tools"] = mustRawList(kept)
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

// normalizeOpenCodeItems corrects the tool-call items in place: a nameless call
// is dropped because the client could not dispatch it, an overlong call_id is
// clamped, and an `arguments` or `output` that is not a string is stringified.
// Each is a shape the strict backend refuses outright (#393).
func normalizeOpenCodeItems(body map[string]json.RawMessage) {
	items, ok := decodeOpenCodeItems(body["input"])
	if !ok {
		return
	}
	kept := make([]json.RawMessage, 0, len(items))
	for _, raw := range items {
		item, ok := decodeOpenCodeBody(raw)
		if !ok {
			kept = append(kept, raw)
			continue
		}
		switch stringMember(item, "type") {
		case "function_call":
			name := strings.TrimSpace(stringMember(item, "name"))
			if name == "" {
				continue
			}
			if len(name) > openCodeMaxToolNameLen {
				name = name[:openCodeMaxToolNameLen]
			}
			item["name"] = mustRawString(name)
			item["call_id"] = mustRawString(clampOpenCodeCallID(stringMember(item, "call_id")))
			item["arguments"] = mustRawString(coerceOpenCodeArguments(item["arguments"]))
		case "function_call_output":
			item["call_id"] = mustRawString(clampOpenCodeCallID(stringMember(item, "call_id")))
			item["output"] = mustRawString(coerceOpenCodeOutput(item["output"]))
		}
		kept = append(kept, mustRaw(item))
	}
	body["input"] = mustRawList(kept)
}

// clampOpenCodeCallID bounds a call id, replacing an absent one with a
// deterministic value so a call and its output still correlate.
func clampOpenCodeCallID(id string) string {
	if id == "" {
		return "call_pannelai"
	}
	if len(id) > openCodeCallIDLimit {
		return id[:openCodeCallIDLimit]
	}
	return id
}

// coerceOpenCodeArguments renders an arguments value as the JSON string the wire
// expects. An object is encoded once, a string that already parses is kept, and
// anything else becomes an empty object rather than a double-encoded fragment.
func coerceOpenCodeArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return "{}"
		}
		var probe json.RawMessage
		if json.Unmarshal([]byte(text), &probe) == nil {
			return text
		}
		return "{}"
	}
	if json.Valid(raw) {
		return string(raw)
	}
	return "{}"
}

// coerceOpenCodeOutput renders a tool result as the string the wire expects: a
// string is kept, an array's text parts are joined, and anything else is encoded
// once.
func coerceOpenCodeOutput(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err == nil {
		var joined strings.Builder
		for _, part := range parts {
			entry, ok := decodeOpenCodeBody(part)
			if !ok {
				joined.WriteString(string(part))
				continue
			}
			if partText := stringMember(entry, "text"); partText != "" {
				joined.WriteString(partText)
				continue
			}
			joined.WriteString(string(part))
		}
		return joined.String()
	}
	if json.Valid(raw) {
		return string(raw)
	}
	return ""
}
