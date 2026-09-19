// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_wire.go
// @for       The JSON primitives and finish-reason mapping every translator
//
//	shares, plus the one role name Gemini spells differently.
//
// @uses      encoding/json, strings.
// @reason    SPEC-API-001 §7.15 makes format translation the data plane's core
//
//	job. Every translator reads the same untyped upstream payload, and
//	the finish-reason mapping is mirrored in both directions: a private
//	copy per file is how the two directions start disagreeing. The JSON
//	readers are the declared decode boundary AGENTS.md §1.4 allows, so
//	the type assertions live here rather than at a call site.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"
	"strings"
)

// The roles, block names, and finish reasons the translators emit. One spelling
// each: the OpenAI and Anthropic families disagree about all three, and a private
// copy per file is how two directions start disagreeing.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
	RoleDeveloper = "developer"

	// RoleModel is the assistant role as Gemini spells it
	// (open-sse/translator/schema/roles.js).
	RoleModel = "model"

	// The OpenAI finish_reason set, which is the hub value a client reads.
	FinishStop          = "stop"
	FinishLength        = "length"
	FinishToolCalls     = "tool_calls"
	FinishContentFilter = "content_filter"

	// Claude stop reasons (open-sse/translator/schema/finishReasons.js).
	StopEndTurn      = "end_turn"
	StopMaxTokens    = "max_tokens"
	StopToolUse      = "tool_use"
	StopStopSequence = "stop_sequence"
)

// object is the decode boundary for a JSON object the translators must inspect
// without knowing its schema: an upstream chunk, a tool's argument payload, a
// client's passthrough block. Every field read from it goes through the typed
// helpers below, so no raw assertion chain leaks out of this file.
type object = map[string]json.RawMessage

// decodeObject decodes raw JSON into an object, or reports false.
func decodeObject(raw []byte) (object, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var parsed object
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

// stringField reads a string member, returning "" when absent or mistyped.
func stringField(obj object, key string) string {
	var value string
	if raw, ok := obj[key]; ok {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}

// intField reads an integer member, returning 0 when absent or mistyped.
func intField(obj object, key string) int {
	var value int
	if raw, ok := obj[key]; ok {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}

// objectField reads a nested object member.
func objectField(obj object, key string) (object, bool) {
	raw, ok := obj[key]
	if !ok {
		return nil, false
	}
	return decodeObject(raw)
}

// arrayField reads an array member as its raw elements.
func arrayField(obj object, key string) ([]json.RawMessage, bool) {
	raw, ok := obj[key]
	if !ok {
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	return items, true
}

// mustJSON marshals a value the translators built themselves. A failure is
// impossible for the shapes used (structs, maps of strings, numbers), and is
// reported as a JSON null rather than panicking on the request path.
func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("null")
	}
	return encoded
}

// openAIFinishReason maps a provider-native stop reason onto OpenAI's
// finish_reason, which is the hub value the client reads.
//
// An unknown reason maps to "stop" rather than being passed through: OpenAI
// documents a closed set, and a client switching on it would fall through on a
// value it has never seen.
func openAIFinishReason(reason, format string) string {
	switch format {
	case TargetClaude:
		switch reason {
		case StopEndTurn, StopStopSequence:
			return FinishStop
		case StopMaxTokens:
			return FinishLength
		case StopToolUse:
			return FinishToolCalls
		default:
			return FinishStop
		}
	case formatGemini:
		switch strings.ToUpper(reason) {
		case "STOP":
			return FinishStop
		case "MAX_TOKENS":
			return FinishLength
		case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT":
			return FinishContentFilter
		default:
			return FinishStop
		}
	default:
		return reason
	}
}

// claudeStopReason maps an OpenAI finish_reason onto Anthropic's stop_reason.
func claudeStopReason(reason string) string {
	switch reason {
	case FinishStop:
		return StopEndTurn
	case FinishLength:
		return StopMaxTokens
	case FinishToolCalls:
		return StopToolUse
	case FinishContentFilter:
		return StopStopSequence
	default:
		return StopEndTurn
	}
}

// formatGemini names the Gemini wire format, which has translators for the
// payload but no registry provider that can reach them in P1: the providers
// declaring it (gemini, gemini-cli, antigravity, vertex) wrap their requests in a
// vendor envelope with a project id, so the registry reports them as
// `routability: connector` and the data plane refuses them by name.
const formatGemini = "gemini"

// splitDataURI splits a base64 data URI into its media type and payload, or
// reports false when the value is not one.
//
// The reference tolerates newlines inside the payload
// (`/^data:([^;]+);base64,([\s\S]+)$/`, concerns/image.js) and so does this: a
// client that wraps a long base64 string is still sending a valid image.
func splitDataURI(value string) (mediaType, payload string, ok bool) {
	const prefix = "data:"
	if !strings.HasPrefix(value, prefix) {
		return "", "", false
	}
	marker := strings.Index(value, ";base64,")
	if marker < 0 {
		return "", "", false
	}
	mediaType = value[len(prefix):marker]
	payload = strings.ReplaceAll(value[marker+len(";base64,"):], "\n", "")
	if mediaType == "" || payload == "" {
		return "", "", false
	}
	return mediaType, payload, true
}

// encodeDataURI builds a base64 data URI, the inverse of splitDataURI.
func encodeDataURI(mediaType, payload string) string {
	return "data:" + mediaType + ";base64," + payload
}

// isRemoteURL reports whether a value is an http(s) reference the upstream can
// fetch for itself.
func isRemoteURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
