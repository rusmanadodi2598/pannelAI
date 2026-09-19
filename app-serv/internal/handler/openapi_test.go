// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_test.go
// @for       The embedded contract's structural invariants.
// @uses      encoding/json, strings, testing.
// @reason    §7.17 serves the document as the contract of record, so the tests
//
//	pin what a diffing consumer relies on: a declared OpenAPI
//	version, paths that all carry the §11.1 version prefix, and the
//	two auth schemes §4 names. Route coverage lives in the router
//	package, beside the mux the document must match.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenAPIDocument_Structure(t *testing.T) {
	var doc struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title string `json:"title"`
		} `json:"info"`
		Components struct {
			SecuritySchemes map[string]json.RawMessage `json:"securitySchemes"`
		} `json:"components"`
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(OpenAPIDocument(), &doc); err != nil {
		t.Fatalf("decoding the embedded document: %v", err)
	}
	if doc.OpenAPI == "" {
		t.Fatal("the document declares no OpenAPI version")
	}
	if doc.Info.Title == "" {
		t.Fatal("the document declares no title")
	}
	if len(doc.Paths) == 0 {
		t.Fatal("the document declares no paths")
	}
	for _, scheme := range []string{"sessionCookie", "gatewayKey"} {
		if _, ok := doc.Components.SecuritySchemes[scheme]; !ok {
			t.Fatalf("the document declares no %q security scheme", scheme)
		}
	}
	for path := range doc.Paths {
		if !strings.HasPrefix(path, "/api/v1/") {
			t.Fatalf("path %s bypasses the §11.1 version prefix", path)
		}
	}
}
