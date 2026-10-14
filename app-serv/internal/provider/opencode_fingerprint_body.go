// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_fingerprint_body.go
// @for       The body-level half of the OpenCode fingerprint pass: the choice retarget, the history rename, the placeholder note, and the pass that runs them.
// @uses      encoding/json, strings.
// @reason    The request body is a member map the transform owns, so these rules read and write members directly rather than through a typed shape: an explicit tool_choice that names a concealed caller tool has to follow it to its alias, a prior-turn call to that tool has to be renamed with it or the model reads its own history as calls to OpenCode's tool of the same name, and the model has to be told the appended declarations are placeholders or it calls them (measured live). They sit apart from opencode_fingerprint.go, which owns the tool-level rules, for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"encoding/json"
	"strings"
)

// openCodeAliasesByOriginal inverts the conceal map: fingerprint name -> alias.
func openCodeAliasesByOriginal(aliases map[string]string) map[string]string {
	inverted := make(map[string]string, len(aliases))
	for alias, original := range aliases {
		inverted[original] = alias
	}
	return inverted
}

// retargetOpenCodeToolChoice points an explicit choice at the alias of a
// concealed caller tool, so it still names a tool the request declares.
func retargetOpenCodeToolChoice(body map[string]json.RawMessage, aliases map[string]string) {
	if len(aliases) == 0 {
		return
	}
	byOriginal := openCodeAliasesByOriginal(aliases)
	choice, ok := decodeOpenCodeBody(body["tool_choice"])
	if !ok {
		return
	}
	if name, isString := openCodeStringMember(choice, "name"); isString {
		if alias, found := byOriginal[strings.TrimSpace(name)]; found {
			choice["name"] = mustRawString(alias)
			body["tool_choice"] = mustRaw(choice)
		}
		return
	}
	function, ok := decodeOpenCodeBody(choice["function"])
	if !ok {
		return
	}
	if name, isString := openCodeStringMember(function, "name"); isString {
		if alias, found := byOriginal[strings.TrimSpace(name)]; found {
			function["name"] = mustRawString(alias)
			choice["function"] = mustRaw(function)
			body["tool_choice"] = mustRaw(choice)
		}
	}
}

// aliasOpenCodeHistoryToolCalls renames prior-turn calls to a concealed caller
// tool as well; otherwise the model reads its own earlier calls as calls to
// OpenCode's tool of the same name.
func aliasOpenCodeHistoryToolCalls(body map[string]json.RawMessage, aliases map[string]string) {
	if len(aliases) == 0 {
		return
	}
	byOriginal := openCodeAliasesByOriginal(aliases)

	if items, ok := decodeOpenCodeItems(body["input"]); ok {
		changed := false
		for index, raw := range items {
			item, ok := decodeOpenCodeBody(raw)
			if !ok || stringMember(item, "type") != "function_call" {
				continue
			}
			if alias, found := byOriginal[stringMember(item, "name")]; found {
				item["name"] = mustRawString(alias)
				items[index] = mustRaw(item)
				changed = true
			}
		}
		if changed {
			body["input"] = mustRawList(items)
		}
	}

	if messages, ok := decodeOpenCodeItems(body["messages"]); ok {
		changed := false
		for index, raw := range messages {
			message, ok := decodeOpenCodeBody(raw)
			if !ok {
				continue
			}
			calls, ok := decodeOpenCodeItems(message["tool_calls"])
			if !ok {
				continue
			}
			callsChanged := false
			for callIndex, callRaw := range calls {
				call, ok := decodeOpenCodeBody(callRaw)
				if !ok {
					continue
				}
				function, ok := decodeOpenCodeBody(call["function"])
				if !ok {
					continue
				}
				if alias, found := byOriginal[stringMember(function, "name")]; found {
					function["name"] = mustRawString(alias)
					call["function"] = mustRaw(function)
					calls[callIndex] = mustRaw(call)
					callsChanged = true
				}
			}
			if callsChanged {
				message["tool_calls"] = mustRawList(calls)
				messages[index] = mustRaw(message)
				changed = true
			}
		}
		if changed {
			body["messages"] = mustRawList(messages)
		}
	}
}

// openCodePlaceholderNote writes the sentence that keeps the model off the
// appended tools. The wording is the reference's: "not available" made the
// model also refuse the caller's own tools, "placeholders ... use the other
// tools" kept those working.
func openCodePlaceholderNote(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, "`"+name+"`")
	}
	list := strings.Join(quoted, " or ")
	if len(names) > 1 {
		return "Do not call the " + list + " tools; they are placeholders that do nothing. Use the other tools you have been given instead."
	}
	return "Do not call the " + list + " tool; it is a placeholder that does nothing. Use the other tools you have been given instead."
}

// noteOpenCodePlaceholderTools tells the model the appended fingerprint tools
// are placeholders. The upstream does not gate on the prompt text.
func noteOpenCodePlaceholderTools(body map[string]json.RawMessage, responses bool, names []string) {
	note := openCodePlaceholderNote(names)
	if responses {
		if instructions := stringMember(body, "instructions"); strings.TrimSpace(instructions) != "" {
			body["instructions"] = mustRawString(instructions + "\n\n" + note)
			return
		}
		body["instructions"] = mustRawString(note)
		return
	}
	// OpenAI chat shape only; Claude-shaped bodies carry `system` instead.
	if _, present := body["system"]; present {
		return
	}
	messages, ok := decodeOpenCodeItems(body["messages"])
	if !ok {
		return
	}
	if len(messages) > 0 {
		first, ok := decodeOpenCodeBody(messages[0])
		if ok && stringMember(first, "role") == "system" {
			if content, isString := openCodeStringMember(first, "content"); isString {
				first["content"] = mustRawString(content + "\n\n" + note)
				messages[0] = mustRaw(first)
				body["messages"] = mustRawList(messages)
				return
			}
		}
	}
	placeholder := mustRaw(map[string]json.RawMessage{
		"role":    mustRawString("system"),
		"content": mustRawString(note),
	})
	body["messages"] = mustRawList(append([]json.RawMessage{placeholder}, messages...))
}

// applyOpenCodeFingerprintTools runs the full request-side pass: alias a caller
// tool that reuses a fingerprint name with a different definition, append
// OpenCode's own read/shell declarations, point a choice at the alias, rename
// prior calls, and tell the model the appended tools are placeholders.
//
// responses selects the wire's tool vocabulary. The Responses wire keeps its
// explicit auto default; neither wire ever defaults to "none", which the
// upstream answers with 403 FreeTierError.
func applyOpenCodeFingerprintTools(body map[string]json.RawMessage, responses bool) {
	tools, _ := decodeOpenCodeItems(body["tools"])
	concealed, aliases := concealOpenCodeFingerprintTools(tools)
	tools, appended := appendOpenCodeFingerprintTools(concealed, responses)
	body["tools"] = mustRawList(tools)
	retargetOpenCodeToolChoice(body, aliases)
	aliasOpenCodeHistoryToolCalls(body, aliases)
	if len(appended) > 0 {
		noteOpenCodePlaceholderTools(body, responses, appended)
	}
	if _, present := body["tool_choice"]; !present && responses {
		body["tool_choice"] = json.RawMessage(`"auto"`)
	}
}
