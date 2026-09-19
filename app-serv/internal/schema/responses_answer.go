// Package schema holds the typed contracts each route decodes into.
//
// @file      internal/schema/responses_answer.go
// @for       The OpenAI Responses answer contract: the response object, its
//
//	output items, and the accounting it reports.
//
// @uses      encoding/json.
// @reason    SPEC-API-001 §7.15 makes POST /api/v1/responses a P3 deliverable,
//
//	and the answer has to be built rather than forwarded whenever the
//	resolved provider speaks another format. One typed shape is what
//	keeps the non-streamed answer and the streamed lifecycle events
//	describing the same object.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import "encoding/json"

// The answer's lifecycle statuses and item kinds.
const (
	ResponsesStatusInProgress = "in_progress"
	ResponsesStatusCompleted  = "completed"
	ResponsesStatusFailed     = "failed"
	ResponsesStatusIncomplete = "incomplete"

	ResponsesObjectResponse = "response"

	// ResponsesSummaryText is the one summary-part kind a reasoning item carries.
	ResponsesSummaryText = "summary_text"
)

// ResponsesAnswer is the body POST /api/v1/responses answers with.
type ResponsesAnswer struct {
	ID        string                `json:"id"`
	Object    string                `json:"object"`
	CreatedAt int64                 `json:"created_at"`
	Status    string                `json:"status"`
	Model     string                `json:"model,omitempty"`
	Output    []ResponsesOutputItem `json:"output"`
	Usage     *ResponsesUsage       `json:"usage,omitempty"`
}

// ResponsesOutputItem is one entry of the answer's output array. One struct
// carries every item kind, discriminated by Type, matching the wire.
type ResponsesOutputItem struct {
	ID        string                `json:"id,omitempty"`
	Type      string                `json:"type"`
	Role      string                `json:"role,omitempty"`
	Content   []ResponsesOutputPart `json:"content,omitempty"`
	Summary   []ResponsesSummary    `json:"summary,omitempty"`
	CallID    string                `json:"call_id,omitempty"`
	Name      string                `json:"name,omitempty"`
	Arguments string                `json:"arguments,omitempty"`
}

// ResponsesOutputPart is one part of a message item's content. Annotations and
// logprobs are carried as empty arrays rather than omitted, which is the shape
// the reference emits and what a client reading them expects to find.
type ResponsesOutputPart struct {
	Type        string            `json:"type"`
	Text        string            `json:"text"`
	Annotations []json.RawMessage `json:"annotations"`
	Logprobs    []json.RawMessage `json:"logprobs"`
}

// ResponsesUsage is the Responses accounting block. The cached and reasoning
// splits travel as details because input_tokens already includes the cached
// tokens, the same rule the upstream direction reads.
type ResponsesUsage struct {
	InputTokens  int                       `json:"input_tokens"`
	OutputTokens int                       `json:"output_tokens"`
	TotalTokens  int                       `json:"total_tokens"`
	InputDetails *ResponsesInputTokenInfo  `json:"input_tokens_details,omitempty"`
	OutDetails   *ResponsesOutputTokenInfo `json:"output_tokens_details,omitempty"`
}

// ResponsesInputTokenInfo is the input side's detail block.
type ResponsesInputTokenInfo struct {
	CachedTokens int `json:"cached_tokens"`
}

// ResponsesOutputTokenInfo is the output side's detail block.
type ResponsesOutputTokenInfo struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

// ResponsesStatus is the response object the stream's lifecycle events carry,
// which is a subset of the answer: a client reads its id, status, and creation
// instant, and nothing else is known at that point.
type ResponsesStatus struct {
	ID         string                `json:"id"`
	Object     string                `json:"object"`
	CreatedAt  int64                 `json:"created_at"`
	Status     string                `json:"status"`
	Background bool                  `json:"background"`
	Error      *struct{}             `json:"error"`
	Output     []ResponsesOutputItem `json:"output,omitempty"`
}

// NewResponsesStatus builds the object a lifecycle event reports.
func NewResponsesStatus(id string, createdAt int64, status string, withOutput bool) ResponsesStatus {
	statusObject := ResponsesStatus{
		ID: id, Object: ResponsesObjectResponse, CreatedAt: createdAt, Status: status,
	}
	if withOutput {
		statusObject.Output = []ResponsesOutputItem{}
	}
	return statusObject
}

// ResponsesUsageFrom folds the gateway's internal accounting into the Responses
// shape. The internal block already follows the same rule (prompt tokens include
// the cached ones), so the cached split is copied as a detail rather than added.
func ResponsesUsageFrom(usage Usage) ResponsesUsage {
	out := ResponsesUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
	}
	if usage.PromptTokensDetails != nil {
		cached := usage.PromptTokensDetails.CachedTokens
		if cached > 0 {
			out.InputDetails = &ResponsesInputTokenInfo{CachedTokens: cached}
		}
	}
	if usage.CompletionTokensDetail != nil {
		reasoning := usage.CompletionTokensDetail.ReasoningTokens
		if reasoning > 0 {
			out.OutDetails = &ResponsesOutputTokenInfo{ReasoningTokens: reasoning}
		}
	}
	return out
}

// TextPart builds an output_text part carrying the given text.
func TextPart(text string) ResponsesOutputPart {
	return ResponsesOutputPart{
		Type: ResponsesPartOutputText, Text: text,
		Annotations: []json.RawMessage{}, Logprobs: []json.RawMessage{},
	}
}

// SummaryPart builds a summary_text part carrying the given text.
func SummaryPart(text string) ResponsesSummary {
	return ResponsesSummary{Type: ResponsesSummaryText, Text: text}
}
