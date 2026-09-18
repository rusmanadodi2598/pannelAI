// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_response.go
// @for       The OpenAI-wire response contracts: completion, streamed chunk,
//
//	usage block, and the models list.
//
// @uses      internal/schema (ChatMessage, MessageContent).
// @reason    SPEC-API-001 §7.15 serves these shapes to CLI tools and §4 fixes the
//
//	usage chunk that stream_options.include_usage asks for. Keeping them
//	in the schema layer is what lets the translator and the handler agree
//	on one wire shape instead of each declaring its own (AGENTS.md §2.4).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

// Usage is the OpenAI token accounting block.
type Usage struct {
	PromptTokens           int                  `json:"prompt_tokens"`
	CompletionTokens       int                  `json:"completion_tokens"`
	TotalTokens            int                  `json:"total_tokens"`
	PromptTokensDetails    *PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetail *CompletionDetail    `json:"completion_tokens_details,omitempty"`
}

// PromptTokensDetails carries the cached-prompt split: cached_tokens is the
// OpenAI field, and the two cache_* names are the Claude cache fields a client
// that reads them keeps working.
type PromptTokensDetails struct {
	CachedTokens         int `json:"cached_tokens,omitempty"`
	CacheCreationTokens  int `json:"cache_creation_tokens,omitempty"`
	CacheReadInputTokens int `json:"cache_read_input_tokens,omitempty"`
}

// CompletionDetail carries the reasoning-token split OpenAI reports. Reasoning
// tokens are already counted inside CompletionTokens, which is why this is a
// detail and never an addition.
type CompletionDetail struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// ChatCompletionChunk is one streamed OpenAI frame.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// ChunkChoice is one streamed choice.
type ChunkChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// Delta is the incremental content of a streamed choice. Empty members are
// omitted, so a frame never claims to carry a field it did not.
type Delta struct {
	Role             string          `json:"role,omitempty"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCallDelta `json:"tool_calls,omitempty"`
}

// ToolCallDelta is one incremental tool call inside a streamed delta.
type ToolCallDelta struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function *FunctionDelta `json:"function,omitempty"`
}

// FunctionDelta is the incremental name and argument fragment of a tool call.
type FunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ChatCompletionResponse is the non-streamed OpenAI answer.
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

// ChatChoice is one non-streamed choice.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ModelObject is one entry of the OpenAI models list.
type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelList is the OpenAI models list response (§7.15).
type ModelList struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

// UsageChunk builds the final OpenAI frame that carries token accounting: an
// empty choices array with a usage object, which is the shape
// stream_options.include_usage asks for (SPEC-API-001 §4). The empty choices list
// is required, not decorative — a client that reads the first choice from every
// frame would otherwise read a frame with no content.
func UsageChunk(id string, created int64, model string, usage Usage) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []ChunkChoice{},
		Usage:   &usage,
	}
}

// TokenTotal reports the total tokens OpenAI expects, deriving it when a caller
// supplied only the two parts.
func (u Usage) TokenTotal() int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.PromptTokens + u.CompletionTokens
}
