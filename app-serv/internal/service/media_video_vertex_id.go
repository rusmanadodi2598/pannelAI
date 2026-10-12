// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_vertex_id.go
// @for       The vertex video job id codec: an operation name spelled as one path segment, and the decode that refuses anything else.
// @uses      encoding/base64, regexp, strings, internal/dataplane.
// @reason    The operation name is a resource path and the poll route's id is one path segment, so the name travels base64url-encoded, and the decode refuses anything that would not re-encode to itself because a decoded name is spliced into a URL. It lives apart from media_video_vertex.go for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// The three shapes the job id codec reads: the client-facing spelling, the
// operation's own resource path, and the characters one path segment may
// carry.
var (
	vertexJobIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	vertexOperationPattern = regexp.MustCompile(`^projects/[^/]+/locations/[^/]+/publishers/[^/]+/models/[^/]+/operations/[^/]+$`)
	vertexSegmentPattern   = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// vertexJobIDLimit bounds a client-typed job id: an operation name has no
// business being longer, and the reference refuses the same way
// (videoProviders/vertex.js:30).
const vertexJobIDLimit = 1024

// vertexModelPath is the part of an operation name a fetchPredictOperation
// call addresses: everything before the operation's own segment. The name has
// already been anchored by the decode, so that segment is present exactly
// once and the cut leaves the model path.
func vertexModelPath(operation string) string {
	path, _, _ := strings.Cut(operation, "/operations/")
	return path
}

// encodeVertexJobID spells an operation name as the job id a client polls
// with. A resource path carries slashes and the poll route's id is one path
// segment, so the name travels base64url-encoded
// (videoProviders/vertex.js:15).
func encodeVertexJobID(operation string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(operation))
}

// decodeVertexJobID turns the client-facing job id back into the operation
// name, refusing anything that could address something other than the
// operation's own model path.
//
// A permissive decoder drops invalid characters silently, so an id is accepted
// only when it re-encodes to itself byte for byte; and because the decoded
// name is spliced into a URL path, its shape is anchored and every one of its
// segments must be a plain resource name (the reference's decodeJobId,
// videoProviders/vertex.js:26-34).
func decodeVertexJobID(id string) (string, error) {
	raw := strings.TrimSpace(id)
	if raw == "" || len(raw) > vertexJobIDLimit || !vertexJobIDPattern.MatchString(raw) {
		return "", vertexJobRefusal()
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != raw {
		return "", vertexJobRefusal()
	}
	operation := string(decoded)
	if !vertexOperationPattern.MatchString(operation) {
		return "", vertexJobRefusal()
	}
	for _, segment := range strings.Split(operation, "/") {
		if !vertexSegment(segment) {
			return "", vertexJobRefusal()
		}
	}
	return operation, nil
}

// vertexJobRefusal is the one answer an unusable job id gets, so the client is
// told what to fix without being told how the decoder reads.
func vertexJobRefusal() error {
	return dataplane.ValidationError("the vertex video job id is not usable")
}

// vertexSegment reports whether one value may be spliced into a URL path as a
// segment. The two dot names are refused on their own: they pass the character
// class, and a path walk would rewrite the URL rather than address the
// resource.
func vertexSegment(value string) bool {
	if value == "." || value == ".." {
		return false
	}
	return vertexSegmentPattern.MatchString(value)
}
