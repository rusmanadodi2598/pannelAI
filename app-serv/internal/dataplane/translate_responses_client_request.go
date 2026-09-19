// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_client_request.go
// @for       Responses API request to OpenAI chat request translation, for a
//
//	client that called POST /api/v1/responses.
//
// @uses      internal/schema.
// @reason    SPEC-API-001 §7.15 serves the Responses wire and §8 refuses a format
//
//	the gateway cannot translate, so a client on that wire has to reach
//	any resolved provider. The reference implements this direction as a
//	pure function (openaiResponsesToOpenAIRequest,
//	open-sse/translator/request/openai-responses.js), so this is one
//	too: no clock, no I/O, no package state. The item-level rules live
//	in translate_responses_client_items.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// ResponsesToOpenAI translates a Responses API request into an OpenAI chat
// request, which is the pivot every other target is reached through.
//
// `instructions` becomes the leading system message because the Responses API
// has one instruction field while the chat wire has a system role, and dropping
// it would lose what the client told the model.
func ResponsesToOpenAI(req schema.ResponsesRequest, upstreamModel string, stream bool) schema.ChatRequest {
	out := schema.ChatRequest{
		Model:       upstreamModel,
		Stream:      stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Tools:       responsesClientTools(req.Tools),
	}
	// A non-positive ceiling is not a ceiling: the route's validation refuses one,
	// and a body that reached here anyway must not send `max_tokens: 0`, which
	// some providers read as "produce nothing".
	if req.MaxOutputTokens != nil && *req.MaxOutputTokens > 0 {
		out.MaxTokens = req.MaxOutputTokens
	}
	// The Responses wire carries accounting in its closing event rather than
	// behind a client-facing flag, so a streamed answer has to ask the upstream
	// for it: without this an OpenAI provider reports none and the client's
	// token counts come back empty. The reference omits the request and reports
	// no usage at all.
	if stream {
		out.StreamOptions = &schema.StreamOptions{IncludeUsage: true}
	}
	if req.Instructions != "" {
		out.Messages = append(out.Messages, schema.ChatMessage{
			Role: schema.RoleSystem, Content: schema.MessageContent{Text: req.Instructions},
		})
	}
	out.Messages = append(out.Messages, responsesClientMessages(req.Input)...)
	return out
}

// ResponsesToClaude translates a Responses API request into an Anthropic
// messages request by way of the OpenAI chat request, which is the pivot the
// reference's registry uses: one mapping per pair, not one per combination.
func ResponsesToClaude(req schema.ResponsesRequest, upstreamModel string, stream bool) ClaudeRequest {
	return OpenAIToClaude(ResponsesToOpenAI(req, upstreamModel, stream), upstreamModel, stream)
}
