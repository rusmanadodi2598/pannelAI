// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail_openai.go
// @for       The OpenAI-shaped halves of the ponytail injector: the chat body
//
//	and the Responses body.
//
// @uses      encoding/json.
// @reason    SPEC-API-002 §7 injects into whichever system slot the wire has:
//
//	a chat body carries a system message, and a Responses body carries
//	an instructions string or a system message item.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import "encoding/json"

// injectResponsesSystem writes the instruction into a Responses body: the
// instructions string when it carries one, and otherwise a system message item
// at the head of the input list.
func injectResponsesSystem(envelope object, prompt string) bool {
	if instructions, ok := envelope.memberString("instructions"); ok {
		if hasPromptSegment(instructions, prompt) {
			return false
		}
		encoded, err := marshalNoEscape(appendPromptSegment(instructions, prompt))
		if err != nil {
			return false
		}
		envelope["instructions"] = encoded
		return true
	}
	items, ok := decodeArray(envelope["input"])
	if !ok {
		return false
	}
	for index, raw := range items {
		item, ok := decodeObject(raw)
		if !ok || !hasMemberValue(item, "type", "message") || !isSystemRole(item) {
			continue
		}
		if !appendToMessageContent(item, prompt, "input_text") {
			return false
		}
		encoded, err := marshalObject(item)
		if err != nil {
			return false
		}
		items[index] = encoded
		return replaceMember(envelope, "input", items)
	}
	part, err := marshalNoEscape(textPart{Type: "input_text", Text: prompt})
	if err != nil {
		return false
	}
	content, err := marshalNoEscape([]json.RawMessage{part})
	if err != nil {
		return false
	}
	item, err := marshalNoEscape(object{
		"type":    json.RawMessage(`"message"`),
		"role":    json.RawMessage(`"system"`),
		"content": content,
	})
	if err != nil {
		return false
	}
	return replaceMember(envelope, "input", append([]json.RawMessage{item}, items...))
}

// injectChatSystem writes the instruction into an OpenAI chat body: the first
// system or developer message when there is one, and a new leading system
// message otherwise.
func injectChatSystem(envelope object, prompt string) bool {
	messages, ok := decodeArray(envelope["messages"])
	if !ok {
		return false
	}
	for index, raw := range messages {
		message, ok := decodeObject(raw)
		if !ok || !isSystemRole(message) {
			continue
		}
		if !appendToMessageContent(message, prompt, "text") {
			return false
		}
		encoded, err := marshalObject(message)
		if err != nil {
			return false
		}
		messages[index] = encoded
		return replaceMember(envelope, "messages", messages)
	}
	message, err := marshalNoEscape(systemMessage{Role: "system", Content: prompt})
	if err != nil {
		return false
	}
	return replaceMember(envelope, "messages", append([]json.RawMessage{message}, messages...))
}
