// Package schema defines the typed request and response contracts of the API.
//
// @file      internal/schema/messages_responses_validation_test.go
// @for       The validation that must reach inside an Anthropic content block and a Responses item.
// @uses      testing
// @reason    A tag on a nested type is only reached when the parent field dives. Both wires carried tags below a field without one, so a malformed block or an unbounded number passed as validated input.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-04
package schema

import "testing"

func messagesRequestWith(blocks ...Block) MessagesRequest {
	return MessagesRequest{
		Model:    "claude-x",
		Messages: []Message{{Role: "user", Content: MessageBlocks(blocks)}},
	}
}

func TestValidateStruct_MessagesDescendsIntoContentBlocks(t *testing.T) {
	cases := []struct {
		name    string
		request MessagesRequest
		wantErr bool
	}{
		{
			name:    "a block that names no type is refused",
			request: messagesRequestWith(Block{Text: "hello"}),
			wantErr: true,
		},
		{
			name: "an image source outside the permitted kinds is refused",
			request: messagesRequestWith(Block{
				Type: "image", Source: &MediaSource{Type: "inline", Data: "AAAA"},
			}),
			wantErr: true,
		},
		{
			name:    "a message carrying no content is refused",
			request: MessagesRequest{Model: "claude-x", Messages: []Message{{Role: "user"}}},
			wantErr: true,
		},
		{
			name:    "a well formed text block is accepted",
			request: messagesRequestWith(Block{Type: "text", Text: "hello"}),
			wantErr: false,
		},
		{
			name: "a base64 source is accepted",
			request: messagesRequestWith(Block{
				Type: "image", Source: &MediaSource{Type: "base64", MediaType: "image/png", Data: "AAAA"},
			}),
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(tc.request)
			if tc.wantErr && err == nil {
				t.Fatal("ValidateStruct() passed a body the contract must refuse")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateStruct() = %v, want the valid body accepted", err)
			}
		})
	}
}

func TestValidateStruct_MessagesBoundsStopSequences(t *testing.T) {
	// One past the `max=32` guard on the stop_sequences tag.
	many := make([]string, 33)
	for index := range many {
		many[index] = "stop"
	}
	if err := ValidateStruct(MessagesRequest{
		Model: "claude-x", Messages: messagesRequestWith(Block{Type: "text"}).Messages, StopSequences: many,
	}); err == nil {
		t.Fatal("an unbounded stop_sequences array passed validation")
	}
	if err := ValidateStruct(MessagesRequest{
		Model: "claude-x", Messages: messagesRequestWith(Block{Type: "text"}).Messages,
		StopSequences: []string{"\n\nHuman:"},
	}); err != nil {
		t.Fatalf("one stop sequence failed validation: %v", err)
	}
}

func TestValidateStruct_ResponsesRequest(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "a bare string input is accepted",
			body:    `{"model":"gpt-5","input":"hello"}`,
			wantErr: false,
		},
		{
			name:    "an input item part that names no type is refused",
			body:    `{"model":"gpt-5","input":[{"role":"user","content":[{"text":"hello"}]}]}`,
			wantErr: true,
		},
		{
			name:    "a temperature outside the accepted range is refused",
			body:    `{"model":"gpt-5","input":"hello","temperature":9}`,
			wantErr: true,
		},
		{
			name:    "a top_p above one is refused",
			body:    `{"model":"gpt-5","input":"hello","top_p":4}`,
			wantErr: true,
		},
		{
			name:    "a tool declaration with no name is accepted, the wire allows a type-only entry",
			body:    `{"model":"gpt-5","input":"hello","tools":[{"type":"function"}]}`,
			wantErr: false,
		},
		{
			name:    "a prompt cache key past the guard length is refused",
			body:    `{"model":"gpt-5","input":"hello","prompt_cache_key":"` + repeatA(513) + `"}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := DecodeResponsesRequest([]byte(tc.body))
			if err != nil {
				t.Fatalf("DecodeResponsesRequest() error = %v", err)
			}
			err = ValidateStruct(req)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateStruct() passed %s", tc.body)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateStruct() = %v for %s", err, tc.body)
			}
		})
	}
}

// repeatA builds a string of n "a" bytes, so a length bound can be tested without a literal.
func repeatA(n int) string {
	runes := make([]byte, n)
	for index := range runes {
		runes[index] = 'a'
	}
	return string(runes)
}
