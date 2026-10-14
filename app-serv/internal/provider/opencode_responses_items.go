// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_items.go
// @for       The input-item corrections the Responses wire needs before a request leaves for the OpenCode upstream.
// @uses      encoding/json, strings, internal/provider/opencode_responses_pairing.go.
// @reason    The strict Responses backends refuse four shapes outright: a call_id past 64 characters, a name the wire cannot spell, an `arguments` or `output` that is not a string, and an open tool call no one answered (#393, and the compacted-turn 400 the pairing fixes). Each correction is a rename or a clamp rather than a removal of the client's content, which is the narrow licence this connector has to rewrite a same-format body. Corrections complete before pairing because the upstream pairs on the values it receives, not on what the client meant.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-24
package provider

import (
	"encoding/json"
	"strings"
)

// normalizeOpenCodeItems corrects the tool-call items in place: a nameless call
// is dropped because the client could not dispatch it, an overlong call_id is
// clamped, an `arguments` or `output` that is not a string is stringified, and
// the surviving calls are then paired with their outputs.
func normalizeOpenCodeItems(body map[string]json.RawMessage) {
	items, ok := decodeOpenCodeItems(body["input"])
	if !ok {
		return
	}
	corrected := make([]openCodeItem, 0, len(items))
	for _, raw := range items {
		item, ok := decodeOpenCodeBody(raw)
		if !ok {
			corrected = append(corrected, openCodeItem{raw: raw})
			continue
		}
		switch stringMember(item, "type") {
		case "function_call":
			name := strings.TrimSpace(stringMember(item, "name"))
			if name == "" {
				continue
			}
			callID := clampOpenCodeCallID(stringMember(item, "call_id"))
			item["name"] = mustRawString(sanitizeOpenCodeToolName(name))
			item["call_id"] = mustRawString(callID)
			item["arguments"] = mustRawString(coerceOpenCodeArguments(item["arguments"]))
			corrected = append(corrected, openCodeItem{
				raw: mustRaw(item), kind: openCodeItemCall, callID: callID,
			})
		case "function_call_output":
			callID := clampOpenCodeCallID(stringMember(item, "call_id"))
			item["call_id"] = mustRawString(callID)
			item["output"] = mustRawString(coerceOpenCodeOutput(item["output"]))
			corrected = append(corrected, openCodeItem{
				raw: mustRaw(item), kind: openCodeItemCallOutput, callID: callID,
			})
		default:
			corrected = append(corrected, openCodeItem{raw: mustRaw(item)})
		}
	}
	body["input"] = mustRawList(pairOpenCodeItems(corrected))
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
