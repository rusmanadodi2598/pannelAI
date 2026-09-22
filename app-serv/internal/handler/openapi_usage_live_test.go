// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_usage_live_test.go
// @for       The live Usage route's contract: the stream media type, the frame
//
//	shape a generated client reads, and the session requirement.
//
// @uses      encoding/json, sort, strings, testing.
// @reason    A generated client cannot read a stream it was not told about: an
//
//	operation documented only as `application/json` produces a client
//	that buffers, which turns a live view into one delayed blob. The
//	route is also the one Usage read whose response is not the §8
//	envelope, so the two facts that make it usable (the event-stream
//	media type and a frame schema) are pinned here rather than left to
//	review.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// liveSchemaRef is the schema reference one media type resolves to.
type liveSchemaRef struct {
	Schema struct {
		Ref string `json:"$ref"`
	} `json:"schema"`
}

// liveResponse is one declared response, reduced to the media types it carries
// and the schema reference behind each. The shared contractOperation keeps
// responses raw so its own structural checks can run without interpreting a
// schema, so this file decodes the one operation whose stream shape it pins.
type liveResponse struct {
	Content map[string]liveSchemaRef `json:"content"`
}

// liveResponsesOf reads one operation's response map by status code.
func liveResponsesOf(t *testing.T, operation contractOperation) map[string]liveResponse {
	t.Helper()
	raw, err := json.Marshal(operation.Responses)
	if err != nil {
		t.Fatalf("re-encoding the responses: %v", err)
	}
	var responses map[string]liveResponse
	if err := json.Unmarshal(raw, &responses); err != nil {
		t.Fatalf("decoding the responses: %v", err)
	}
	return responses
}

// mediaTypesOf lists the media types a response declares, sorted, so a failure
// names what was found instead of only what was missing.
func mediaTypesOf(response liveResponse) string {
	names := make([]string, 0, len(response.Content))
	for name := range response.Content {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// TestOpenAPIContract_LiveUsageDeclaresAnEventStream pins the media type and the
// frame schema. A client generated from this document has to know it is reading
// SSE, and it has to have a schema to decode each `data:` payload with.
func TestOpenAPIContract_LiveUsageDeclaresAnEventStream(t *testing.T) {
	doc := loadContract(t)
	path, ok := doc.Paths["/api/v1/usage/live"]
	if !ok {
		t.Fatal("the contract declares no /api/v1/usage/live path")
	}
	get, ok := path["get"]
	if !ok {
		t.Fatal("the live route declares no GET operation")
	}
	success, ok := liveResponsesOf(t, get)["200"]
	if !ok {
		t.Fatal("the live route declares no 200 response")
	}

	stream, ok := success.Content["text/event-stream"]
	if !ok {
		t.Fatalf("the live route's 200 declares %q, want text/event-stream", mediaTypesOf(success))
	}
	if stream.Schema.Ref != "#/components/schemas/UsageLiveFrame" {
		t.Fatalf("the stream schema is %q, want the frame schema", stream.Schema.Ref)
	}
	if _, ok := success.Content["application/json"]; ok {
		t.Fatal("the live route also declares application/json, which would tell a generated client to buffer")
	}
}

// TestOpenAPIContract_LiveUsageRequiresASession pins the credential: the route
// carries request identity, so it answers to the same session the four Usage
// reads beside it do.
func TestOpenAPIContract_LiveUsageRequiresASession(t *testing.T) {
	doc := loadContract(t)
	security := doc.Paths["/api/v1/usage/live"]["get"].Security
	if !declaresScheme(security, "sessionCookie") {
		t.Fatal("the live route does not declare the sessionCookie scheme")
	}
}

// TestOpenAPIContract_LiveFrameCarriesTheThreeFacts pins the frame's members
// against the three facts SPEC-UI §6.5 names, in both directions: a member the
// contract omits is one the panel cannot read, and a member it invents is one
// the gateway never sends.
func TestOpenAPIContract_LiveFrameCarriesTheThreeFacts(t *testing.T) {
	doc := loadContract(t)
	raw, ok := doc.Components.Schemas["UsageLiveFrame"]
	if !ok {
		t.Fatal("the contract declares no UsageLiveFrame schema")
	}
	var definition struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatalf("decoding the frame schema: %v", err)
	}
	want := map[string]bool{"active": true, "recent": true, "error_provider": true}
	for name := range want {
		if _, ok := definition.Properties[name]; !ok {
			t.Errorf("the frame schema is missing %q, which is one of the three facts §6.5 names", name)
		}
	}
	for name := range definition.Properties {
		if !want[name] {
			t.Errorf("the frame schema declares %q, which the gateway does not send", name)
		}
	}
	// Every member is required: the frame is full state, so an omitted list would
	// be read as "nothing to report" rather than "not stated".
	required := make(map[string]bool, len(definition.Required))
	for _, name := range definition.Required {
		required[name] = true
	}
	for name := range want {
		if !required[name] {
			t.Errorf("the frame schema does not require %q, so a client could not tell an omitted list from an empty one", name)
		}
	}
}

// TestOpenAPIContract_LiveListsAreArraysNotNull pins that neither list is
// nullable in the contract, because the gateway never sends null: Go marshals a
// nil slice that way, and the frame builder normalizes it so an empty set is
// spelled one way on the wire.
func TestOpenAPIContract_LiveListsAreArraysNotNull(t *testing.T) {
	doc := loadContract(t)
	raw := doc.Components.Schemas["UsageLiveFrame"]
	var definition struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatalf("decoding the frame schema: %v", err)
	}
	for _, name := range []string{"active", "recent"} {
		property, ok := definition.Properties[name]
		if !ok {
			t.Fatalf("the frame schema declares no %q", name)
		}
		if property.Type != "array" {
			t.Fatalf("%s is declared as %q, want an array: the gateway never sends null", name, property.Type)
		}
	}
}
