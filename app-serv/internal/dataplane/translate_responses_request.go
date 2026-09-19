// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_request.go
// @for       OpenAI chat request to Responses API request translation.
// @uses      internal/schema.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable and §7.15
//
//	lets a client on either wire reach a provider that speaks it. The
//	reference implements this direction as a pure function
//	(open-sse/translator/request/openai-responses.js), so this is one too:
//	no clock, no I/O, no package state. The payload types and item
//	builders live in translate_responses_items.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// ResponsesCallIDMaxLen is the call_id ceiling the Responses API enforces. A
// longer id is clamped rather than sent to be rejected (reference
// `MAX_CALL_ID_LEN`, open-sse/translator/request/openai-responses.js).
const ResponsesCallIDMaxLen = 64

// OpenAIToResponses translates an OpenAI chat request into a Responses API
// request. upstreamModel is the id the upstream expects.
//
// The first system message becomes `instructions` and the rest are dropped,
// which is the reference's rule: the Responses API has one instruction field, and
// joining several system turns would change what the model is told. Tool results
// become `function_call_output` items because the Responses input array carries
// them as items rather than as messages.
func OpenAIToResponses(req schema.ChatRequest, upstreamModel string, stream bool) ResponsesRequest {
	out := ResponsesRequest{
		Model:        upstreamModel,
		Input:        make([]ResponsesItem, 0, len(req.Messages)),
		Instructions: "",
		Stream:       stream,
		Store:        false,
		Temperature:  req.Temperature,
		TopP:         req.TopP,
		Tools:        responsesTools(req.Tools),
	}
	if maxTokens := req.MaxOutputTokens(); maxTokens > 0 {
		out.MaxTokens = &maxTokens
	}

	instructionsSet := false
	for _, message := range req.Messages {
		switch message.Role {
		case schema.RoleSystem, schema.RoleDeveloper:
			if !instructionsSet {
				out.Instructions = message.Content.TextContent()
				instructionsSet = true
			}
			continue
		case schema.RoleUser, schema.RoleAssistant:
			if item, ok := responsesMessageItem(message); ok {
				out.Input = append(out.Input, item)
			}
		case schema.RoleTool:
			out.Input = append(out.Input, ResponsesItem{
				Type:   ItemFunctionCallOut,
				CallID: clampResponsesCallID(message.ToolCallID),
				Output: responsesToolOutput(message.Content),
			})
		}
		if message.Role == schema.RoleAssistant {
			out.Input = append(out.Input, responsesCallItems(message.ToolCalls)...)
		}
	}
	return out
}

// ClaudeToResponses translates an Anthropic messages request into a Responses
// API request by way of the OpenAI chat request, which is the pivot the
// reference's registry uses: one mapping per pair, not one per combination.
func ClaudeToResponses(req schema.MessagesRequest, upstreamModel string, stream bool) ResponsesRequest {
	return OpenAIToResponses(ClaudeToOpenAI(req, upstreamModel, stream), upstreamModel, stream)
}
