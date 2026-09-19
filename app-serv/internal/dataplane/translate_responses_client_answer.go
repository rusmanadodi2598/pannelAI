// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_client_answer.go
// @for       Building the Responses answer a client on /api/v1/responses
//
//	receives, whichever format the upstream wrote it in.
//
// @uses      encoding/json, strings, internal/schema.
// @reason    SPEC-API-001 §7.15 serves the Responses wire, so the answer has to
//
//	be assembled rather than forwarded whenever the resolved provider
//	speaks another format. One builder folds the pivot form the
//	translators already produce, which is what keeps three upstream
//	formats from becoming three answer builders.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// OpenAIToResponsesAnswer translates a raw OpenAI completion into a Responses
// answer.
func OpenAIToResponsesAnswer(raw []byte, model string, created int64) (schema.ResponsesAnswer, error) {
	var completion schema.ChatCompletionResponse
	if err := json.Unmarshal(raw, &completion); err != nil {
		return schema.ResponsesAnswer{}, dataPlaneError(CodeUpstreamError, "the upstream answered with an unreadable body")
	}
	return ResponsesAnswerFromCompletion(completion, model, created), nil
}

// ClaudeToResponsesAnswer translates an Anthropic messages response into a
// Responses answer, by way of the OpenAI completion the pivot already produces.
func ClaudeToResponsesAnswer(raw []byte, model string, created int64) (schema.ResponsesAnswer, error) {
	completion, err := ClaudeToOpenAIResponse(raw, model, created)
	if err != nil {
		return schema.ResponsesAnswer{}, err
	}
	return ResponsesAnswerFromCompletion(completion, model, created), nil
}

// ResponsesToResponsesAnswer reads an upstream that already speaks the Responses
// wire. The body is normalized rather than forwarded so the client sees the same
// identity and model the other directions report.
func ResponsesToResponsesAnswer(raw []byte, model string) (schema.ResponsesAnswer, error) {
	var answer schema.ResponsesAnswer
	if err := json.Unmarshal(raw, &answer); err != nil {
		return schema.ResponsesAnswer{}, dataPlaneError(CodeUpstreamError, "the upstream answered with an unreadable body")
	}
	if answer.Object == "" {
		answer.Object = schema.ResponsesObjectResponse
	}
	if answer.ID == "" {
		answer.ID = responsesAnswerID("")
	}
	if answer.Status == "" {
		answer.Status = schema.ResponsesStatusCompleted
	}
	if answer.Output == nil {
		answer.Output = []schema.ResponsesOutputItem{}
	}
	answer.Model = model
	return answer, nil
}

// ResponsesAnswerFromCompletion folds the pivot completion into the Responses
// answer.
//
// Output items are emitted reasoning first, then the message, then one
// function_call per call, each at its own index. The reference's own assembler
// keys items by the OpenAI choice and tool-call index, so a non-streamed answer
// carrying both reasoning and text loses one of them; the gateway builds the
// answer from the decoded completion instead, which is what keeps every part the
// upstream sent.
func ResponsesAnswerFromCompletion(completion schema.ChatCompletionResponse, model string, created int64) schema.ResponsesAnswer {
	id := responsesAnswerID(completion.ID)
	answer := schema.ResponsesAnswer{
		ID: id, Object: schema.ResponsesObjectResponse,
		CreatedAt: created, Status: schema.ResponsesStatusCompleted, Model: model,
		Output: []schema.ResponsesOutputItem{},
	}

	if len(completion.Choices) > 0 {
		message := completion.Choices[0].Message
		if message.Reasoning != "" {
			answer.Output = append(answer.Output, schema.ResponsesOutputItem{
				ID: "rs_" + id + "_0", Type: schema.ResponsesItemReasoning,
				Summary: []schema.ResponsesSummary{schema.SummaryPart(message.Reasoning)},
			})
		}
		if text := message.Content.TextContent(); text != "" {
			answer.Output = append(answer.Output, schema.ResponsesOutputItem{
				ID: "msg_" + id + "_0", Type: schema.ResponsesItemMessage, Role: schema.RoleAssistant,
				Content: []schema.ResponsesOutputPart{schema.TextPart(text)},
			})
		}
		for _, call := range message.ToolCalls {
			if item, ok := responsesAnswerCall(call); ok {
				answer.Output = append(answer.Output, item)
			}
		}
	}

	if completion.Usage != nil {
		usage := schema.ResponsesUsageFrom(*completion.Usage)
		answer.Usage = &usage
	}
	return answer
}

// responsesAnswerCall builds the function_call item one tool call stands for, or
// reports false when the call has no name: a client cannot dispatch a nameless
// call, so reporting one would look like a call it can make.
func responsesAnswerCall(call schema.ToolCall) (schema.ResponsesOutputItem, bool) {
	if call.Function.Name == "" {
		return schema.ResponsesOutputItem{}, false
	}
	arguments := call.Function.Arguments
	if arguments == "" {
		arguments = "{}"
	}
	return schema.ResponsesOutputItem{
		ID: "fc_" + call.ID, Type: schema.ResponsesItemFunctionCall,
		CallID: call.ID, Name: call.Function.Name, Arguments: arguments,
	}, true
}

// responsesAnswerID names the answer: the upstream's id under the Responses
// prefix, so a client correlating its logs with the provider's has one
// identifier rather than two.
func responsesAnswerID(upstreamID string) string {
	base := responseID(upstreamID, "pannelai")
	if strings.HasPrefix(base, "resp_") {
		return base
	}
	return "resp_" + base
}
