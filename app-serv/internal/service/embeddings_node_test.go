// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings_node_test.go
// @for       The embeddings block a custom OpenAI-compatible node gets, and the
//
//	refusals that stay for the node kinds without such a service.
//
// @uses      internal/dataplane, internal/provider, internal/registry, context,
//
//	testing.
//
// @reason    A node is routable through the same pipeline as a registry
//
//	provider, so the synthesized block has to complete the URL and
//	place the credential exactly as the reference's openaiCompatNode
//	adapter does. Each row pins one shape of that rule, and the
//	Anthropic-compatible row pins the refusal that must not move.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// nodeOfKind builds the node under test and returns the synthesized provider
// entry, which is what the data plane hands to mediaConfig.
func nodeOfKind(t *testing.T, id, apiType, baseURL string) registry.Provider {
	t.Helper()
	node := registry.CustomNode{
		ID: id, Name: "Node Under Test", Prefix: "stubnode", APIType: apiType, BaseURL: baseURL,
	}
	entry, ok := nodeIndex(t, node).Provider(id)
	if !ok {
		t.Fatalf("Provider(%q) not found in the synthesized index", id)
	}
	return entry
}

// TestEmbeddingsService_NodeEmbeddingsMedia pins the synthesized block and the
// target it produces, including the credential placement the node's chat path
// also uses.
func TestEmbeddingsService_NodeEmbeddingsMedia(t *testing.T) {
	const openAINode = registry.OpenAICompatiblePrefix + "node1"
	cases := []struct {
		name          string
		id            string
		apiType       string
		baseURL       string
		wantURL       string
		wantErrorText string
	}{
		{
			name: "an OpenAI-compatible node", id: openAINode, apiType: "chat",
			baseURL: "http://127.0.0.1:8091/v1", wantURL: "http://127.0.0.1:8091/v1/embeddings",
		},
		{
			name: "a base URL that already names embeddings", id: openAINode, apiType: "chat",
			baseURL: "http://127.0.0.1:8091/v1/embeddings", wantURL: "http://127.0.0.1:8091/v1/embeddings",
		},
		{
			name: "a base URL with a trailing slash", id: openAINode, apiType: "chat",
			baseURL: "http://127.0.0.1:8091/v1/", wantURL: "http://127.0.0.1:8091/v1/embeddings",
		},
		{
			name: "a Responses-API node", id: openAINode, apiType: registry.OpenAITypeResponses,
			baseURL: "http://127.0.0.1:8091/v1", wantURL: "http://127.0.0.1:8091/v1/embeddings",
		},
		{
			name: "an Anthropic-compatible node", id: registry.AnthropicCompatiblePrefix + "node1",
			baseURL: "http://127.0.0.1:8091", wantErrorText: "does not offer embeddings",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &EmbeddingsService{}
			media, baseURL, err := svc.mediaConfig(context.Background(), nodeOfKind(t, tc.id, tc.apiType, tc.baseURL))
			if tc.wantErrorText != "" {
				if err == nil || !containsText(err.Error(), tc.wantErrorText) {
					t.Fatalf("error = %v, want it to mention %q", err, tc.wantErrorText)
				}
				return
			}
			if err != nil {
				t.Fatalf("mediaConfig() error = %v", err)
			}
			target, headers, err := dataplane.MediaTarget(media, baseURL, provider.Credential{APIKey: "sk-node"}, nil)
			if err != nil {
				t.Fatalf("MediaTarget() error = %v", err)
			}
			if target != tc.wantURL {
				t.Fatalf("target = %q, want %q", target, tc.wantURL)
			}
			if got := headers[provider.DefaultAuthHeader]; got != "Bearer sk-node" {
				t.Fatalf("%s = %q, want the node's key as a bearer token", provider.DefaultAuthHeader, got)
			}
		})
	}
}

// TestEmbeddingsService_NodeEmbeddingsDeclaredFormat pins the benign control
// for the format rule: a node pointed at a host that also serves a native
// protocol keeps the OpenAI shape, because the synthesized block declares it.
func TestEmbeddingsService_NodeEmbeddingsDeclaredFormat(t *testing.T) {
	const geminiOpenAI = "https://generativelanguage.googleapis.com/v1beta/openai"
	svc := &EmbeddingsService{}
	media, baseURL, err := svc.mediaConfig(context.Background(), nodeOfKind(t,
		registry.OpenAICompatiblePrefix+"node1", "chat", geminiOpenAI))
	if err != nil {
		t.Fatalf("mediaConfig() error = %v", err)
	}
	if dataplane.IsGeminiEmbedding(media) {
		t.Fatalf("media %+v was read as the native Gemini protocol", media)
	}
	if want := geminiOpenAI + "/embeddings"; baseURL != want {
		t.Fatalf("baseURL = %q, want %q", baseURL, want)
	}
}
