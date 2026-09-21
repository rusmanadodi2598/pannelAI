// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_validation_parts.go
// @for       The nested message and content rules of the OpenAI chat contract.
// @uses      bytes, encoding/json, strconv, strings, internal/domain.
// @reason    The union and closed vocabularies of the message array are a
// separate concern from the request-level cross-field rules, and keeping them
// apart holds both files inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-21
package schema

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// validateMessage checks one turn. An assistant turn carrying tool calls is
// allowed to have empty content: that is the shape a tool-calling conversation
// actually sends, and refusing it would break the flow the route exists to
// serve.
func validateMessage(message ChatMessage, index int) error {
	if message.Content.IsEmpty() {
		if message.Role == RoleAssistant && len(message.ToolCalls) > 0 {
			return nil
		}
		return domain.NewValidationError("messages[" + strconv.Itoa(index) + "].content is required")
	}
	if message.Content.Parts == nil {
		return nil
	}
	for partIndex, part := range message.Content.Parts {
		if err := validateContentPart(part, index, partIndex); err != nil {
			return err
		}
	}
	return nil
}

// validateContentPart checks one multimodal entry against its declared type, so
// a part whose payload does not match its discriminator is refused rather than
// translated into an empty block.
func validateContentPart(part ContentPart, messageIndex, partIndex int) error {
	prefix := "messages[" + strconv.Itoa(messageIndex) + "].content[" + strconv.Itoa(partIndex) + "]"
	switch part.Type {
	case PartText:
		if part.Text == "" {
			return domain.NewValidationError(prefix + ".text is required")
		}
	case PartImageURL:
		if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
			return domain.NewValidationError(prefix + ".image_url.url is required")
		}
		if part.ImageURL.Detail != "" && !validImageDetail(part.ImageURL.Detail) {
			return domain.NewValidationError(prefix + ".image_url.detail has an unexpected value")
		}
	case PartInputAudio:
		if !isJSONObject(part.Audio) {
			return domain.NewValidationError(prefix + ".input_audio must be a JSON object")
		}
	case PartFile:
		if !isJSONObject(part.File) {
			return domain.NewValidationError(prefix + ".file must be a JSON object")
		}
	default:
		return domain.NewValidationError(prefix + ".type has an unexpected value")
	}
	return nil
}

// isJSONObject reports whether raw holds a JSON object. An absent member, a
// scalar, or a malformed value all fail: a part must carry the payload its type
// promises.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(trimmed, &object) == nil
}

// validImageDetail reports whether detail is one of the documented values.
func validImageDetail(detail string) bool {
	switch detail {
	case "auto", "low", "high":
		return true
	default:
		return false
	}
}

// validReasoningEffort reports whether effort is one of the documented values.
// The set is OpenAI's own rather than a guess: the field is forwarded verbatim
// on the same-format path, so an unknown value would reach the provider as-is.
func validReasoningEffort(effort string) bool {
	switch effort {
	case "minimal", "low", "medium", "high":
		return true
	default:
		return false
	}
}
