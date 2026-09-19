// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail.go
// @for       The ponytail injector: appending the level's instruction to the
//
//	upstream body's system slot, and the wire-agnostic rules the
//	injectors share.
//
// @uses      encoding/json, strings.
// @reason    SPEC-API-002 §7 makes ponytail a bias rather than a transform: the
//
//	instruction has to reach the model as a system message, and the wire
//	shapes disagree about where that lives. Injecting after translation
//	is what keeps the instruction from being re-translated away. The
//	shared rules live here so both wire files append the same way.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"encoding/json"
	"strings"
)

// The upstream wire shapes the injector understands. The names are the ones the
// data plane's targets carry, and a service-level test pins them end to end.
const (
	WireOpenAI    = "openai"
	WireClaude    = "claude"
	WireResponses = "openai-responses"
)

// ponytailSeparator is the segment separator the idempotency check splits on:
// the instruction is present only when it is its own segment, never when its
// text happens to appear inside someone else's words.
const ponytailSeparator = "\n\n"

// textPart is one content part the injector creates.
type textPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// systemMessage is the chat message the injector creates when a body carries no
// system slot at all.
type systemMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// InjectPonytail appends the level's instruction to the body's system slot,
// leaving every other member as it was. An unknown level, an unknown wire, or a
// body it cannot read leaves the body untouched: the saver is an optimization,
// and a request must never fail because of one.
func InjectPonytail(body []byte, wire, level string) []byte {
	prompt, ok := ponytailPrompts[level]
	if !ok {
		return body
	}
	envelope, ok := decodeObject(body)
	if !ok {
		return body
	}
	var changed bool
	switch wire {
	case WireClaude:
		changed = injectClaudeSystem(envelope, prompt)
	case WireResponses:
		changed = injectResponsesSystem(envelope, prompt)
	case WireOpenAI:
		changed = injectChatSystem(envelope, prompt)
	default:
		// An unknown wire is not guessed at: the caller names the target's
		// format, and guessing here would write a slot the upstream rejects.
		return body
	}
	if !changed {
		return body
	}
	encoded, err := marshalObject(envelope)
	if err != nil {
		return body
	}
	return encoded
}

// replaceMember renders an array into an envelope member, reporting whether it
// was written.
func replaceMember(envelope object, member string, items []json.RawMessage) bool {
	encoded, err := marshalNoEscape(items)
	if err != nil {
		return false
	}
	envelope[member] = encoded
	return true
}

// appendToMessageContent appends the instruction to one message's content,
// reporting whether it changed. Content that is a string grows by a segment;
// content that is a list of parts gains one part. The part type differs per wire,
// which is why the caller names it.
func appendToMessageContent(message object, prompt, partType string) bool {
	if content, ok := message.memberString("content"); ok {
		if hasPromptSegment(content, prompt) {
			return false
		}
		encoded, err := marshalNoEscape(appendPromptSegment(content, prompt))
		if err != nil {
			return false
		}
		message["content"] = encoded
		return true
	}
	parts, ok := decodeArray(message["content"])
	if !ok {
		encoded, err := marshalNoEscape(prompt)
		if err != nil {
			return false
		}
		message["content"] = encoded
		return true
	}
	if containsPromptBlock(parts, prompt) {
		return false
	}
	part, err := marshalNoEscape(textPart{Type: partType, Text: prompt})
	if err != nil {
		return false
	}
	return replaceMember(message, "content", append(parts, part))
}

// isSystemRole reports whether a message carries the system or developer role.
func isSystemRole(message object) bool {
	role, ok := message.memberString("role")
	return ok && (role == "system" || role == "developer")
}

// containsPromptBlock reports whether any part of a content list carries the
// instruction as its own text.
func containsPromptBlock(parts []json.RawMessage, prompt string) bool {
	for _, raw := range parts {
		part, ok := decodeObject(raw)
		if !ok {
			continue
		}
		text, ok := part.memberString("text")
		if ok && hasPromptSegment(text, prompt) {
			return true
		}
	}
	return false
}

// hasPromptSegment reports whether the instruction is already present as its own
// separator-delimited segment, or as the whole text. A substring match would be
// wrong: a user may legitimately quote the instruction back.
func hasPromptSegment(text, prompt string) bool {
	if text == prompt {
		return true
	}
	for _, segment := range strings.Split(text, ponytailSeparator) {
		if segment == prompt {
			return true
		}
	}
	return false
}

// appendPromptSegment appends the instruction as its own segment.
func appendPromptSegment(text, prompt string) string {
	if text == "" {
		return prompt
	}
	return text + ponytailSeparator + prompt
}
