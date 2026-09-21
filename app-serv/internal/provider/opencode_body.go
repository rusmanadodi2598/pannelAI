// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_body.go
// @for       The request shape the OpenCode Free tier accepts.
// @uses      encoding/json.
// @reason    The free tier answers 403 unless the outbound body both streams and
//
//	carries the bash and read decoy tools, and the Responses wire answers
//	400 to `max_tokens` and to a `tool_choice` other than `auto`. Those
//	rules are the provider's, so they are applied here rather than by a
//	branch on the provider id in the translation layer. Everything is a
//	set union or a rename over the client's own body: the client's tools,
//	ceiling, and items all survive, because a gateway that dropped them
//	would answer a different question than the one it was asked.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"encoding/json"
	"fmt"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// The decoy tools the free tier requires. It answers 403 unless the payload
// declares both `bash` and `read`, so the connector injects whichever is
// missing. The description is the reference's own text: it tells the model the
// tool is unavailable, which keeps a decoy from being called.
const (
	openCodeDecoyToolDescription = "This tool is currently unavailable and must not be used."
	openCodeDecoyBash            = "bash"
	openCodeDecoyRead            = "read"
)

// openCodeDecoyNames are the tools the gate checks for, in the order they are
// injected so the outbound body is byte-stable across calls.
var openCodeDecoyNames = []string{openCodeDecoyBash, openCodeDecoyRead}

// The Responses item type that carries a prior turn's reasoning. It is dropped
// because the free tier pools anonymous credentials and a reasoning item is
// bound to the caller it was issued to (the reference's sanitizeResponsesItems,
// commit eafac37d).
const openCodeReasoningItem = "reasoning"

// TransformRequest rewrites the outbound body into the shape the free tier
// accepts, and reports the forced stream on the request so the core knows the
// answer will arrive as a stream.
func (c *OpenCode) TransformRequest(req *Request) error {
	body, ok := decodeOpenCodeBody(req.Body)
	if !ok {
		return fmt.Errorf("provider %s: the request body could not be read", c.ID)
	}

	wire := opencodeWire(*req, c.entryFor(*req))
	// The free tier refuses a non-streaming request on either wire, so the body
	// always streams and the core is told to read one.
	body["stream"] = json.RawMessage("true")
	req.Stream = true

	if wire == registry.FormatOpenAIResponses {
		transformOpenCodeResponses(body)
	}
	ensureOpenCodeDecoys(body, wire)

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("provider %s: the request body could not be re-encoded: %w", c.ID, err)
	}
	req.Body = encoded
	return nil
}

// decodeOpenCodeBody decodes the outbound body into a member map, reporting false
// for anything that is not a JSON object: a body the connector cannot read must
// be refused rather than forwarded half rewritten.
func decodeOpenCodeBody(raw []byte) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, false
	}
	return body, true
}

// transformOpenCodeResponses applies the Responses wire's own rules: the output
// ceiling is named max_output_tokens, tool_choice is normalised to auto, and the
// statelessness rules are enforced on the input items.
func transformOpenCodeResponses(body map[string]json.RawMessage) {
	renameOpenCodeCeiling(body)
	body["tool_choice"] = json.RawMessage(`"auto"`)
	dropOpenCodeReasoning(body)
}

// renameOpenCodeCeiling moves a chat-shaped output ceiling onto the Responses
// field name. The Responses wire answers 400 to `max_tokens`, and an already
// present `max_output_tokens` wins because it is the client's own explicit
// choice for this wire.
func renameOpenCodeCeiling(body map[string]json.RawMessage) {
	if _, present := body["max_output_tokens"]; present {
		delete(body, "max_tokens")
		delete(body, "max_completion_tokens")
		return
	}
	for _, key := range []string{"max_completion_tokens", "max_tokens"} {
		if value, present := body[key]; present {
			body["max_output_tokens"] = value
			break
		}
	}
	delete(body, "max_tokens")
	delete(body, "max_completion_tokens")
}

// dropOpenCodeReasoning removes the prior-turn reasoning items and every
// `encrypted_content` member, which is what lets a multi-turn conversation and a
// tool-calling loop survive a pooled credential. The items carrying the
// conversation are kept.
func dropOpenCodeReasoning(body map[string]json.RawMessage) {
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
		if stringMember(item, "type") == openCodeReasoningItem {
			continue
		}
		delete(item, "encrypted_content")
		delete(item, "reasoning_encrypted_content")
		encoded, err := json.Marshal(item)
		if err != nil {
			kept = append(kept, raw)
			continue
		}
		kept = append(kept, encoded)
	}
	body["input"] = mustRawList(kept)
}

// decodeOpenCodeItems reads an input array, reporting false when the member is
// absent or is not an array.
func decodeOpenCodeItems(raw json.RawMessage) ([]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	return items, true
}
