// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_messages.go
// @for       Reading and reshaping the message turns a Qoder request carries.
// @uses      encoding/json, strings.
// @reason    The vendor accepts a plain string for a text turn and an array only when a turn carries an image, and it wants every system turn lifted out of the history into its own field. That is a small set of reads over a shape clients write many ways, so it is kept where a reviewer can check each case.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"encoding/json"
	"strings"
)

// normalizeQoderMessages lifts every system turn out of the history, the way the
// vendor's own client does, and flattens a text-only content array to the string the
// endpoint expects. An array that carries anything else is passed through untouched.
func normalizeQoderMessages(in []qoderMessage) ([]qoderMessage, string) {
	var system []string
	out := make([]qoderMessage, 0, len(in))

	for _, message := range in {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role == "system" {
			if text := qoderContentText(message.Content); text != "" {
				system = append(system, text)
			}
			continue
		}
		if role == "" {
			role = "user"
		}
		out = append(out, qoderMessage{Role: role, Content: qoderFlattenContent(message.Content)})
	}
	return out, strings.Join(system, "\n\n")
}

// qoderFlattenContent renders a content array as a plain string when all of it is
// text, and as itself otherwise: the vendor reads both, and an image turn must keep
// its structure.
func qoderFlattenContent(raw json.RawMessage) json.RawMessage {
	text, isText := qoderContentAsText(raw)
	if isText {
		encoded, err := json.Marshal(text)
		if err == nil {
			return encoded
		}
	}
	return raw
}

// qoderContentAsText reports the single string a content member holds: a JSON string
// as it is, or an array whose every part is a text block joined with newlines.
func qoderContentAsText(raw json.RawMessage) (string, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", true
	}
	if trimmed[0] != '[' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", false
		}
		return text, true
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", false
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != "" && block.Type != "text" {
			return "", false
		}
		parts = append(parts, block.Text)
	}
	joined := strings.Join(parts, "\n")
	if len(blocks) == 0 {
		return "", true
	}
	return joined, true
}

// qoderContentText is the read-only form of the same rule, used to lift system text
// and to name the turn the vendor echoes back.
func qoderContentText(raw json.RawMessage) string {
	if text, ok := qoderContentAsText(raw); ok {
		return text
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func lastQoderUserText(messages []qoderMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		return qoderContentText(messages[i].Content)
	}
	return ""
}

// parseQoderTools keeps the client's tool definitions as objects. A malformed array
// becomes no tools rather than a refused call, because the vendor answers a request
// without tools and the client still gets an answer.
func parseQoderTools(raw json.RawMessage) []qoderTool {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return []qoderTool{}
	}
	var tools []qoderTool
	if err := json.Unmarshal(raw, &tools); err != nil {
		return []qoderTool{}
	}
	return tools
}

// qoderMaxTokens is the vendor's ceiling for the model, lowered by whatever the
// client asked for: the configuration states what the model can write, and a client
// may ask for less of it.
func qoderMaxTokens(config json.RawMessage, incoming qoderOpenAIRequest) int {
	var parsed struct {
		MaxOutputTokens int `json:"max_output_tokens"`
	}
	_ = json.Unmarshal(config, &parsed)

	limit := qoderDefaultMaxTokens
	if parsed.MaxOutputTokens > 0 {
		limit = parsed.MaxOutputTokens
	}
	for _, asked := range []int{incoming.MaxTokens, incoming.MaxCompletionTokens} {
		if asked > 0 && asked < limit {
			limit = asked
		}
	}
	return limit
}

// qoderIsReasoning reads the one flag the vendor's configuration states that the
// gateway also needs to know before the call, because the answer's reasoning parts
// depend on it.
func qoderIsReasoning(config json.RawMessage) bool {
	var parsed struct {
		IsReasoning bool `json:"is_reasoning"`
	}
	_ = json.Unmarshal(config, &parsed)
	return parsed.IsReasoning
}

// qoderModelKey is the vendor's own name for the model. The resolved registry id is
// authoritative; a body that still carries a prefixed name is trimmed to match.
func qoderModelKey(req *Request, fromBody string) string {
	if id := strings.TrimSpace(req.Model.ID); id != "" {
		return id
	}
	name := strings.TrimSpace(fromBody)
	if prefix := req.Provider.ID + "/"; strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return name
}
