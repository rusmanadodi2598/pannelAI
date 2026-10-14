// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_choice_test.go
// @for       The tool_choice rule the OpenCode chat wire enforces.
// @uses      testing, internal/registry.
// @reason    The chat wire forwards whatever a client chose, including `none`: the connector only ever avoids *defaulting* to none, which upstream answers with 403 FreeTierError. The Responses half moved to opencode_responses_test.go when the reference's quirk list was ported, because the wire only refuses a non-auto choice on the models that declare the quirk.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-21
package provider

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestOpenCode_TransformKeepsAChatWiresOwnToolChoice pins the half of the rule
// that is still unconditional: the connector never rewrites a choice the client
// wrote, so every value survives the transform. It never *defaults* to "none"
// either (opencode_fingerprint_body_test.go), because upstream answers a
// defaulted none with 403 FreeTierError. The Responses half moved to
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
