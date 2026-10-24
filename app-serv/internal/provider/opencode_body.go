// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_body.go
// @for       The request shape the OpenCode Free tier accepts.
// @uses      encoding/json.
// @reason    The free tier answers 403 unless the outbound body both streams and carries the read/shell fingerprint tools, and the Responses wire answers 400 to `max_tokens`, to a `tool_choice` other than `auto`, and to an output ceiling below 16. Those rules are the provider's, so they are applied here rather than by a branch on the provider id in the translation layer. Everything is a set union or a rename over the client's own body: the client's tools, ceiling, and items all survive, because a gateway that dropped them would answer a different question than the one it was asked. A ceiling that survives is still raised to the minimum the model declares, because a reasoning model spends a smaller one on thinking and returns an empty body the upstream counts as success.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-21
package provider

import (
	"encoding/json"
	"fmt"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// The Responses item type that carries a prior turn's reasoning. It is dropped
// because the free tier pools anonymous credentials and a reasoning item is
// bound to the caller it was issued to (the reference's own
// sanitizeResponsesItems).
const openCodeReasoningItem = "reasoning"

// TransformRequest rewrites the outbound body into the shape the free tier
// accepts, and reports the forced stream on the request so the core knows the
// answer will arrive as a stream.
func (c *OpenCode) TransformRequest(req *Request) error {
	body, ok := decodeOpenCodeBody(req.Body)
	if !ok {
		return fmt.Errorf("provider %s: the request body could not be read", c.ID)
	}

	entry := c.entryFor(*req)
	wire := opencodeWire(*req, entry)
	// The free tier refuses a non-streaming request on either wire, so the body
	// always streams and the core is told to read one.
	body["stream"] = json.RawMessage("true")
	req.Stream = true

	// The fingerprint pass runs last on either wire: the Responses rules
	// flatten and correct the client's own declarations first, and only then is
	// a caller tool that reuses a fingerprint name concealed and the canonical
	// read/shell declarations appended beside the survivors.
	//
	// The alias map the pass reports travels on the request: the core restores
	// those names in the answer, so a caller that declared its own `read` reads
	// back calls to `read` rather than to the alias it was concealed under.
	if wire == registry.FormatOpenAIResponses {
		transformOpenCodeResponses(body, req.Model, entry)
		req.ToolAliases = applyOpenCodeFingerprintTools(body, true)
	} else {
		clampOpenCodeChatCeiling(body, req.Model)
		req.ToolAliases = applyOpenCodeFingerprintTools(body, false)
	}

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
