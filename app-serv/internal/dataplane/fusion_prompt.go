// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/fusion_prompt.go
// @for       How a fusion panel member and its judge are asked, and how an
//
//	answer is read back (SPEC-API-001 §7.7).
//
// @uses      internal/schema, encoding/json, strconv, strings.
// @reason    A panel member receives the client's request stripped to what it
//
//	must answer in prose, while the judge receives it whole plus one
//	appended turn. Both shapes are edits to the same body, so keeping
//	them together makes the difference between the two auditable, and
//	keeping them out of the engine leaves the fan-out readable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// panelRequest is the client's request as a panel member receives it: never
// streamed, and with the tools withdrawn. A member that streamed would produce
// no complete answer to fuse, and a member that called a tool would return no
// prose at all.
func panelRequest(in Request) (Request, error) {
	panel := in
	panel.Stream = false
	raw, err := panelBody(in.Raw)
	if err != nil {
		return Request{}, err
	}
	panel.Raw = raw
	if in.Chat != nil {
		chat := *in.Chat
		chat.Stream = false
		chat.StreamOptions = nil
		chat.Tools = nil
		chat.ToolChoice = nil
		panel.Chat = &chat
	}
	if in.Messages != nil {
		messages := *in.Messages
		messages.Stream = false
		messages.Tools = nil
		messages.ToolChoice = nil
		panel.Messages = &messages
	}
	return panel, nil
}

// judgeRequest appends the synthesis directive as one more user turn. It starts
// from the client's request rather than the panel's, because the judge answers
// the client's actual question and is allowed to stream and to call tools.
func judgeRequest(in Request, prompt string) (Request, error) {
	request := in
	raw, err := appendUserTurn(in.Raw, prompt)
	if err != nil {
		return Request{}, err
	}
	request.Raw = raw
	if in.Chat != nil {
		chat := *in.Chat
		chat.Messages = append(append([]schema.ChatMessage(nil), in.Chat.Messages...),
			schema.ChatMessage{Role: RoleUser, Content: schema.MessageContent{Text: prompt}})
		request.Chat = &chat
	}
	if in.Messages != nil {
		messages := *in.Messages
		messages.Messages = append(append([]schema.Message(nil), in.Messages.Messages...),
			schema.Message{Role: RoleUser, Content: schema.MessageBlocks{{Type: schema.BlockText, Text: prompt}}})
		request.Messages = &messages
	}
	return request, nil
}

// panelBody edits a forwarded body into the panel's shape: the tool
// declarations are withdrawn so a member answers in prose instead of asking for
// a tool it cannot be given, and the stream flag is written false because a
// same-format call forwards the raw body — clearing only the decoded field
// would leave the member streaming.
func panelBody(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	body, ok := decodeObject(raw)
	if !ok {
		return nil, dataPlaneError(CodeValidation, "the request body could not be read")
	}
	delete(body, "tools")
	delete(body, "tool_choice")
	delete(body, "stream_options")
	body["stream"] = mustJSON(false)
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, wrapDataPlaneError(CodeValidation, "the request body could not be re-encoded", err)
	}
	return encoded, nil
}

// appendUserTurn adds one user turn to a forwarded body's message list.
func appendUserTurn(raw []byte, text string) ([]byte, error) {
	body, ok := decodeObject(raw)
	if !ok {
		return nil, dataPlaneError(CodeValidation, "the request body could not be read")
	}
	messages, _ := arrayField(body, "messages")
	turn, err := json.Marshal(map[string]string{"role": RoleUser, "content": text})
	if err != nil {
		return nil, wrapDataPlaneError(CodeValidation, "the judge request could not be built", err)
	}
	body["messages"] = mustJSON(append(messages, turn))
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, wrapDataPlaneError(CodeValidation, "the judge request could not be re-encoded", err)
	}
	return encoded, nil
}

// answerText reads the assistant prose out of a non-streamed answer, in the
// client's own wire format: the panel's answers are already translated back for
// the client, so the judge's input is read from the same shape the client would
// have received.
func answerText(body []byte, format schema.DataPlaneFormat) string {
	if len(body) == 0 {
		return ""
	}
	if format == schema.FormatAnthropic {
		var response schema.MessagesResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return ""
		}
		texts := make([]string, 0, len(response.Content))
		for _, block := range response.Content {
			if block.Type == schema.BlockText && block.Text != "" {
				texts = append(texts, block.Text)
			}
		}
		return joinText(texts)
	}
	var response schema.ChatCompletionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ""
	}
	if len(response.Choices) == 0 {
		return ""
	}
	return response.Choices[0].Message.Content.TextContent()
}

// buildJudgePrompt is the synthesis directive the judge model receives. Sources
// are anonymized ("Source N") so the judge weighs substance rather than the
// reputation of a model brand, and the judge is told not to mention the panel:
// the client asked one question and receives one answer.
func buildJudgePrompt(answers []fusionAnswer) string {
	var panel strings.Builder
	for index, answer := range answers {
		if index > 0 {
			panel.WriteString("\n\n")
		}
		panel.WriteString("[Source ")
		panel.WriteString(strconv.Itoa(index + 1))
		panel.WriteString("]\n")
		panel.WriteString(answer.text)
	}
	return "You are the JUDGE in a model-fusion panel. " + strconv.Itoa(len(answers)) +
		" expert models independently answered the user's most recent request. Their responses are " +
		"below, anonymized by source.\n\n" +
		"Do NOT mention that multiple models were used, and do NOT refer to the sources. Produce ONE " +
		"authoritative final answer addressed directly to the user.\n\n" +
		"First, internally analyze the panel along these dimensions: consensus (points most sources " +
		"agree on — treat as higher-confidence), contradictions (where they disagree — resolve with " +
		"your own judgment), partial coverage, unique insights only one source surfaced, and blind " +
		"spots every source missed. Then write the best possible final answer grounded in that " +
		"analysis — more complete and correct than any single response, with no filler.\n\n" +
		"=== PANEL RESPONSES ===\n" + panel.String() + "\n=== END PANEL RESPONSES ===\n\n" +
		"Now write the final answer to the user's original request."
}
