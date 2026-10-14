// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_fingerprint.go
// @for       The read/shell tools the OpenCode free tier gates on, and the alias rule for a caller's own tool that already uses one of those names.
// @uses      encoding/json, reflect, strconv, strings.
// @reason    Live upstream probes (OpenCode v2, /inference/v1) show that free-tier requests must declare OpenCode's own `read` and `shell` tools with their exact descriptions and parameter schemas; stubbed descriptions, renamed schemas or tool_choice "none" are answered with 403 FreeTierError. A caller tool that already uses an exact fingerprint name with a different definition travels under an alias instead, so the canonical declaration can be appended without a duplicate name. Everything else is preserved verbatim, including case variants such as `Read`, which upstream accepts alongside `read`. The body-level pass lives in opencode_fingerprint_body.go for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// openCodeFingerprintNames are the tools the gate requires, in the order they
// are appended so the outbound body is byte-stable across calls.
var openCodeFingerprintNames = []string{"read", "shell"}

// openCodeFingerprintDefinition is one canonical declaration. The upstream
// compares description and parameter schema, so both are verbatim from the
// OpenCode v2 CLI (the reference's FINGERPRINT_TOOL_DEFINITIONS); do not reword
// or reformat them.
type openCodeFingerprintDefinition struct {
	description string
	parameters  json.RawMessage
}

var openCodeFingerprintDefinitions = map[string]openCodeFingerprintDefinition{
	"read": {
		description: "Read the contents of a file or directory. Supports text files, images, and PDFs. Images and PDFs are presented directly to the model. Each text line is prefixed by its 1-based line number as <line>: <content>. The prefix is for reference and is not part of the file content. Directory entries are returned one per line. Use offset and limit to read large files or directories in sections. Prefer one larger read over many small slices, and use grep to find specific content in large files.",
		parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"File or directory to read"},` +
			`"offset":{"type":"integer","minimum":0,"description":"The line or directory entry to start reading from (1-based)"},` +
			`"limit":{"type":"integer","minimum":0,"description":"The maximum number of lines or directory entries to read (defaults to and capped at 2000)"}},` +
			`"required":["path"],"additionalProperties":false}`),
	},
	"shell": {
		description: "Execute a shell command and return its output. Commands run on Linux using bash. Quote file paths containing spaces or special characters. Prefer dedicated tools over shell commands when possible. When output is large, the full result is saved to a file and a truncated preview is returned. Rely on automatic truncation unless filtering the output is more useful. Commands accept an optional timeout, background commands have no timeout by default. Background commands return immediately, and you will be notified when they complete.",
		parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"command":{"type":"string","description":"Shell command string to execute"},` +
			`"workdir":{"type":"string","description":"Working directory to execute the command in. Defaults to the current working directory. When possible, avoid changing directories in the command and set the working directory here instead."},` +
			`"timeout":{"type":"integer","minimum":0,"description":"Timeout in milliseconds. Set to 0 to disable the timeout. Defaults to 120000 for foreground commands. Background commands have no timeout by default."},` +
			`"background":{"type":"boolean","description":"Run the command in the background and return immediately (useful for dev servers and long-running builds). You do not need to use '&' at the end of the command when using this parameter. You will be notified when it completes. DO NOT poll for completion."}},` +
			`"required":["command"],"additionalProperties":false}`),
	},
}

// openCodeFingerprintKey reports the fingerprint name when a tool name is
// exactly one of them, and "" otherwise. The match is exact after trimming:
// case variants such as `Read` are caller tools upstream accepts alongside
// `read`, and the pre-v2 bash/grep spellings are not part of this gate.
func openCodeFingerprintKey(name string) string {
	trimmed := strings.TrimSpace(name)
	for _, want := range openCodeFingerprintNames {
		if trimmed == want {
			return want
		}
	}
	return ""
}

// openCodeToolName reads a tool's function name in either wire shape, trimmed,
// as the reference's toolNameOf does: the flat name wins when it has one,
// otherwise the chat-shaped nested function names it.
func openCodeToolName(raw json.RawMessage) string {
	tool, ok := decodeOpenCodeBody(raw)
	if !ok {
		return ""
	}
	if name := strings.TrimSpace(stringMember(tool, "name")); name != "" {
		return name
	}
	function, ok := decodeOpenCodeBody(tool["function"])
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringMember(function, "name"))
}

// openCodeFingerprintToolDefinition reads the declaration a tool carries: the
// nested function for the chat shape, the tool itself for the flat one, which
// is the shape the reference compares against.
func openCodeFingerprintToolDefinition(raw json.RawMessage) (string, json.RawMessage, bool) {
	tool, ok := decodeOpenCodeBody(raw)
	if !ok {
		return "", nil, false
	}
	if function, ok := decodeOpenCodeBody(tool["function"]); ok {
		return stringMember(function, "description"), function["parameters"], true
	}
	return stringMember(tool, "description"), tool["parameters"], true
}

// openCodeIsFingerprintDefinition reports whether a tool already carries the
// canonical definition, in which case it is left exactly as the caller wrote
// it. The comparison is by parsed value: a JSON API compares the decoded
// schema, and a byte comparison is not even self-consistent here because the
// standard marshaller re-escapes `&`, `<` and `>` inside the declaration the
// connector appends.
func openCodeIsFingerprintDefinition(raw json.RawMessage, key string) bool {
	description, parameters, ok := openCodeFingerprintToolDefinition(raw)
	if !ok {
		return false
	}
	want := openCodeFingerprintDefinitions[key]
	return description == want.description && openCodeJSONEqual(parameters, want.parameters)
}

// openCodeJSONEqual compares two JSON values by value rather than by text, so a
// schema a client serialised with a different member order or escaping still
// counts as OpenCode's own.
func openCodeJSONEqual(left, right json.RawMessage) bool {
	var a, b any
	if json.Unmarshal(left, &a) != nil || json.Unmarshal(right, &b) != nil {
		return false
	}
	return reflect.DeepEqual(a, b)
}

// openCodeRenameTool returns the tool with its function name replaced, in
// whichever shape it was written.
func openCodeRenameTool(raw json.RawMessage, name string) json.RawMessage {
	tool, ok := decodeOpenCodeBody(raw)
	if !ok {
		return raw
	}
	if function, ok := decodeOpenCodeBody(tool["function"]); ok {
		function["name"] = mustRawString(name)
		tool["function"] = mustRaw(function)
		return mustRaw(tool)
	}
	tool["name"] = mustRawString(name)
	return mustRaw(tool)
}

// concealOpenCodeFingerprintTools moves a caller tool off a fingerprint name
// when its definition differs from OpenCode's, so the canonical declaration can
// be appended without a duplicate name; the returned map is sent name ->
// fingerprint name. Everything else is preserved verbatim, including case
// variants such as `Read`. A name declared twice is dropped after the first,
// which is the local port's measured rule for any name.
func concealOpenCodeFingerprintTools(tools []json.RawMessage) ([]json.RawMessage, map[string]string) {
	aliases := make(map[string]string)
	taken := make(map[string]bool, len(tools))
	for _, raw := range tools {
		if name := openCodeToolName(raw); name != "" {
			taken[name] = true
		}
	}
	seen := make(map[string]bool, len(tools))
	kept := make([]json.RawMessage, 0, len(tools))
	for _, raw := range tools {
		name := openCodeToolName(raw)
		if name == "" {
			kept = append(kept, raw)
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		key := openCodeFingerprintKey(name)
		if key == "" || openCodeIsFingerprintDefinition(raw, key) {
			kept = append(kept, raw)
			continue
		}
		alias := key + "_client"
		for suffix := 2; taken[alias]; suffix++ {
			alias = key + "_client" + strconv.Itoa(suffix)
		}
		taken[alias] = true
		aliases[alias] = key
		kept = append(kept, openCodeRenameTool(raw, alias))
	}
	return kept, aliases
}

// openCodeFingerprintTool builds one canonical declaration in the wire's own
// shape: the Responses wire names the function on the tool itself, chat nests
// it under `function`.
func openCodeFingerprintTool(name string, responses bool) json.RawMessage {
	definition := openCodeFingerprintDefinitions[name]
	declaration := map[string]json.RawMessage{
		"name":        mustRawString(name),
		"description": mustRawString(definition.description),
		"parameters":  definition.parameters,
	}
	if responses {
		declaration["type"] = mustRawString("function")
		declaration["strict"] = json.RawMessage("false")
		return mustRaw(declaration)
	}
	return mustRaw(map[string]json.RawMessage{
		"type":     mustRawString("function"),
		"function": mustRaw(declaration),
	})
}

// appendOpenCodeFingerprintTools adds the canonical declaration for each
// fingerprint tool the body does not already declare, and reports which were
// appended.
func appendOpenCodeFingerprintTools(tools []json.RawMessage, responses bool) ([]json.RawMessage, []string) {
	appended := make([]string, 0, len(openCodeFingerprintNames))
	for _, name := range openCodeFingerprintNames {
		declared := false
		for _, raw := range tools {
			if openCodeFingerprintKey(openCodeToolName(raw)) == name {
				declared = true
				break
			}
		}
		if declared {
			continue
		}
		tools = append(tools, openCodeFingerprintTool(name, responses))
		appended = append(appended, name)
	}
	return tools, appended
}
