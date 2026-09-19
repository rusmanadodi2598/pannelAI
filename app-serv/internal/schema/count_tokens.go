// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/count_tokens.go
// @for       The Anthropic count_tokens contract: the estimate request, its
//
//	answer, and the message text the estimate is measured over.
//
// @uses      encoding/json, internal/domain.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/messages/count_tokens in P3.
//
//	The route answers an estimate instead of calling an upstream, so the
//	only input it reads is the message text; the union that carries it is
//	resolved here for the same reason the messages route resolves it here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// CountTokensRequest is the body of POST /api/v1/messages/count_tokens.
//
// It carries the members the estimate reads and tolerates the ones it does not
// model (`betas`, `metadata`): this route faces CLI tools, and refusing a body
// the upstream would accept would make the estimate less available than the
// call it estimates. `model` is required because the Anthropic contract names
// one; `messages` is not, because an empty list is a legitimate estimate of
// zero and the reference answers it.
type CountTokensRequest struct {
	Model      string          `json:"model" validate:"required,min=1,max=200"`
	Messages   []Message       `json:"messages,omitempty" validate:"dive"`
	System     TextBlocks      `json:"system,omitempty"`
	Tools      []ToolBlock     `json:"tools,omitempty" validate:"dive"`
	ToolChoice *ToolChoiceRule `json:"tool_choice,omitempty"`
	Thinking   *ThinkingBlock  `json:"thinking,omitempty"`
}

// CountTokensResponse is the route's answer, in Anthropic's own shape.
type CountTokensResponse struct {
	InputTokens int `json:"input_tokens"`
}

// DecodeCountTokensRequest decodes an Anthropic-wire body into its typed
// contract, the way DecodeMessagesRequest does for the messages route.
func DecodeCountTokensRequest(raw []byte) (CountTokensRequest, error) {
	var req CountTokensRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return CountTokensRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}

// InputChars counts the characters the estimate is measured over: the text of
// every message's content blocks. A bare-string body is already one text block
// by the time it is decoded here, so both permitted shapes measure the same.
//
// Non-text blocks (images, tool calls and their results, thinking) contribute
// nothing, which is the reference's rule: it counts `text` parts only.
func (r CountTokensRequest) InputChars() int {
	chars := 0
	for _, message := range r.Messages {
		for _, block := range message.Content {
			if block.Type == BlockText {
				chars += len(block.Text)
			}
		}
	}
	return chars
}
