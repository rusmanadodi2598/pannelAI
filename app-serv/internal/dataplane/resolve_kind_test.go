// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/resolve_kind_test.go
// @for       The kind guard: a model that is not a chat model is refused by the
//
//	chat plane rather than served with a chat body.
//
// @uses      context, testing, internal/registry.
// @reason    A provider entry may declare a model whose payload is its own
//
//	vocabulary rather than a chat request (the reference's
//	`kind: "systemone"`, served by a separate route). Measured before this
//	guard existed: `opencode/jev-1.13-free` resolved to target `openai`
//	and was served as a chat completion, so the chat body was sent to an
//	endpoint that answers a decision payload. Refusing by name is the
//	honest answer until that route exists, because a 404 the client can
//	read beats a request the upstream cannot parse.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestResolver_RefusesANonChatModel pins the guard: a declared model whose kind
// is not chat must not resolve for the chat data plane.
//
// The cases are the kinds the registry can carry: a systemone decision model,
// an image model, and an embedding model are all non-chat, while an unset kind
// and the two chat spellings the document uses must still resolve. The last
// case is the one that keeps the guard from over-refusing, which is the failure
// mode that would remove every ordinary model from the data plane.
func TestResolver_RefusesANonChatModel(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		wantErr bool
	}{
		{name: "no kind is chat", kind: "", wantErr: false},
		{name: "llm is chat", kind: "llm", wantErr: false},
		{name: "chat is chat", kind: "chat", wantErr: false},
		{name: "systemone is not chat", kind: "systemone", wantErr: true},
		{name: "image is not chat", kind: "image", wantErr: true},
		{name: "embedding is not chat", kind: "embedding", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index := kindRegistry{model: registry.Model{ID: "m", Kind: tc.kind}}
			resolver, err := NewResolver(index, fakeLookup{})
			if err != nil {
				t.Fatalf("NewResolver() error = %v", err)
			}
			resolution, err := resolver.Resolve(context.Background(), "p/m")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Resolve() served kind %q as chat (model %s, target %s), want a refusal",
						tc.kind, resolution.ModelID, resolution.Target)
				}
				if code := AsError(err).Code; code != CodeModelNotFound {
					t.Fatalf("Resolve() code = %q, want %q", code, CodeModelNotFound)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error = %v, want the model served", err)
			}
			if resolution.ModelID != "m" {
				t.Fatalf("ModelID = %q, want m", resolution.ModelID)
			}
		})
	}
}

// kindRegistry is a ProviderRegistry declaring one provider with one model, so
// this test states a kind and reads the answer without a wider fixture.
type kindRegistry struct{ model registry.Model }

func (r kindRegistry) Provider(name string) (registry.Provider, bool) {
	if name != "p" {
		return registry.Provider{}, false
	}
	return registry.Provider{
		ID:       "p",
		Category: "apikey",
		Transport: registry.Transport{
			BaseURL: "https://example.test/v1/chat/completions",
			Format:  registry.DefaultFormat,
		},
		Models: []registry.Model{r.model},
	}, true
}

func (r kindRegistry) Model(providerID, modelID string) (registry.Model, bool) {
	if providerID == "p" && modelID == r.model.ID {
		return r.model, true
	}
	return registry.Model{}, false
}

func (r kindRegistry) All() []registry.Provider {
	entry, _ := r.Provider("p")
	return []registry.Provider{entry}
}
