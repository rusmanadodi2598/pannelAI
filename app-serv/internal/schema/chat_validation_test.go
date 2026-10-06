// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_validation_test.go
// @for       Table-driven semantic validation coverage for OpenAI chat requests.
// @uses      encoding/json, testing.
// @reason    F1 of the Playground Chat readiness plan requires nested union and semantic rules that struct tags alone cannot express.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-09-21
package schema

import (
	"encoding/json"
	"testing"
)

func TestChatRequestSemanticValidation(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		valid bool
	}{
		{name: "text conversation", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`, valid: true},
		{name: "developer role", body: `{"model":"gpt-4o","messages":[{"role":"developer","content":"rules"}]}`, valid: true},
		{name: "image content", body: `{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image_url","image_url":{"url":"https://example.test/a.png","detail":"high"}}]}]}`, valid: true},
		{name: "tool declaration", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"call it"}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],"tool_choice":"auto"}`, valid: true},
		{name: "stop array", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"stop":["END","DONE"]}`, valid: true},
		{name: "unknown field compatibility", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"user":"playground"}`, valid: true},
		{name: "arbitrary role", body: `{"model":"gpt-4o","messages":[{"role":"banana","content":"hello"}]}`, valid: false},
		{name: "empty text content", body: `{"model":"gpt-4o","messages":[{"role":"user","content":""}]}`, valid: false},
		{name: "part without type", body: `{"model":"gpt-4o","messages":[{"role":"user","content":[{"text":"hello"}]}]}`, valid: false},
		{name: "unknown part", body: `{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"banana","text":"hello"}]}]}`, valid: false},
		{name: "image without url", body: `{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"image_url","image_url":{}}]}]}`, valid: false},
		{name: "invalid stop object", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"stop":{"value":"END"}}`, valid: false},
		{name: "invalid reasoning effort", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"banana"}`, valid: false},
		// The body door must admit every word the reasoning engine itself acts
		// on, which is the LevelToBudget set plus the off and auto words, the
		// same vocabulary the model-name suffix accepts.
		{name: "xhigh effort", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"xhigh"}`, valid: true},
		{name: "max effort", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"max"}`, valid: true},
		{name: "none effort", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"none"}`, valid: true},
		{name: "off effort", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"off"}`, valid: true},
		{name: "auto effort", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"auto"}`, valid: true},
		// The suffix-only spellings stay refused on the body door: "ultra" is
		// normalized per model on the OpenAI wire alone, and a budget belongs
		// to the parenthesised form, not to the enum field.
		{name: "ultra effort stays suffix-only", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"ultra"}`, valid: false},
		{name: "budget effort stays suffix-only", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"1024"}`, valid: false},
		{name: "uppercase effort", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"HIGH"}`, valid: false},
		{name: "schema format without schema", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"response_format":{"type":"json_schema"}}`, valid: false},
		{name: "both token limits", body: `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"max_tokens":10,"max_completion_tokens":20}`, valid: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := DecodeChatRequest([]byte(tc.body))
			if err == nil {
				err = ValidateStruct(req)
			}
			if (err == nil) != tc.valid {
				t.Fatalf("validation error = %v, valid = %v", err, tc.valid)
			}
		})
	}
}

func TestChatRequestSemanticValidation_Boundaries(t *testing.T) {
	cases := []struct {
		name  string
		stop  string
		valid bool
	}{
		{name: "omitted stop", stop: "", valid: true},
		{name: "single stop", stop: `"END"`, valid: true},
		{name: "four stops", stop: `["a","b","c","d"]`, valid: true},
		{name: "empty stop", stop: `""`, valid: false},
		{name: "five stops", stop: `["a","b","c","d","e"]`, valid: false},
		{name: "numeric stop", stop: `42`, valid: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]`
			if tc.stop != "" {
				body += `,"stop":` + tc.stop
			}
			body += `}`
			req, err := DecodeChatRequest([]byte(body))
			if err == nil {
				err = ValidateStruct(req)
			}
			if (err == nil) != tc.valid {
				t.Fatalf("validation error = %v, valid = %v", err, tc.valid)
			}
		})
	}
}

func TestChatRequestSemanticValidation_JSONSchemaShape(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		valid bool
	}{
		{name: "object schema", raw: `{"type":"json_schema","json_schema":{"name":"answer","schema":{"type":"object"}}}`, valid: true},
		{name: "array schema", raw: `{"type":"json_schema","json_schema":{"name":"answer","schema":{"type":"array"}}}`, valid: true},
		{name: "schema is scalar", raw: `{"type":"json_schema","json_schema":{"name":"answer","schema":"text"}}`, valid: false},
		{name: "schema name missing", raw: `{"type":"json_schema","json_schema":{"schema":{"type":"object"}}}`, valid: false},
		{name: "text with schema block", raw: `{"type":"text","json_schema":{"name":"answer","schema":{"type":"object"}}}`, valid: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"response_format":` + tc.raw + `}`
			var req ChatRequest
			if err := json.Unmarshal([]byte(body), &req); err != nil {
				t.Fatalf("decode: %v", err)
			}
			err := ValidateStruct(req)
			if (err == nil) != tc.valid {
				t.Fatalf("validation error = %v, valid = %v", err, tc.valid)
			}
		})
	}
}
