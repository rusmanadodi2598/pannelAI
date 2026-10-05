// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_opencode_free_floor_test.go
// @for       The declared model floor reaching the upstream through the real
//
//	registry and the real connector.
//
// @uses      context, encoding/json, testing, internal/schema.
// @reason    The unit tests of the connector hand it a model entry, and the unit
//
//	tests of the registry read the YAML back. Neither proves the two meet:
//	a floor typed into the wrong entry, or a rename that misses the floor,
//	leaves both green while the reported repro, a 60-token ceiling answered
//	with an empty body, still reaches the client. This drives the whole
//	pipeline once per model, on the same two wire-mates, so the difference
//	is the declaration and nothing else (draft 037 §13).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package dataplane

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestOpenCodeFree_DeclaredModelFloorReachesTheUpstream sends the ceiling the live
// combo test sent (60) through the free lane and reads what the upstream actually
// received. Muse Spark 1.3 declares 512 because it spends a smaller ceiling on
// thinking; the member beside it in that combo declares none, so the client's own
// number must still arrive unchanged, the floor is per model, not a provider-wide
// override (draft 037 §13).
func TestOpenCodeFree_DeclaredModelFloorReachesTheUpstream(t *testing.T) {
	cases := []struct {
		model    string
		wantCeil float64
	}{
		{model: "muse-spark-1.3-contributor-free", wantCeil: 512},
		{model: "mimo-v2.6-flash-free", wantCeil: 60},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			seen := []openCodeFreeCall{}
			server := openCodeFreeUpstream(t, &seen)
			engine := newOpenCodeFreeEngine(t, server.URL, &seen)

			raw := []byte(`{"model":"opencode/` + tc.model + `","messages":[{"role":"user",` +
				`"content":"Say pong only."}],"max_tokens":60,"stream":true}`)
			decoded, err := schema.DecodeChatRequest(raw)
			if err != nil {
				t.Fatalf("decoding the client body: %v", err)
			}
			request := Request{
				Route: RouteChatCompletions, ClientFormat: schema.FormatOpenAI,
				Model: decoded.Model, Chat: &decoded, Raw: raw, Stream: decoded.Stream,
			}
			if _, err := engine.Relay(context.Background(), request, &recordingSink{}); err != nil {
				t.Fatalf("Relay() error = %v", err)
			}
			if len(seen) != 1 {
				t.Fatalf("upstream calls = %d, want one", len(seen))
			}
			if got := openCodeFreeCeiling(t, seen[0].Body); got != tc.wantCeil {
				t.Fatalf("max_output_tokens = %v, want %v for %s", got, tc.wantCeil, tc.model)
			}
		})
	}
}

// openCodeFreeCeiling reads the output ceiling the upstream received, under
// whichever name the wire it arrived on carries: the connector renames a
// chat-shaped ceiling onto `max_output_tokens` only on the Responses wire.
func openCodeFreeCeiling(t *testing.T, body []byte) float64 {
	t.Helper()
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("the upstream received an undecodable body: %v", err)
	}
	for _, key := range []string{"max_output_tokens", "max_completion_tokens", "max_tokens"} {
		raw, present := decoded[key]
		if !present {
			continue
		}
		var ceiling float64
		if err := json.Unmarshal(raw, &ceiling); err != nil {
			t.Fatalf("%s is not a number (%s): %v", key, raw, err)
		}
		return ceiling
	}
	t.Fatalf("the upstream received no output ceiling: %s", body)
	return 0
}
