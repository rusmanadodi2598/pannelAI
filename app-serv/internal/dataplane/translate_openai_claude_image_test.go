// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_openai_claude_image_test.go
// @for       How an image reference is placed when a chat crosses to Anthropic.
// @uses      testing, internal/schema.
// @reason    A data URI and an http(s) URL are different contracts, one is inlined as base64 and the other is fetched by the upstream, so the branch is pinned on its own rather than inside the request table.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestOpenAIToClaude_ImageBlocks pins how an image reference is placed: a data URI
// becomes inline base64 and an http(s) URL stays a URL the upstream fetches.
func TestOpenAIToClaude_ImageBlocks(t *testing.T) {
	const dataURI = "data:image/png;base64,iVBORw0KGgo="
	cases := []struct {
		name      string
		url       string
		wantType  string
		wantMedia string
		wantURL   string
		wantData  string
	}{
		{name: "a png data URI becomes inline base64", url: dataURI, wantType: "base64", wantMedia: "image/png", wantData: "iVBORw0KGgo="},
		{name: "a jpeg data URI keeps its media type", url: "data:image/jpeg;base64,AAAA", wantType: "base64", wantMedia: "image/jpeg", wantData: "AAAA"},
		{name: "a remote https URL stays a URL", url: "https://example.test/a.png", wantType: "url", wantURL: "https://example.test/a.png"},
		{name: "a remote http URL stays a URL", url: "http://example.test/a.png", wantType: "url", wantURL: "http://example.test/a.png"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image_url","image_url":{"url":"` + tc.url + `"}}]}]}`
			got := OpenAIToClaude(chatRequest(t, body), "m", false)
			var image *schema.Block
			for i := range got.Messages[0].Content {
				if got.Messages[0].Content[i].Type == schema.BlockImage {
					image = &got.Messages[0].Content[i]
				}
			}
			if image == nil || image.Source == nil {
				t.Fatalf("no image block produced for %q: %+v", tc.url, got.Messages)
			}
			if image.Source.Type != tc.wantType {
				t.Fatalf("source type = %q, want %q", image.Source.Type, tc.wantType)
			}
			if image.Source.MediaType != tc.wantMedia {
				t.Fatalf("media_type = %q, want %q", image.Source.MediaType, tc.wantMedia)
			}
			if image.Source.URL != tc.wantURL {
				t.Fatalf("url = %q, want %q", image.Source.URL, tc.wantURL)
			}
			if image.Source.Data != tc.wantData {
				t.Fatalf("data = %q, want %q", image.Source.Data, tc.wantData)
			}
		})
	}
}
