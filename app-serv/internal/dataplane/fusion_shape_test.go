// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/fusion_shape_test.go
// @for       How a panel member and the judge are shaped: the same-format raw
//
//	body and the decoded request are both edited, or a cross-format call
//	would keep what a same-format one withdrew.
//
// @uses      testing, internal/schema.
// @reason    These transformations decide what a panel member is asked and what
//
//	the judge keeps, and each is decidable without a network — so a table
//	pins every wire-format spelling here rather than in a fusion test that
//	would report a regression as a mysterious panel failure.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestPanelBodyWithdrawsToolsAndStreaming pins the same-format edit: the raw
// body a panel member receives carries no tools, no stream options, and an
// explicit stream of false, whatever the client sent.
func TestPanelBodyWithdrawsToolsAndStreaming(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "an OpenAI body",
			raw: `{"model":"panel","stream":true,"stream_options":{"include_usage":true},` +
				`"messages":[{"role":"user","content":"ping"}],` +
				`"tools":[{"type":"function","function":{"name":"lookup"}}],"tool_choice":"auto"}`,
		},
		{
			name: "an Anthropic body",
			raw: `{"model":"panel","stream":true,"max_tokens":16,` +
				`"messages":[{"role":"user","content":"ping"}],` +
				`"tools":[{"name":"lookup","input_schema":{"type":"object"}}],"tool_choice":{"type":"auto"}}`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			edited, err := panelBody([]byte(testCase.raw))
			if err != nil {
				t.Fatalf("panelBody() error = %v", err)
			}
			body, ok := decodeObject(edited)
			if !ok {
				t.Fatalf("panelBody() = %s, want a JSON object", edited)
			}
			for _, withdrawn := range []string{"tools", "tool_choice", "stream_options"} {
				if _, present := body[withdrawn]; present {
					t.Fatalf("panelBody() kept %q: %s", withdrawn, edited)
				}
			}
			if stream := string(body["stream"]); stream != "false" {
				t.Fatalf("panelBody() stream = %s, want false", stream)
			}
			if _, present := body["messages"]; !present {
				t.Fatalf("panelBody() dropped the conversation: %s", edited)
			}
		})
	}
}

// TestPanelBodyRefusesAnUnreadableBody pins the boundary: a body that is not a
// JSON object is a client error, not a panel call.
func TestPanelBodyRefusesAnUnreadableBody(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "not JSON", raw: `ping`},
		{name: "a JSON array", raw: `[{"role":"user"}]`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := panelBody([]byte(testCase.raw))
			if err == nil {
				t.Fatal("panelBody() = nil error, want a validation failure")
			}
			if code := AsError(err).Code; code != CodeValidation {
				t.Fatalf("panelBody() code = %q, want %q", code, CodeValidation)
			}
		})
	}
}

// TestPanelRequestClearsTheDecodedBodyToo pins the cross-format half: when the
// member speaks another wire format the body is rebuilt from the decoded
// request, so the same withdrawal has to happen there.
func TestPanelRequestClearsTheDecodedBodyToo(t *testing.T) {
	openAI := fusionRequest("panel", true, true)
	panel, err := panelRequest(openAI)
	if err != nil {
		t.Fatalf("panelRequest(OpenAI) error = %v", err)
	}
	if panel.Stream || panel.Chat.Stream || panel.Chat.Tools != nil || panel.Chat.ToolChoice != nil {
		t.Fatalf("panelRequest(OpenAI) = %+v, want the panel shape", panel.Chat)
	}

	anthropic := Request{
		Route:        RouteMessages,
		ClientFormat: schema.FormatAnthropic,
		Model:        "panel",
		Stream:       true,
		Messages: &schema.MessagesRequest{
			Model:  "panel",
			Stream: true,
			Messages: []schema.Message{{
				Role: schema.RoleUser, Content: schema.MessageBlocks{{Type: schema.BlockText, Text: "ping"}},
			}},
			Tools: []schema.ToolBlock{{Name: "lookup"}},
		},
		Raw: []byte(`{"model":"panel","stream":true,"messages":[{"role":"user","content":"ping"}],` +
			`"tools":[{"name":"lookup"}]}`),
	}
	panel, err = panelRequest(anthropic)
	if err != nil {
		t.Fatalf("panelRequest(Anthropic) error = %v", err)
	}
	if panel.Stream || panel.Messages.Stream || panel.Messages.Tools != nil || panel.Messages.ToolChoice != nil {
		t.Fatalf("panelRequest(Anthropic) = %+v, want the panel shape", panel.Messages)
	}
}

// TestJudgeRequestAppendsTheDirectiveAndKeepsTools pins the judge's shape: the
// client's conversation and tools survive, and the directive arrives as the last
// user turn.
func TestJudgeRequestAppendsTheDirectiveAndKeepsTools(t *testing.T) {
	request, err := judgeRequest(fusionRequest("panel", true, false), "DIRECTIVE")
	if err != nil {
		t.Fatalf("judgeRequest() error = %v", err)
	}
	if request.Chat == nil || len(request.Chat.Messages) != 2 {
		t.Fatalf("judgeRequest() messages = %+v, want the client turn plus the directive", request.Chat)
	}
	last := request.Chat.Messages[1]
	if last.Role != RoleUser || last.Content.TextContent() != "DIRECTIVE" {
		t.Fatalf("judgeRequest() last turn = %+v, want the directive", last)
	}
	if request.Chat.Tools == nil {
		t.Fatal("judgeRequest() dropped the tools, want them kept for the served call")
	}

	raw, ok := decodeObject(request.Raw)
	if !ok {
		t.Fatalf("judgeRequest() raw = %s, want a JSON object", request.Raw)
	}
	messages, ok := arrayField(raw, "messages")
	if !ok || len(messages) != 2 {
		t.Fatalf("judgeRequest() raw messages = %s, want two turns", request.Raw)
	}
	if _, present := raw["tools"]; !present {
		t.Fatalf("judgeRequest() raw dropped the tools: %s", request.Raw)
	}
}
