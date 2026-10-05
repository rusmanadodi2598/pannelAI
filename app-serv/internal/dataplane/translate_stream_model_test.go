// Package dataplane implements the request-path data plane of app-serv.
//
// @file      internal/dataplane/translate_stream_model_test.go
// @for       Which model name a streamed answer reports: the one the caller asked
//
//	for, not the alias the upstream echoes.
//
// @uses      encoding/json, strings, testing.
// @reason    Qoder answers every model it serves as `auto`, and the gateway was
//
//	adopting that echo into the frames it forwards, so a client that asked
//	`qoder/qfmodel` was told its answer came from `auto`, a name the
//	client cannot re-send, and one that routes to a pool the vendor answers
//	429 for. StreamState already documents the rule in its own field
//	comment; these cases are what keep the implementation to it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenAIStreamReportsTheAskedModel(t *testing.T) {
	cases := []struct {
		name     string
		seed     string
		echoes   []string
		wantName string
		wantNot  string
	}{
		{
			name:     "a vendor alias cannot overwrite the model the caller asked for",
			seed:     "qfmodel",
			echoes:   []string{"auto"},
			wantName: "qfmodel", wantNot: "auto",
		},
		{
			name:     "a vendor that changes its alias mid-stream still does not name the answer",
			seed:     "qfmodel",
			echoes:   []string{"auto", "performance", "auto"},
			wantName: "qfmodel", wantNot: "performance",
		},
		{
			name:     "an upstream that names nothing leaves the requested id in place",
			seed:     "qfmodel",
			echoes:   []string{""},
			wantName: "qfmodel",
		},
		{
			// Nothing was asked for, so the upstream's name is the only answer
			// there is, and an empty `model` member is not one a client can use.
			name:     "with no requested id the upstream's own name is kept",
			seed:     "",
			echoes:   []string{"gpt-4o-2024-08-06"},
			wantName: "gpt-4o-2024-08-06",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", tc.seed, 1700000000, false)
			var frames [][]byte
			for _, echo := range tc.echoes {
				chunk := `{"id":"up-1","object":"chat.completion.chunk","created":1,` +
					`"choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`
				if echo != "" {
					chunk = strings.Replace(chunk, `"created":1,`, `"created":1,"model":"`+echo+`",`, 1)
				}
				frames = append(frames, state.Frames(TargetOpenAI, []byte(chunk))...)
			}
			rendered := frameText(append(frames, state.Finish()...))

			if strings.Contains(rendered, `"model":"`+tc.wantNot+`"`) && tc.wantNot != "" {
				t.Fatalf("the upstream alias reached the client: %s", rendered)
			}
			if !strings.Contains(rendered, `"model":"`+tc.wantName+`"`) {
				t.Fatalf("frames must name the answer %q, got: %s", tc.wantName, rendered)
			}
		})
	}
}

// TestOpenAIStreamModelIsStableAcrossFrames counts the distinct model names across
// the frames of one answer, which is what a client that reads the name from any
// frame would see.
func TestOpenAIStreamModelIsStableAcrossFrames(t *testing.T) {
	state := NewStreamState("", "qfmodel", 1700000000, false)
	names := map[string]bool{}
	for _, echo := range []string{"auto", "qfmodel", "auto"} {
		chunk := `{"id":"up-1","created":1,"model":"` + echo +
			`","choices":[{"index":0,"delta":{"content":"pong"}}]}`
		for _, frame := range state.Frames(TargetOpenAI, []byte(chunk)) {
			var decoded struct {
				Model string `json:"model"`
			}
			payload := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(frameText([][]byte{frame})), "data:"))
			if json.Unmarshal([]byte(payload), &decoded) != nil {
				continue
			}
			names[decoded.Model] = true
		}
	}
	if len(names) != 1 || !names["qfmodel"] {
		t.Fatalf("model names across the answer = %v, want only qfmodel", names)
	}
}
