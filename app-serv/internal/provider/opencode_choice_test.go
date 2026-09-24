// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_choice_test.go
// @for       The tool_choice rule the OpenCode Responses wire enforces.
// @uses      testing, internal/registry.
// @reason    The Responses wire answers 400 to any tool_choice other than auto, so
//
//	the connector normalises whatever a client sent rather than forwarding
//	it to be rejected. The chat wire accepts `none`, so the rule is
//	per-wire and pinned on its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestOpenCode_TransformKeepsAChatWiresOwnToolChoice pins the half of the rule
// that is still unconditional: the chat wire accepts every value, so the
// connector must not rewrite one. The Responses half moved to
// opencode_responses_test.go when the reference's quirk list was ported, because
// the wire only refuses a non-auto choice on the models that declare the quirk.
func TestOpenCode_TransformKeepsAChatWiresOwnToolChoice(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name   string
		choice string
	}{
		{name: "none", choice: `"none"`},
		{name: "required", choice: `"required"`},
		{name: "a function object", choice: `{"type":"function","name":"my_tool"}`},
		{name: "auto", choice: `"auto"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"tool_choice":` + tc.choice + `}`
			request := Request{Model: registry.Model{ID: "big-pickle"}, Body: []byte(body)}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			var want any
			if err := json.Unmarshal([]byte(tc.choice), &want); err != nil {
				t.Fatalf("decoding the case's own choice: %v", err)
			}
			if got := decodeBody(t, request.Body)["tool_choice"]; !reflect.DeepEqual(got, want) {
				t.Fatalf("tool_choice = %#v, want the client's %#v kept on the chat wire", got, want)
			}
		})
	}
}
