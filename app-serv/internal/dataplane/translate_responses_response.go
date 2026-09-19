// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_response.go
// @for       Non-streamed Responses API answer translation, in both client
//
//	directions.
//
// @uses      internal/schema.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable and §7.15
//
//	serves the OpenAI and Anthropic wires, so one upstream answer has to
//	become either. Both directions read one decoded form, which is what
//	keeps the text, tool calls, and usage from being extracted twice; the
//	item readers live in translate_responses_answer.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// responsesAnswer is the decoded form both client directions read.
type responsesAnswer struct {
	// ID is the upstream's response id, kept so a client correlating its own
	// logs with the provider's has one identifier.
	ID string
	// Texts are the output_text parts, in order.
	Texts []string
	// Reasoning is the joined reasoning summary, when the upstream reported one.
	Reasoning string
	// Calls are the function calls the model asked for.
	Calls []schema.ToolCall
	// Finish is the OpenAI finish reason the answer's status implies.
	Finish string
	// Usage is the accounting the upstream reported, or nil when it reported
	// none, so a caller records nothing rather than a fabricated zero.
	Usage *schema.Usage

	// reasonings collects every reasoning item's summary, in order.
	reasonings []string
}

// ResponsesToOpenAIResponse translates a non-streamed Responses object into an
// OpenAI completion.
func ResponsesToOpenAIResponse(raw []byte, model string, created int64) (schema.ChatCompletionResponse, error) {
	answer, err := decodeResponsesAnswer(raw)
	if err != nil {
		return schema.ChatCompletionResponse{}, err
	}
	message := schema.ChatMessage{
		Role:      RoleAssistant,
		Content:   schema.MessageContent{Text: joinText(answer.Texts)},
		ToolCalls: answer.Calls,
		Reasoning: answer.Reasoning,
	}
	return schema.ChatCompletionResponse{
		ID:      responseID(answer.ID, "chatcmpl-pannelai"),
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []schema.ChatChoice{{
			Index:        0,
			Message:      message,
			FinishReason: answer.Finish,
		}},
		Usage: answer.Usage,
	}, nil
}

// ResponsesToClaudeResponse translates a non-streamed Responses object into an
// Anthropic messages response, which is the path a client on /api/v1/messages
// takes when the resolved provider speaks the Responses API.
func ResponsesToClaudeResponse(raw []byte, model string) (schema.MessagesResponse, error) {
	answer, err := decodeResponsesAnswer(raw)
	if err != nil {
		return schema.MessagesResponse{}, err
	}

	blocks := make([]schema.Block, 0, len(answer.Calls)+2)
	if text := joinText(answer.Texts); text != "" {
		blocks = append(blocks, schema.Block{Type: schema.BlockText, Text: text})
	}
	if answer.Reasoning != "" {
		blocks = append(blocks, schema.Block{Type: schema.BlockThinking, Thinking: answer.Reasoning})
	}
	for _, call := range answer.Calls {
		blocks = append(blocks, schema.Block{
			Type:  schema.BlockToolUse,
			ID:    call.ID,
			Name:  call.Function.Name,
			Input: argumentObject(call.Function.Arguments),
		})
	}

	usage := schema.MessagesUsage{}
	if answer.Usage != nil {
		usage = OpenAIToClaudeUsage(*answer.Usage)
	}
	return schema.MessagesResponse{
		ID:           responseID(answer.ID, "msg_pannelai"),
		Type:         "message",
		Role:         RoleAssistant,
		Model:        model,
		Content:      blocks,
		StopReason:   claudeStopReason(answer.Finish),
		StopSequence: nil,
		Usage:        usage,
	}, nil
}

// decodeResponsesAnswer reads one Responses object into the form both client
// directions consume. An unreadable body is an upstream failure, not a client
// error: the client's own request was already accepted.
func decodeResponsesAnswer(raw []byte) (responsesAnswer, error) {
	response, ok := decodeObject(raw)
	if !ok {
		return responsesAnswer{}, dataPlaneError(CodeUpstreamError, "the upstream answered with an unreadable body")
	}
	answer := responsesAnswer{ID: stringField(response, "id")}

	items, _ := arrayField(response, "output")
	for _, rawItem := range items {
		item, ok := decodeObject(rawItem)
		if !ok {
			continue
		}
		switch stringField(item, "type") {
		case ItemMessage:
			answer.Texts = append(answer.Texts, responsesTextParts(item)...)
		case ItemFunctionCall:
			if call, ok := responsesCall(item); ok {
				answer.Calls = append(answer.Calls, call)
			}
		case ItemReasoning:
			answer.reasonings = append(answer.reasonings, responsesReasoningParts(item)...)
		}
	}
	answer.Reasoning = joinText(answer.reasonings)

	answer.Finish = responsesFinishReason(response, len(answer.Calls) > 0)
	if usage, ok := objectField(response, "usage"); ok {
		answer.Usage = responsesUsageFromObject(usage)
	}
	return answer, nil
}
