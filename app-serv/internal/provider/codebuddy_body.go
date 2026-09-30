// The CodeBuddy request shape: a leading system turn of its own, typed user
// content, and reasoning reported the way this service reads it.
//
// @file      internal/provider/codebuddy_body.go
// @for       Rewrites an outbound CodeBuddy chat body into the shape the vendor accepts.
// @uses      encoding/json, fmt, strings.
// @reason    The vendor answers a plain OpenAI body with `11101 invalid request`; it wants a
//
//	system turn leading the conversation and user content as typed blocks rather than a bare
//	string (the reference's executors/codebuddy-intl.js:20-38). The caller's own system text
//	is folded into that one leading turn instead of dropped, so a client that steers the
//	model through a system prompt keeps its instruction and the request still has exactly one
//	system message, which is what the live vendor was measured accepting on 2026-09-29.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package provider

import (
	"encoding/json"
	"fmt"
	"strings"
)

// codeBuddySystemPrompt is the turn this service requires first, in the vendor's own
// words as the reference sends them.
const codeBuddySystemPrompt = "You are CodeBuddy Code."

// codeBuddySilencedReasoning are the answers that mean "do not think", which this
// vendor reads as an invalid request rather than as an absence: the reference deletes
// the field instead of forwarding it.
var codeBuddySilencedReasoning = map[string]bool{"none": true, "off": true}

// codeBuddyTurn is one message as it leaves this connector: a role and content whose
// concrete shape depends on the turn, plus whatever extra fields the caller sent
// (a tool turn's call id, an assistant turn's tool calls) which are carried verbatim
// rather than re-declared here.
type codeBuddyTurn struct {
	role    string
	content json.RawMessage
	extra   map[string]json.RawMessage
}

// TransformRequest rewrites the translated body into the shape CodeBuddy accepts. It is
// the connector's second reason to exist beside the forced stream: the URL, headers and
// credential are the plain OpenAI wire the fallback already serves, but the message list
// is not.
func (c *CodeBuddy) TransformRequest(req *Request) error {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return fmt.Errorf("provider %s: the request body is not a JSON object", c.ID)
	}

	messages, err := codeBuddyMessages(body)
	if err != nil {
		return fmt.Errorf("provider %s: %w", c.ID, err)
	}
	body["messages"] = messages

	if err := mirrorCodeBuddyReasoning(body); err != nil {
		return fmt.Errorf("provider %s: %w", c.ID, err)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("provider %s: the shaped body could not be encoded: %w", c.ID, err)
	}
	req.Body = encoded
	return nil
}

// codeBuddyMessages rebuilds the message list: one leading system turn carrying the
// vendor's prompt followed by every system/developer instruction the caller sent, then
// the remaining turns in their original order with a string user content lifted into
// typed blocks. Content the caller already sent as blocks — the image parts the vision
// path builds — is left exactly as it arrived.
func codeBuddyMessages(body map[string]json.RawMessage) (json.RawMessage, error) {
	raw, present := body["messages"]
	if !present {
		return json.Marshal([]codeBuddyTurn{{role: "system", content: codeBuddyString(codeBuddySystemPrompt)}})
	}
	var source []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, fmt.Errorf("messages must be a list of message objects")
	}

	var instructions []string
	kept := make([]codeBuddyTurn, 0, len(source))
	for _, message := range source {
		role, _ := jsonString(message["role"])
		content := message["content"]
		if role == "system" || role == "developer" {
			if text := codeBuddyText(content); text != "" {
				instructions = append(instructions, text)
			}
			continue
		}
		if text, isString := jsonString(content); role == "user" && isString {
			content = codeBuddyTextBlocks(text)
		}
		kept = append(kept, codeBuddyTurn{role: role, content: content, extra: withoutRoleAndContent(message)})
	}

	leading := codeBuddySystemPrompt
	if len(instructions) > 0 {
		leading += "\n\n" + strings.Join(instructions, "\n\n")
	}
	ordered := append([]codeBuddyTurn{{role: "system", content: codeBuddyString(leading)}}, kept...)
	return json.Marshal(ordered)
}

// MarshalJSON writes the turn's declared fields and then every field the caller sent
// that this connector does not own, so nothing a client asked for is quietly dropped.
func (t codeBuddyTurn) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(t.extra)+2)
	for name, value := range t.extra {
		fields[name] = value
	}
	fields["role"] = codeBuddyString(t.role)
	fields["content"] = t.content
	return json.Marshal(fields)
}

// withoutRoleAndContent is everything a caller's message carried besides the two fields
// this connector decides.
func withoutRoleAndContent(message map[string]json.RawMessage) map[string]json.RawMessage {
	fields := make(map[string]json.RawMessage, len(message))
	for name, value := range message {
		if name == "role" || name == "content" {
			continue
		}
		fields[name] = value
	}
	return fields
}

// codeBuddyString encodes a value as a JSON string, or as the empty string when even
// that fails, because a string cannot fail to encode and a silently missing field would
// be worse than an empty one.
func codeBuddyString(value string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}

// codeBuddyText reads the plain text a message carries, whether its content is a string
// or a list of blocks, so a caller's instruction survives either shape.
func codeBuddyText(raw json.RawMessage) string {
	if text, ok := jsonString(raw); ok {
		return text
	}
	var blocks []codeBuddyTextBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// codeBuddyTextBlock is one typed content block, the shape this vendor wants a user
// turn in.
type codeBuddyTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// codeBuddyTextBlocks encodes a string content as the single text block the vendor
// expects, with the text escaped by the encoder rather than by hand.
func codeBuddyTextBlocks(text string) json.RawMessage {
	encoded, err := json.Marshal([]codeBuddyTextBlock{{Type: "text", Text: text}})
	if err != nil {
		// A string cannot fail to encode; an empty list is not a value a caller could
		// mistake for their own question.
		return json.RawMessage(`[]`)
	}
	return encoded
}

// mirrorCodeBuddyReasoning reports reasoning the way this vendor reads the OpenAI
// params: a stated effort is mirrored by `reasoning_summary: "auto"`, and the answers
// that mean "do not think" are removed rather than sent.
func mirrorCodeBuddyReasoning(body map[string]json.RawMessage) error {
	raw, present := body["reasoning_effort"]
	if !present {
		return nil
	}
	effort, ok := jsonString(raw)
	if !ok {
		return fmt.Errorf("reasoning_effort must be a string")
	}
	if effort == "" || codeBuddySilencedReasoning[effort] {
		delete(body, "reasoning_effort")
		return nil
	}
	body["reasoning_summary"] = json.RawMessage(`"auto"`)
	return nil
}

// jsonString reads a raw value as a string, reporting false for anything else.
func jsonString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}
