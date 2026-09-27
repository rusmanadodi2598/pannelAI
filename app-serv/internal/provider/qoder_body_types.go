// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_body_types.go
// @for       The two payload shapes of a Qoder chat call: what the gateway translated, and what
//
//	the vendor reads.
//
// @uses      encoding/json.
// @reason    The vendor reads an agent request, not an OpenAI one, and every field here is
//
//	named because several of them route the call (`chat_task`, `agent_id`,
//	`session_type`) while a wrong `model_config` is answered with a different
//	model. They sit apart from the builder so the wire shape can be read against
//	the reference without the derivation in the way.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"encoding/json"
)

// qoderOpenAIRequest is the slice of the translated body this builder reads. The
// members it does not name are dropped on purpose: the agent payload has no place
// for a client's `temperature`, and passing an unknown field to this vendor is how a
// request gets refused for a reason the client cannot act on.
type qoderOpenAIRequest struct {
	Model               string          `json:"model"`
	Messages            []qoderMessage  `json:"messages"`
	Tools               json.RawMessage `json:"tools"`
	MaxTokens           int             `json:"max_tokens"`
	MaxCompletionTokens int             `json:"max_completion_tokens"`
}

// qoderMessage keeps content raw until it is inspected: the vendor accepts a plain
// string for a text turn and an array when a turn carries an image.
type qoderMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// qoderAgentRequest is the outbound payload. Every field is named because the vendor
// routes on several of them (`chat_task`, `agent_id`, `session_type`) and rejects a
// request whose shape it does not recognise.
type qoderAgentRequest struct {
	RequestID      string           `json:"request_id"`
	RequestSetID   string           `json:"request_set_id"`
	ChatRecordID   string           `json:"chat_record_id"`
	SessionID      string           `json:"session_id"`
	Stream         bool             `json:"stream"`
	ChatTask       string           `json:"chat_task"`
	IsReply        bool             `json:"is_reply"`
	IsRetry        bool             `json:"is_retry"`
	Source         int              `json:"source"`
	Version        string           `json:"version"`
	SessionType    string           `json:"session_type"`
	AgentID        string           `json:"agent_id"`
	TaskID         string           `json:"task_id"`
	CodeLanguage   string           `json:"code_language"`
	ChatPrompt     string           `json:"chat_prompt"`
	ImageURLs      []string         `json:"image_urls"`
	AliyunUserType string           `json:"aliyun_user_type"`
	System         string           `json:"system"`
	Messages       []qoderMessage   `json:"messages"`
	Tools          []qoderTool      `json:"tools"`
	Parameters     qoderParameters  `json:"parameters"`
	ChatContext    qoderChatContext `json:"chat_context"`
	ModelConfig    json.RawMessage  `json:"model_config"`
	Business       qoderBusiness    `json:"business"`
}

type qoderParameters struct {
	MaxTokens int `json:"max_tokens"`
}

// qoderTool is kept as a raw object: the vendor executes tools, and a gateway that
// re-picked their fields would be free to change what the model is offered.
type qoderTool = json.RawMessage

type qoderChatContext struct {
	ChatPrompt string                `json:"chatPrompt"`
	ImageURLs  []string              `json:"imageUrls"`
	Extra      qoderChatContextExtra `json:"extra"`
	Features   []string              `json:"features"`
	Text       string                `json:"text"`
}

type qoderChatContextExtra struct {
	Context         []string            `json:"context"`
	ModelConfig     qoderModelSelection `json:"modelConfig"`
	OriginalContent string              `json:"originalContent"`
}

type qoderModelSelection struct {
	Key         string `json:"key"`
	IsReasoning bool   `json:"is_reasoning"`
}

type qoderBusiness struct {
	Product string `json:"product"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Stage   string `json:"stage"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	BeginAt int64  `json:"begin_at"`
}
