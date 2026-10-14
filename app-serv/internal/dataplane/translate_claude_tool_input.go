// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_claude_tool_input.go
// @for       Rewriting a tool call's arguments onto the parameter names the named Claude tool declares.
// @uses      encoding/json, strconv, strings.
// @reason    A non-Anthropic model answering on the Anthropic wire hallucinates parameter aliases, `cmd` for Bash's `command`, `path` for Read's `file_path`, and Claude Code rejects the call before running it with `InputValidationError: The required parameter 'command' is missing`, which reads to the user as a broken tool rather than a mis-named argument. Renaming the aliases the model did use is what makes the call dispatchable; the tool is not invented and no value is invented with it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"encoding/json"
	"strconv"
	"strings"
)

// claudeLegacyToolPrefix is the name a Claude OAuth proxy hands a built-in tool.
// It is deliberately not ClaudeToolPrefix: that constant is empty here as an
// anti-fingerprint choice, and the two are kept apart so one can change without
// silently renaming the other's tools.
const claudeLegacyToolPrefix = "proxy_"

// claudeReadLimitMax is the largest page count Read will accept; above it the
// reference caps rather than refusing.
const claudeReadLimitMax = 2000

// normalizeClaudeToolInput rewrites one tool_use argument object onto the names
// the named tool declares. An input that is not a JSON object is returned
// exactly as it arrived: this fixes names, it does not judge values, and a call
// this cannot read is a call the client should see unchanged.
func normalizeClaudeToolInput(toolName string, args json.RawMessage) json.RawMessage {
	object, ok := decodeObject(args)
	if !ok {
		return args
	}
	name := claudeBuiltinTool(toolName)
	switch name {
	case "bash", "execute_command":
		adoptArg(object, "command", "cmd", "script", "command_line", "commandLine", "input", "code")
		dropArgIfSet(object, "command", "cmd", "script", "command_line", "commandLine")
	case "read", "edit", "write", "multiedit":
		adoptArg(object, "file_path", "path", "file", "filepath", "filePath", "filename")
		if name == "edit" || name == "multiedit" {
			adoptArg(object, "old_string", "old_str", "old_text", "search")
			adoptArg(object, "new_string", "new_str", "new_text", "replace")
		}
		if name == "write" {
			adoptArg(object, "content", "text", "data", "body")
		}
		if name == "read" {
			normalizeClaudeReadLimits(object)
		}
	case "glob", "grep":
		adoptArg(object, "pattern", "query", "search", "regex", "glob")
		adoptArg(object, "path", "dir", "directory", "folder", "file_path")
	}
	return mustJSON(object)
}

// claudeBuiltinTool names the built-in tool a call targets, lowercased and
// without the OAuth prefix, so a renamed tool is matched by what it is rather
// than by how the proxy spelled it.
func claudeBuiltinTool(toolName string) string {
	return strings.ToLower(strings.TrimPrefix(toolName, claudeLegacyToolPrefix))
}

// adoptArg copies the first alias that carries text onto `target`, and leaves
// `target` alone when the caller already named it, a client that used the real
// parameter is never second-guessed.
func adoptArg(object object, target string, aliases ...string) {
	if _, present := object[target]; present {
		return
	}
	for _, alias := range aliases {
		var value string
		raw, present := object[alias]
		if !present || json.Unmarshal(raw, &value) != nil || value == "" {
			continue
		}
		object[target] = mustJSON(value)
		return
	}
}

// dropArgIfSet retires the aliases once the real parameter exists, so the client
// does not see two spellings of one value.
func dropArgIfSet(object object, target string, aliases ...string) {
	if _, present := object[target]; !present {
		return
	}
	for _, alias := range aliases {
		delete(object, alias)
	}
}

// normalizeClaudeReadLimits puts Read's paging numbers back into the numeric
// shape the tool declares: a digit string is a model spelling a number as text,
// a page count above the cap is clamped, and a zero-page read is dropped so the
// tool reads from the start instead of nothing at all.
func normalizeClaudeReadLimits(object object) {
	if limit, ok := numericArg(object, "limit"); ok {
		switch {
		case limit > claudeReadLimitMax:
			object["limit"] = mustJSON(claudeReadLimitMax)
		case limit < 1:
			delete(object, "limit")
		default:
			object["limit"] = mustJSON(limit)
		}
	}
	if offset, ok := numericArg(object, "offset"); ok {
		if offset < 0 {
			offset = 0
		}
		object["offset"] = mustJSON(offset)
	}
}

// numericArg reads a member as an integer whether it arrived as a number or as a
// digit string, reporting false when it is neither.
func numericArg(object object, name string) (int, bool) {
	raw, present := object[name]
	if !present {
		return 0, false
	}
	var number int
	if json.Unmarshal(raw, &number) == nil {
		return number, true
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return 0, false
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, false
	}
	return parsed, true
}
