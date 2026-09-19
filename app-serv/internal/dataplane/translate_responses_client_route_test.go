// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_client_route_test.go
// @for       Table-driven tests for the upstream body a Responses client's
//
//	request becomes, per resolved target.
//
// @uses      testing, encoding/json, strings.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses from any resolved
//
//	provider, so the client's format has to reach every translator. A
//	Responses-speaking target is forwarded verbatim, which is the rule
//	that keeps an unmodelled field from being dropped.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// responsesRouteRequest decodes one Responses body into the request routing sees.
func responsesRouteRequest(t *testing.T, body string) Request {
	t.Helper()
	decoded, err := schema.DecodeResponsesRequest([]byte(body))
	if err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	return Request{
		Route: RouteResponses, ClientFormat: schema.FormatOpenAIResponses,
		Model: decoded.Model, Stream: decoded.Stream, Responses: &decoded, Raw: []byte(body),
	}
}

// decodeBody unwraps one translated body into a generic object.
func decodeBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decoding translated body %s: %v", body, err)
	}
	return decoded
}

// TestResponsesClientRoute_UpstreamBody pins the body each target receives. The
// streaming flag and the usage request matter most: the Responses wire reports
// accounting in its closing event, so a streamed answer that never asks for it
// comes back with no counts at all.
func TestResponsesClientRoute_UpstreamBody(t *testing.T) {
	const body = `{"model":"pannelai-model","input":"hi","instructions":"be terse","max_output_tokens":64,"stream":true}`
	cases := []struct {
		name         string
		target       string
		wantModel    string
		wantCeiling  float64
		wantUsageAsk bool
	}{
		{
			name:   "an OpenAI target gets a chat request that asks for usage",
			target: TargetOpenAI, wantModel: "up-1",
			wantCeiling: 64, wantUsageAsk: true,
		},
		{
			name:   "an Anthropic target gets a messages request",
			target: TargetClaude, wantModel: "up-1",
			wantCeiling: 64,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := responsesRouteRequest(t, body)
			translated, err := translateUpstreamBody(request, Resolution{
				ModelID: "pannelai-model", UpstreamID: "up-1", Target: tc.target,
			})
			if err != nil {
				t.Fatalf("translateUpstreamBody() error = %v", err)
			}
			decoded := decodeBody(t, translated)

			if got, _ := decoded["model"].(string); got != tc.wantModel {
				t.Fatalf("model = %q, want %q", got, tc.wantModel)
			}
			if stream, _ := decoded["stream"].(bool); !stream {
				t.Fatalf("stream = %v, want true", decoded["stream"])
			}
			if got, _ := decoded["max_tokens"].(float64); got != tc.wantCeiling {
				t.Fatalf("max_tokens = %v, want %v", decoded["max_tokens"], tc.wantCeiling)
			}
			messages, _ := decoded["messages"].([]any)
			if len(messages) == 0 {
				t.Fatalf("translated body carries no messages: %s", translated)
			}
			// The instructions travel as a system turn, which the Anthropic
			// envelope lifts to a top-level member and the chat wire keeps as a
			// leading message.
			instruction, _ := decoded["system"].(string)
			if instruction == "" {
				// The Anthropic envelope carries the system turn as blocks.
				if blocks, ok := decoded["system"].([]any); ok && len(blocks) > 0 {
					instruction = "carried as system blocks"
				}
			}
			if instruction == "" {
				first, _ := messages[0].(map[string]any)
				if role, _ := first["role"].(string); role == "system" {
					instruction = "carried as a leading message"
				}
			}
			if instruction == "" {
				t.Fatalf("translated body lost the instructions: %s", translated)
			}
			if tc.wantUsageAsk {
				options, ok := decoded["stream_options"].(map[string]any)
				if !ok {
					t.Fatalf("streamed body carries no stream_options: %s", translated)
				}
				if include, _ := options["include_usage"].(bool); !include {
					t.Fatalf("stream_options = %v, want include_usage true", options)
				}
			}
		})
	}
}

// TestResponsesClientRoute_SameFormatIsForwarded pins the forwarding rule: a
// target already speaking the Responses wire receives the client's own body with
// only the model replaced, so a field the gateway does not model survives.
func TestResponsesClientRoute_SameFormatIsForwarded(t *testing.T) {
	const body = `{"model":"pannelai-model","input":[{"type":"message","role":"user","content":"hi"}],` +
		`"store":false,"include":["reasoning.encrypted_content"],"prompt_cache_key":"cache-1"}`
	request := responsesRouteRequest(t, body)
	translated, err := upstreamBody(request, Resolution{
		ModelID: "pannelai-model", UpstreamID: "up-1", Target: TargetResponses,
	})
	if err != nil {
		t.Fatalf("upstreamBody() error = %v", err)
	}
	decoded := decodeBody(t, translated)

	if got, _ := decoded["model"].(string); got != "up-1" {
		t.Fatalf("model = %q, want up-1", got)
	}
	for _, member := range []string{"include", "prompt_cache_key"} {
		if _, ok := decoded[member]; !ok {
			t.Fatalf("forwarded body dropped %s: %s", member, translated)
		}
	}
	if _, ok := decoded["input"].([]any); !ok {
		t.Fatalf("forwarded body rewrote input: %s", translated)
	}
	if store, _ := decoded["store"].(bool); store {
		t.Fatalf("forwarded body changed store: %s", translated)
	}
}

// TestResponsesClientRoute_Answer pins the non-streamed answer per target: the
// client always receives the Responses wire, whatever the upstream wrote.
func TestResponsesClientRoute_Answer(t *testing.T) {
	const openAIBody = `{"id":"chatcmpl-1","object":"chat.completion","model":"up",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
	const responsesBody = `{"id":"resp_up","object":"response","status":"completed",` +
		`"output":[{"id":"msg_1","type":"message","role":"assistant",` +
		`"content":[{"type":"output_text","text":"hi","annotations":[],"logprobs":[]}]}]}`
	cases := []struct {
		name   string
		target string
		body   string
		wantID string
	}{
		{name: "an OpenAI answer becomes a Responses answer", target: TargetOpenAI, body: openAIBody, wantID: "resp_chatcmpl-1"},
		{name: "a Responses answer keeps its own id", target: TargetResponses, body: responsesBody, wantID: "resp_up"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer, err := responsesClientAnswer([]byte(tc.body), Resolution{
				ModelID: "pannelai-model", UpstreamID: "up-1", Target: tc.target,
			}, 1700000000)
			if err != nil {
				t.Fatalf("responsesClientAnswer() error = %v", err)
			}
			if answer.ID != tc.wantID {
				t.Fatalf("id = %q, want %q", answer.ID, tc.wantID)
			}
			if answer.Object != schema.ResponsesObjectResponse {
				t.Fatalf("object = %q, want %q", answer.Object, schema.ResponsesObjectResponse)
			}
			if answer.Status != schema.ResponsesStatusCompleted {
				t.Fatalf("status = %q, want completed", answer.Status)
			}
			if answer.Model != "pannelai-model" {
				t.Fatalf("model = %q, want pannelai-model", answer.Model)
			}
			if len(answer.Output) == 0 {
				t.Fatalf("answer carries no output: %+v", answer)
			}
		})
	}
}

// TestResponsesClientRoute_UnreadableAnswer pins that an upstream body the
// Responses shape cannot read is reported as an upstream error rather than an
// empty answer a client would show as a blank reply.
func TestResponsesClientRoute_UnreadableAnswer(t *testing.T) {
	for _, target := range []string{TargetOpenAI, TargetResponses} {
		if _, err := responsesClientAnswer([]byte("not json"), Resolution{
			ModelID: "m", UpstreamID: "up-1", Target: target,
		}, 1700000000); err == nil {
			t.Fatalf("target %s accepted an unreadable body", target)
		}
	}
}

// TestResponsesClientRoute_BodyIsNeverSentAsIs pins that the translated body is
// valid JSON with the client's own text in it, which is the property a hand-built
// string would fail.
func TestResponsesClientRoute_BodyIsNeverSentAsIs(t *testing.T) {
	request := responsesRouteRequest(t, `{"model":"m","input":"say \"hi\"\nplease"}`)
	translated, err := translateUpstreamBody(request, Resolution{
		ModelID: "m", UpstreamID: "up-1", Target: TargetOpenAI,
	})
	if err != nil {
		t.Fatalf("translateUpstreamBody() error = %v", err)
	}
	if !json.Valid(translated) {
		t.Fatalf("translated body is not valid JSON: %s", translated)
	}
	if !strings.Contains(string(translated), `say \"hi\"\nplease`) {
		t.Fatalf("translated body lost the client's text: %s", translated)
	}
}
