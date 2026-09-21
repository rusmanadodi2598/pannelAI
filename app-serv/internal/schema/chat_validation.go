// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_validation.go
// @for       The semantic rules of the OpenAI chat contract: the nested content
//
//	union, the stop shapes, and the cross-field response format.
//
// @uses      bytes, encoding/json, strconv, strings, internal/domain.
// @reason    F1 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md found
//
//	struct tags alone accept shapes the translators cannot carry, so the
//	rules the wire implies live beside the DTO instead of in the handler
//	(AGENTS.md §1.4: no raw input reaches a translator unvalidated).
//	The closed sets are the protocol's own: an unknown role, part, or
//	detail is refused rather than silently dropped downstream.
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

const (
	// maxStopSequences bounds the array spelling of the OpenAI `stop` field.
	maxStopSequences = 4
	stopShapeMessage = "stop must be a non-empty string or an array of up to four strings"
)

// ValidateChatRequest applies the nested and cross-field rules a struct tag
// cannot express: the content union, the closed vocabularies, and the pairs
// that must agree. It runs after ValidateStruct, so a field the tags already
// refused never reaches here.
func ValidateChatRequest(req ChatRequest) error {
	for index, message := range req.Messages {
		if !validChatRole(message.Role) {
			return domain.NewValidationError("messages[" + strconv.Itoa(index) + "].role has an unexpected value")
		}
		if err := validateMessage(message, index); err != nil {
			return err
		}
	}
	if req.MaxTokens != nil && req.MaxCompletionTokens != nil {
		return domain.NewValidationError("max_tokens and max_completion_tokens cannot both be set")
	}
	if err := validateStop(req.Stop); err != nil {
		return err
	}
	if err := validateToolChoice(req.ToolChoice); err != nil {
		return err
	}
	if err := validateTools(req.Tools); err != nil {
		return err
	}
	if err := validateResponseFormat(req.ResponseFormat); err != nil {
		return err
	}
	if req.ReasoningEffort != "" && !validReasoningEffort(req.ReasoningEffort) {
		return domain.NewValidationError("reasoning_effort has an unexpected value")
	}
	return nil
}

// validChatRole reports whether role is one the OpenAI wire defines. The set is
// closed so an arbitrary value is refused here rather than forwarded to a
// provider that answers with an opaque 400.
func validChatRole(role string) bool {
	switch role {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool, RoleDeveloper:
		return true
	default:
		return false
	}
}

// validateMessage and the content-union rules live in chat_validation_parts.go
// so request-level cross-field checks remain independently auditable.

// validateStop checks the two permitted shapes: one non-empty string, or a
// non-empty array of at most four non-empty strings.
func validateStop(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '"' {
		var single string
		if err := json.Unmarshal(trimmed, &single); err != nil || single == "" {
			return domain.NewValidationError(stopShapeMessage)
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(trimmed, &many); err != nil || len(many) == 0 || len(many) > maxStopSequences {
		return domain.NewValidationError(stopShapeMessage)
	}
	for _, value := range many {
		if value == "" {
			return domain.NewValidationError("stop values must not be empty")
		}
	}
	return nil
}

// validateToolChoice refuses a control the translator cannot map. It parses
// through the same function the translators use, so the accepted vocabulary is
// decided once.
func validateToolChoice(raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if _, err := ParseToolChoice(raw); err != nil {
		return err
	}
	return nil
}

// validateTools checks the parameter schema a declared function carries. A
// provider rejects a parameters member that is not an object, and the gateway
// would otherwise forward it unread.
func validateTools(tools []Tool) error {
	for _, tool := range tools {
		if len(bytes.TrimSpace(tool.Function.Parameters)) == 0 {
			continue
		}
		if !isJSONObject(tool.Function.Parameters) {
			return domain.NewValidationError("tool function parameters must be a JSON object")
		}
	}
	return nil
}

// validateResponseFormat checks the cross-field rule of response_format: the
// json_schema block is required for the json_schema type and refused for the
// others, and it must carry the name and object schema a provider needs.
func validateResponseFormat(format *ResponseFormat) error {
	if format == nil {
		return nil
	}
	switch format.Type {
	case "text", "json_object":
		if format.JSONSchema != nil {
			return domain.NewValidationError("response_format.json_schema is only valid for the json_schema type")
		}
	case "json_schema":
		if format.JSONSchema == nil || strings.TrimSpace(format.JSONSchema.Name) == "" {
			return domain.NewValidationError("response_format.json_schema.name is required")
		}
		if !isJSONObject(format.JSONSchema.Schema) {
			return domain.NewValidationError("response_format.json_schema.schema must be a JSON object")
		}
	}
	return nil
}
