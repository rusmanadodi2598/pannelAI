// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_test.go
// @for       The Responses-wire field rules the reference applies before a
//
//	request leaves for the OpenCode upstream.
//
// @uses      testing, internal/registry.
// @reason    Six byte-level differences were measured between this connector and
//
//	the reference's transformRequest, each one a request the upstream
//	refuses or answers differently: a tool_choice forced to auto on a model
//	that does not have the quirk, an absent store, a chat-shaped
//	reasoning_effort, an empty input array, an overlong call_id with an
//	object arguments, and a chat-shaped tool declaration. They are pinned
//	together because they share one decision (how far the connector may
//	rewrite a same-format body) and because the reference applies them in
//	one function.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// responsesConnector builds a connector whose entry declares the quirk the
// reference gives opencode, so a test can tell a quirk model from a plain one.
func responsesConnector() *OpenCode {
	entry := opencodeEntry("https://opencode.ai", registry.DefaultFormat)
	entry.Transport.Quirks.ForceAutoToolChoiceModels = []string{"muse-spark-1.3-contributor-free"}
	return NewOpenCode(entry)
}

// TestOpenCode_ResponsesForcesAutoOnlyForTheQuirkModel pins the first delta: the
// reference demotes tool_choice to auto only for the models its own registry
// lists (registry/opencode.js:22-24), because the wire accepts other values on
// the rest. Forcing it everywhere silently changes a client's request.
func TestOpenCode_ResponsesForcesAutoOnlyForTheQuirkModel(t *testing.T) {
	connector := responsesConnector()

	cases := []struct {
		name     string
		model    string
		choice   string
		wantAuto bool
	}{
		{name: "the quirk model is demoted from required", model: "muse-spark-1.3-contributor-free", choice: `"required"`, wantAuto: true},
		{name: "the quirk model is demoted from a function", model: "muse-spark-1.3-contributor-free", choice: `{"type":"function","name":"t"}`, wantAuto: true},
		{name: "the quirk model keeps auto", model: "muse-spark-1.3-contributor-free", choice: `"auto"`, wantAuto: true},
		{name: "a plain model keeps required", model: "muse-spark-1.2-contributor-free", choice: `"required"`, wantAuto: false},
		{name: "a plain model keeps a function choice", model: "muse-spark-1.2-contributor-free", choice: `{"type":"function","name":"t"}`, wantAuto: false},
		{name: "an absent choice becomes auto", model: "muse-spark-1.2-contributor-free", choice: "", wantAuto: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"` + tc.model + `","input":[]`
			if tc.choice != "" {
				body += `,"tool_choice":` + tc.choice
			}
			body += `}`
			request := Request{
				Model: registry.Model{ID: tc.model, TargetFormat: registry.FormatOpenAIResponses},
				Body:  []byte(body),
			}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			got, present := decodeBody(t, request.Body)["tool_choice"]
			if !present {
				t.Fatal("tool_choice is absent, want the wire to carry one")
			}
			if tc.wantAuto && got != "auto" {
				t.Fatalf("tool_choice = %v, want auto", got)
			}
			if !tc.wantAuto {
				// Deep-compare the decoded forms, because a tool_choice may be an
				// object and two decoded maps are not comparable with ==.
				var want any
				if err := json.Unmarshal([]byte(tc.choice), &want); err != nil {
					t.Fatalf("decoding the case's own choice: %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("tool_choice = %#v, want the client's %#v kept", got, want)
				}
			}
		})
	}
}
