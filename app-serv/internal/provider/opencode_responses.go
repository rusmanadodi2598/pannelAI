// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses.go
// @for       The Responses-wire field rules the connector applies before a
//
//	request leaves for the OpenCode upstream.
//
// @uses      encoding/json, strings, internal/registry.
// @reason    The reference's transformRequest rewrites six members of a
//
//	Responses body that the upstream refuses or reads differently: it
//	demotes tool_choice only for the models its registry lists, pins
//	store to false, folds a chat-shaped reasoning_effort into
//	reasoning:{effort,summary}, replaces an empty input with a placeholder
//	turn, clamps an overlong call_id and stringifies an object arguments,
//	and flattens a chat-shaped tool declaration. Each is a request the
//	upstream would otherwise reject (or answer with a different tool), so
//	the connector owns them rather than forwarding bytes the provider
//	cannot read.
//
//	This is the one place the connector deliberately rewrites a
//	same-format body, which the data plane's lossless rule otherwise
//	leaves alone (engine_translate.go:32-34). The licence is narrow: only
//	members the upstream refuses are touched, and every rewrite is a
//	rename or a clamp rather than a removal of the client's content.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// openCodeMaxToolNameLen bounds a tool name the Responses wire accepts. The
// reference applies the same bound while flattening declarations
// (executors/opencode-go.js:14).
const openCodeMaxToolNameLen = 128

// openCodeCallIDLimit bounds a call_id the strict Responses backends accept;
// an id past it is refused with InputValidationError (#393).
const openCodeCallIDLimit = 64

// transformOpenCodeResponses applies the Responses wire's own rules: the output
// ceiling is named max_output_tokens, the request is stateless, the thinking
// field is the Responses shape, and the input and tools are corrected to the
// shapes the upstream parses.
func transformOpenCodeResponses(body map[string]json.RawMessage, model registry.Model, entry registry.Provider) {
	renameOpenCodeCeiling(body)
	normalizeOpenCodeToolChoice(body, model, entry)
	normalizeOpenCodeReasoning(body)
	normalizeOpenCodeInput(body)
	normalizeOpenCodeTools(body)
	normalizeOpenCodeItems(body)
	dropOpenCodeReasoning(body)
	body["store"] = json.RawMessage("false")
}

// normalizeOpenCodeToolChoice demotes a client's explicit choice to `auto` only
// for the models the entry declares the quirk for, and writes `auto` when the
// client named none, which is the reference's own behaviour for this wire
// (opencodeFingerprint.js:145-147).
//
// Forcing it on every model was the measured delta: a client that asked for a
// specific function on a model without the quirk had its request silently
// changed.
func normalizeOpenCodeToolChoice(body map[string]json.RawMessage, model registry.Model, entry registry.Provider) {
	if !openCodeForcesAutoToolChoice(entry, model) {
		if _, present := body["tool_choice"]; !present {
			body["tool_choice"] = json.RawMessage(`"auto"`)
		}
		return
	}
	body["tool_choice"] = json.RawMessage(`"auto"`)
}

// openCodeForcesAutoToolChoice reports whether the entry lists this model among
// the ones whose tool_choice the wire refuses unless it is `auto`. The list is
// read from the registry rather than hardcoded, so a model that gains the quirk
// upstream is one registry line away (the reference's own reason for declaring
// it there, registry/opencode.js:22-24).
func openCodeForcesAutoToolChoice(entry registry.Provider, model registry.Model) bool {
	for _, declared := range entry.Transport.Quirks.ForceAutoToolChoiceModels {
		if strings.TrimSpace(declared) == model.ID {
			return true
		}
	}
	return false
}

// normalizeOpenCodeReasoning folds a chat-shaped `reasoning_effort` into the
// Responses `reasoning:{effort,summary}` object, which is the field the wire
// reads. An effort the model's own levels do not list is mapped onto the
// strongest level it does, which is the reference's rule
// (executors/opencode.js:373-394).
func normalizeOpenCodeReasoning(body map[string]json.RawMessage) {
	effort := stringMember(body, "reasoning_effort")
	reasoning, _ := decodeOpenCodeBody(body["reasoning"])
	if effort == "" {
		effort = stringMember(reasoning, "effort")
	}
	if effort == "" {
		return
	}
	if reasoning == nil {
		reasoning = map[string]json.RawMessage{}
	}
	reasoning["effort"] = mustRawString(strings.ToLower(strings.TrimSpace(effort)))
	if stringMember(reasoning, "summary") == "" {
		reasoning["summary"] = mustRawString("auto")
	}
	body["reasoning"] = mustRaw(reasoning)
	delete(body, "reasoning_effort")
}

// normalizeOpenCodeInput corrects the `input` union to the item array the wire
// requires: a bare string becomes one user message, and an empty string or an
// empty array becomes one placeholder turn, because an empty `messages[]` is
// refused by every provider (#389).
func normalizeOpenCodeInput(body map[string]json.RawMessage) {
	raw, present := body["input"]
	if !present {
		return
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			text = "..."
		}
		body["input"] = mustRawList([]json.RawMessage{openCodeMessageItem(text)})
		return
	}
	items, ok := decodeOpenCodeItems(raw)
	if !ok {
		return
	}
	if len(items) == 0 {
		body["input"] = mustRawList([]json.RawMessage{openCodeMessageItem("...")})
	}
}

// openCodeMessageItem builds one user message item carrying a single text part,
// which is the shape the Responses wire reads.
func openCodeMessageItem(text string) json.RawMessage {
	return mustRaw(map[string]json.RawMessage{
		"type": mustRawString("message"),
		"role": mustRawString("user"),
		"content": mustRawList([]json.RawMessage{mustRaw(map[string]json.RawMessage{
			"type": mustRawString("input_text"),
			"text": mustRawString(text),
		})}),
	})
}
