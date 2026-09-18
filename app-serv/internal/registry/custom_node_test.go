// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/custom_node_test.go
// @for       Table-driven tests for validating a custom node's identity, prefix, and base URL.
// @uses      testing, internal/registry.
// @reason    SPEC-API-001 §7.4 makes custom nodes P1, and a node's prefix is a

//	model-string namespace: a collision or a malformed base URL is a
//	routing fault, so those rules are pinned here rather than discovered
//	at call time.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import "testing"

func TestCustomNode_Validate(t *testing.T) {
	cases := []struct {
		name    string
		node    CustomNode
		wantErr bool
	}{
		{
			name: "openai compatible chat node",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "chat-1", Name: "My Corp", Prefix: "mycorp",
				APIType: "chat", BaseURL: "https://mycorp.test/v1",
			},
		},
		{
			name: "openai compatible responses node",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "responses-1", Name: "Resp", Prefix: "resp",
				APIType: "responses", BaseURL: "https://resp.test/v1",
			},
		},
		{
			name: "anthropic compatible node needs no api type",
			node: CustomNode{
				ID: AnthropicCompatiblePrefix + "1", Name: "Proxy", Prefix: "prox",
				BaseURL: "http://localhost:8081/v1",
			},
		},
		{
			name: "openai compatible node without api_type",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "2", Name: "Bad", Prefix: "bad",
				BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "openai compatible node with an unknown api_type",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "3", Name: "Bad", Prefix: "bad",
				APIType: "embedding", BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "anthropic compatible node carrying an api_type",
			node: CustomNode{
				ID: AnthropicCompatiblePrefix + "4", Name: "Bad", Prefix: "bad",
				APIType: "chat", BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "unknown id prefix",
			node: CustomNode{
				ID: "mystery", Name: "Bad", Prefix: "bad", BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "empty id",
			node: CustomNode{
				Name: "Bad", Prefix: "bad", APIType: "chat", BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "empty name",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "5", Prefix: "bad", APIType: "chat", BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "empty prefix",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "6", Name: "Bad", APIType: "chat", BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "prefix containing a slash",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "7", Name: "Bad", Prefix: "bad/prefix",
				APIType: "chat", BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "prefix containing whitespace",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "8", Name: "Bad", Prefix: "bad prefix",
				APIType: "chat", BaseURL: "https://bad.test/v1",
			},
			wantErr: true,
		},
		{
			name: "relative base url",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "9", Name: "Bad", Prefix: "bad",
				APIType: "chat", BaseURL: "/v1",
			},
			wantErr: true,
		},
		{
			name: "non-http base url",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "10", Name: "Bad", Prefix: "bad",
				APIType: "chat", BaseURL: "ftp://bad.test",
			},
			wantErr: true,
		},
		{
			name: "base url without a host",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "11", Name: "Bad", Prefix: "bad",
				APIType: "chat", BaseURL: "https://",
			},
			wantErr: true,
		},
		{
			name: "empty base url",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "12", Name: "Bad", Prefix: "bad", APIType: "chat",
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.node.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
