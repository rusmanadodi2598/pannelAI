// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_body.go
// @for       Decoding helpers for the routes whose body may legitimately be absent,
//
//	and the required path-parameter reader.
//
// @uses      encoding/json, errors, internal/domain, internal/schema, io, net/http.
// @reason    §7.5 gives the endpoint test route one optional field and §7.4 gives the
//
//	node test route the same, so a caller may post nothing at all and must
//	still be served. schema.DecodeJSON rejects an absent body by design
//	(every other management body is required), which is why the optional
//	case needs its own entry point here rather than a relaxation of that
//	rule. These are declared I/O boundaries (AGENTS.md §1.4).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// pathValue reads a required path parameter, answering with the §8 envelope when it
// is absent.
func pathValue(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	value := r.PathValue(name)
	if value == "" {
		schema.WriteError(w, domain.NewValidationError(name+" is required"))
		return "", false
	}
	return value, true
}

// decodeOptionalTestEndpoint decodes the endpoint test route's body, treating an
// absent one as "test the default key".
//
// An empty stream is valid here, but a malformed one is still refused: accepting
// garbage as "no body" would silently ignore a client's typo, which is exactly what
// the §2.4 contract discipline exists to catch.
func decodeOptionalTestEndpoint(w http.ResponseWriter, r *http.Request) (schema.TestEndpointRequest, bool) {
	var req schema.TestEndpointRequest
	if r.Body == nil {
		return req, true
	}
	if err := decodeInto(r, &req); err != nil {
		if errors.Is(err, io.EOF) {
			return schema.TestEndpointRequest{}, true
		}
		schema.WriteError(w, domain.NewValidationError("invalid request body: "+cleanTail(err)))
		return schema.TestEndpointRequest{}, false
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return schema.TestEndpointRequest{}, false
	}
	return req, true
}

// decodeOptionalNodeTest decodes the node test route's body, treating an absent one
// as an unauthenticated probe. A node carries no credential of its own, so an
// operator may legitimately test one whose upstream needs none.
//
// It is a second concrete function rather than one generic helper because a generic
// version would need an `any` destination, which AGENTS.md §1.4 forbids outside a
// declared boundary function — and a boundary function is exactly what this pair is.
func decodeOptionalNodeTest(w http.ResponseWriter, r *http.Request) (schema.TestNodeRequest, bool) {
	var req schema.TestNodeRequest
	if r.Body == nil {
		return req, true
	}
	if err := decodeInto(r, &req); err != nil {
		if errors.Is(err, io.EOF) {
			return schema.TestNodeRequest{}, true
		}
		schema.WriteError(w, domain.NewValidationError("invalid request body: "+cleanTail(err)))
		return schema.TestNodeRequest{}, false
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return schema.TestNodeRequest{}, false
	}
	return req, true
}

// decodeInto reads a bounded body into a concrete decode target.
//
// The variadic destination on pgx's Scan and this parameter are the two shapes §1.4
// permits: the value is immediately decoded into a typed struct and validated by the
// caller, so it never propagates past this boundary.
func decodeInto(r *http.Request, dst any) error {
	return json.NewDecoder(io.LimitReader(r.Body, schema.MaxBodyBytes)).Decode(dst)
}

// cleanTail keeps a bounded, English tail of a codec error so the envelope never
// echoes a large or driver-specific message.
func cleanTail(err error) string {
	msg := err.Error()
	const limit = 128
	if len(msg) > limit {
		return msg[len(msg)-limit:]
	}
	return msg
}
