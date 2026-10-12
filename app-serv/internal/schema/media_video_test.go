// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_video_test.go
// @for       The video job contract: the two fields the create request is read for, and the model rewrite the upstream body gets.
// @uses      bytes, encoding/json, testing.
// @reason    SPEC-API-001 §7.10 serves one async video route, and both halves of it are silent when wrong: a request the gateway read a provider from but forwarded unrewritten reaches the upstream as a model id it does not know, and a body the rewrite mangled would drop the provider-specific fields the client sent.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-12
package schema

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestDecodeVideoRequest_ReadsTheTwoRoutingFields pins the contract the route
// validates: a model to resolve the provider from and a prompt to generate.
func TestDecodeVideoRequest_ReadsTheTwoRoutingFields(t *testing.T) {
	req, err := DecodeVideoRequest([]byte(`{"model":"xai/grok-imagine-video","prompt":"a paper boat","duration":8}`))
	if err != nil {
		t.Fatalf("DecodeVideoRequest() error = %v", err)
	}
	if req.Model != "xai/grok-imagine-video" || req.Prompt != "a paper boat" {
		t.Fatalf("DecodeVideoRequest() = %+v, want the model and the prompt", req)
	}
	if err := ValidateStruct(req); err != nil {
		t.Fatalf("ValidateStruct() error = %v, want the decoded body accepted", err)
	}
}

// TestDecodeVideoRequest_RefusesAMissingField pins the two required members.
func TestDecodeVideoRequest_RefusesAMissingField(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no model", `{"prompt":"a paper boat"}`},
		{"no prompt", `{"model":"xai/grok-imagine-video"}`},
		{"an empty prompt", `{"model":"xai/grok-imagine-video","prompt":""}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := DecodeVideoRequest([]byte(tc.body))
			if err != nil {
				t.Fatalf("DecodeVideoRequest() error = %v, want the body decoded for the field check", err)
			}
			if err := ValidateStruct(req); err == nil {
				t.Fatal("ValidateStruct() error = nil, want the incomplete body refused")
			}
		})
	}
}

// TestRewriteVideoModel_PrefixStrippedFieldsKept pins the rewrite: the upstream
// receives the id it knows, and every provider-specific member the client sent
// travels with it.
func TestRewriteVideoModel_PrefixStrippedFieldsKept(t *testing.T) {
	raw := []byte(`{"model":"xai/grok-imagine-video","prompt":"a paper boat","duration":8}`)
	got, err := RewriteVideoModel(raw, "grok-imagine-video")
	if err != nil {
		t.Fatalf("RewriteVideoModel() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("the rewritten body is not JSON: %v", err)
	}
	if decoded["model"] != "grok-imagine-video" {
		t.Fatalf("model = %v, want the upstream id", decoded["model"])
	}
	if decoded["prompt"] != "a paper boat" || decoded["duration"] != float64(8) {
		t.Fatalf("the rewritten body lost a member: %s", got)
	}
}

// TestRewriteVideoModel_MatchingModelStaysByteIdentical pins the reference's
// own rule: a body that already names the upstream model is forwarded exactly
// as it arrived, so the gateway's own re-encoding is not what the upstream
// reads.
func TestRewriteVideoModel_MatchingModelStaysByteIdentical(t *testing.T) {
	raw := []byte(`{"model":"grok-imagine-video",  "prompt":"a paper boat"}`)
	got, err := RewriteVideoModel(raw, "grok-imagine-video")
	if err != nil {
		t.Fatalf("RewriteVideoModel() error = %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("RewriteVideoModel() = %s, want the body unchanged", got)
	}
}

// TestRewriteVideoModel_RefusesABodyItCannotRewrite pins the two refusals: a
// body that is not a JSON object, and a model member that is not a string.
func TestRewriteVideoModel_RefusesABodyItCannotRewrite(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"an array", `[]`},
		{"a number model", `{"model":42,"prompt":"a paper boat"}`},
		{"a truncated object", `{"model":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RewriteVideoModel([]byte(tc.raw), "grok-imagine-video"); err == nil {
				t.Fatal("RewriteVideoModel() error = nil, want the body refused")
			}
		})
	}
}
