// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_chat_request_test.go
// @for       Request-schema parity for the two chat wires.
// @uses      encoding/json, reflect, strings, testing, internal/schema.
// @reason    F5 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md found
// the served contract describing POST /api/v1/chat/completions with the
// Anthropic MessagesRequest. This test pins the request reference per wire and
// the property set of the Chat request DTO, so the two cannot drift apart again.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestOpenAPIContract_ChatRequestSchemas pins the request schema each chat wire
// references, so the OpenAI route never describes the Anthropic body again.
func TestOpenAPIContract_ChatRequestSchemas(t *testing.T) {
	doc := loadContract(t)
	cases := []struct {
		path   string
		method string
		want   string
	}{
		{path: "/api/v1/chat/completions", method: "post", want: "ChatRequest"},
		{path: "/api/v1/messages", method: "post", want: "MessagesRequest"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			operation, ok := doc.Paths[tc.path][tc.method]
			if !ok {
				t.Fatalf("%s is missing from the contract", tc.path)
			}
			got := requestSchemaName(t, operation)
			if got != tc.want {
				t.Fatalf("%s request schema = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// TestOpenAPIContract_ChatRequestProperties pins that the published ChatRequest
// properties are exactly the json tags of schema.ChatRequest, so a renamed field
// fails here rather than at a client that sends the old name.
func TestOpenAPIContract_ChatRequestProperties(t *testing.T) {
	doc := loadContract(t)
	raw, ok := doc.Components.Schemas["ChatRequest"]
	if !ok {
		t.Fatal("ChatRequest is missing from the contract")
	}
	var definition struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatalf("decoding ChatRequest: %v", err)
	}
	want := jsonFieldNames(reflect.TypeOf(schema.ChatRequest{}))
	for _, field := range want {
		if _, present := definition.Properties[field]; !present {
			t.Errorf("ChatRequest is missing property %q", field)
		}
	}
	for property := range definition.Properties {
		if !contains(want, property) {
			t.Errorf("ChatRequest declares property %q, which schema.ChatRequest does not render", property)
		}
	}
	for _, required := range []string{"model", "messages"} {
		if !contains(definition.Required, required) {
			t.Errorf("ChatRequest does not require %q", required)
		}
	}
}

// requestSchemaName follows an operation's request body to the schema name
// behind it, so a test names the wire contract rather than its reference path.
func requestSchemaName(t *testing.T, operation contractOperation) string {
	t.Helper()
	var body struct {
		Content map[string]struct {
			Schema struct {
				Ref string `json:"$ref"`
			} `json:"schema"`
		} `json:"content"`
	}
	if len(operation.RequestBody) == 0 {
		return ""
	}
	if err := json.Unmarshal(operation.RequestBody, &body); err != nil {
		t.Fatalf("decoding the request body: %v", err)
	}
	for _, media := range body.Content {
		name, _, ok := splitRef(media.Schema.Ref)
		if ok {
			return name
		}
	}
	return ""
}

// TestRequestSchemaName_ResolvesTheOpenAIWire is a benign control for the helper
// above: a reference that does not resolve must not read as a match.
func TestRequestSchemaName_ResolvesTheOpenAIWire(t *testing.T) {
	doc := loadContract(t)
	operation := doc.Paths["/api/v1/chat/completions"]["post"]
	if got := requestSchemaName(t, operation); strings.HasPrefix(got, "Messages") {
		t.Fatalf("the OpenAI route resolves to the Anthropic schema %q", got)
	}
}
