// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail_claude.go
// @for       The Anthropic half of the ponytail injector.
// @uses      encoding/json.
// @reason    SPEC-API-002 §7 keeps the Anthropic system member out of the
//
//	message list: the wire rejects a system role inside messages, so the
//	instruction is appended to the dedicated member, before the last
//	cache-control block so the cached prefix stays intact.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import "encoding/json"

// injectClaudeSystem writes the instruction into the Anthropic top-level system
// member, which is where an Anthropic upstream accepts one: a system role inside
// messages is rejected by the wire.
func injectClaudeSystem(envelope object, prompt string) bool {
	raw, present := envelope["system"]
	if !present || string(raw) == "null" {
		encoded, err := marshalNoEscape(prompt)
		if err != nil {
			return false
		}
		envelope["system"] = encoded
		return true
	}
	if system, ok := envelope.memberString("system"); ok {
		if hasPromptSegment(system, prompt) {
			return false
		}
		encoded, err := marshalNoEscape(appendPromptSegment(system, prompt))
		if err != nil {
			return false
		}
		envelope["system"] = encoded
		return true
	}
	blocks, ok := decodeArray(raw)
	if !ok {
		return false
	}
	if containsPromptBlock(blocks, prompt) {
		return false
	}
	block, err := marshalNoEscape(textPart{Type: "text", Text: prompt})
	if err != nil {
		return false
	}
	// The instruction is inserted before the last cache-control block, which is
	// where the reference puts it: the instruction joins the cached prefix
	// instead of riding outside the cache and being paid for on every request.
	// A body with no cached block gets it appended.
	at := len(blocks)
	for index := len(blocks) - 1; index >= 0; index-- {
		part, ok := decodeObject(blocks[index])
		if !ok {
			continue
		}
		if _, cached := part["cache_control"]; cached {
			at = index
			break
		}
	}
	next := make([]json.RawMessage, 0, len(blocks)+1)
	next = append(next, blocks[:at]...)
	next = append(next, block)
	next = append(next, blocks[at:]...)
	encoded, err := marshalNoEscape(next)
	if err != nil {
		return false
	}
	envelope["system"] = encoded
	return true
}
