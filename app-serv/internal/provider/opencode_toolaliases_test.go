// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_toolaliases_test.go
// @for       The request half of the tool-name restore: the fingerprint pass reports the alias map, and the connector records it on the request.
// @uses      testing, internal/registry.
// @reason    The alias map is what lets the core restore a concealed caller tool in the answer, so it has to be recorded on the request the transport reads it from. A pass that aliases without reporting would leave the live failure unfixed while every other fingerprint test still passes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestOpenCode_TransformRecordsTheAliasesForTheResponseSide pins the seam's
// request half: a concealed caller tool is recorded as sent name -> caller's
// name on the request, and a request that renamed nothing records none.
func TestOpenCode_TransformRecordsTheAliasesForTheResponseSide(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name string
		body string
		want map[string]string
	}{
		{
			name: "a caller read with its own definition is recorded",
			body: `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],` +
				`"tools":[{"type":"function","function":{"name":"read","parameters":{"type":"object"}}}]}`,
			want: map[string]string{"read_client": "read"},
		},
		{
			name: "a taken alias steps aside and the stepped-aside name is recorded",
			body: `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],` +
				`"tools":[{"type":"function","function":{"name":"shell","parameters":{"type":"object"}}},` +
				`{"type":"function","function":{"name":"shell_client","parameters":{"type":"object"}}}]}`,
			want: map[string]string{"shell_client2": "shell"},
		},
		{
			name: "nothing renamed records nothing",
			body: `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],` +
				`"tools":[{"type":"function","function":{"name":"my_tool","parameters":{"type":"object"}}}]}`,
			want: map[string]string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{Model: registry.Model{ID: "big-pickle"}, Body: []byte(tc.body)}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			if len(request.ToolAliases) != len(tc.want) {
				t.Fatalf("ToolAliases = %v, want %v", request.ToolAliases, tc.want)
			}
			for alias, original := range tc.want {
				if request.ToolAliases[alias] != original {
					t.Fatalf("ToolAliases = %v, want %s -> %s", request.ToolAliases, alias, original)
				}
			}
		})
	}
}
