// Package dataplane implements the request-path data plane of app-serv.
//
// @file      internal/dataplane/engine_forced_stream_model_test.go
// @for       What a folded answer names as its model when the upstream calls every
//
//	model by one of its own labels.
//
// @uses      io, strings, testing, internal/registry.
// @reason    A provider that only answers streams is folded back into one body and
//
//	forwarded as written when the client speaks the same wire, so the model
//	name the client reads comes from the fold rather than from the
//	translation step — and Qoder writes `auto` into every chunk it sends.
//	That name is the provider's routing label: it is neither the model the
//	caller asked for nor one the caller can send back, because retrying it
//	reaches a pool the vendor answers 429 for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-28
package dataplane

import (
	"io"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

func TestFoldChatStreamNamesTheResolvedModel(t *testing.T) {
	cases := []struct {
		name     string
		modelID  string
		chunks   []string
		wantName string
		wantText string
	}{
		{
			name:    "a vendor alias cannot overwrite the resolved model",
			modelID: "qfmodel",
			chunks: []string{
				`{"id":"c1","model":"auto","choices":[{"index":0,"delta":{"content":"PO"}}]}`,
				`{"id":"c1","model":"auto","choices":[{"index":0,"delta":{"content":"NG"},"finish_reason":"stop"}]}`,
			},
			wantName: "qfmodel", wantText: "PONG",
		},
		{
			name:    "a vendor that names no model at all still answers with the resolved one",
			modelID: "qfmodel",
			chunks: []string{
				`{"id":"c1","choices":[{"index":0,"delta":{"content":"PONG"},"finish_reason":"stop"}]}`,
			},
			wantName: "qfmodel", wantText: "PONG",
		},
		{
			// Nothing was resolved, so the upstream's own name is the only one
			// there is — an empty `model` is not something a client can use.
			name:    "with nothing resolved the upstream's name is kept",
			modelID: "",
			chunks: []string{
				`{"id":"c1","model":"gpt-4o-2024-08-06","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":"stop"}]}`,
			},
			wantName: "gpt-4o-2024-08-06", wantText: "hi",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body strings.Builder
			for _, chunk := range tc.chunks {
				body.WriteString("data: " + chunk + "\n\n")
			}
			body.WriteString("data: [DONE]\n\n")

			upstream := &Upstream{Body: io.NopCloser(strings.NewReader(body.String()))}
			resolution := Resolution{
				Provider: registry.Provider{ID: "qoder"},
				ModelID:  tc.modelID,
				Target:   TargetOpenAI,
			}

			folded, _, err := foldStream(upstream, resolution)
			if err != nil {
				t.Fatalf("foldStream() error = %v", err)
			}
			answer := string(folded)
			if !strings.Contains(answer, `"model":"`+tc.wantName+`"`) {
				t.Fatalf("the folded answer must name %q, got: %s", tc.wantName, answer)
			}
			if !strings.Contains(answer, tc.wantText) {
				t.Fatalf("the folded answer must carry %q, got: %s", tc.wantText, answer)
			}
			if tc.modelID != "" && strings.Contains(answer, `"model":"auto"`) {
				t.Fatalf("the vendor's alias reached the caller: %s", answer)
			}
		})
	}
}
