// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_vertex_id_test.go
// @for       The vertex video job id codec: the round trip and every id shape the decode refuses.
// @uses      encoding/base64, strings, testing, and the vertexOperation/vertexJobID fixtures from media_video_vertex_test.go.
// @reason    The id codec is pinned for the reason the reference guards it: a decoded operation name is spliced into a URL path, so only ids that re-encode to themselves and carry the operation shape may pass. It lives apart from media_video_vertex_test.go for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestDecodeVertexJobID pins the codec the poll reads its task id through: the
// encoded operation round-trips, and every id that could address something
// other than the operation's own model path is refused.
func TestDecodeVertexJobID(t *testing.T) {
	canonical := vertexJobID(vertexOperation)
	// The operation is 109 bytes, one more than a multiple of three, so the
	// final character carries four spare bits: a second spelling of the same
	// bytes exists. The codec must refuse it, and the fixture itself is
	// checked first so a passing refusal proves the re-encode rule rather
	// than a shape accident.
	nonCanonical := canonical[:len(canonical)-1] + "f"
	if decoded, err := base64.RawURLEncoding.DecodeString(nonCanonical); err != nil || string(decoded) != vertexOperation {
		t.Fatalf("the fixture %q does not decode to the operation", nonCanonical)
	}

	cases := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "round-trips the operation name", id: canonical},
		{name: "empty", id: "", wantErr: true},
		{name: "not base64url", id: "a/b+c=", wantErr: true},
		{name: "decodes to a non-operation", id: vertexJobID("hello"), wantErr: true},
		{name: "an operation missing its model", id: vertexJobID("projects/p/locations/l/operations/op"), wantErr: true},
		{name: "an operation walking the path", id: vertexJobID("projects/../locations/us-central1/publishers/google/models/m/operations/op"), wantErr: true},
		{name: "an operation with a space in a segment", id: vertexJobID("projects/p/locations/us-central1/publishers/google/models/veo 3/operations/op"), wantErr: true},
		{name: "a non-canonical spelling of the operation", id: nonCanonical, wantErr: true},
		{name: "longer than the cap", id: strings.Repeat("A", 1025), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeVertexJobID(tc.id)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("decodeVertexJobID(%q) = %q, want a refusal", tc.id, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeVertexJobID() error = %v", err)
			}
			if got != vertexOperation {
				t.Fatalf("decodeVertexJobID() = %q, want the operation name", got)
			}
		})
	}
}
