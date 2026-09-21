// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_contract_test.go
// @for       Structural validation of the served OpenAPI contract.
// @uses      encoding/json, strings, testing.
// @reason    SPEC-API-001 §7.17 serves the document as the contract of record
//
//	for app-ui and the CLI tools, and AGENTS.md §2.4 makes that contract
//	the source an implementation follows. A document that parses but
//	carries an unresolvable reference, a missing response, or a path
//	parameter a client cannot fill would still be served, so those rules
//	are asserted here rather than left to review. The per-plane error
//	envelope rule lives in openapi_error_test.go.
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

// contractDocument is the subset of the document these tests walk. Members are
// decoded as raw messages because the checks are structural: a schema is only
// ever followed by reference, never interpreted.
type contractDocument struct {
	OpenAPI    string                  `json:"openapi"`
	Paths      map[string]contractPath `json:"paths"`
	Components contractComponents      `json:"components"`
}

type contractPath map[string]contractOperation

type contractOperation struct {
	OperationID string                       `json:"operationId"`
	Security    []map[string]json.RawMessage `json:"security"`
	Parameters  []contractParameter          `json:"parameters"`
	RequestBody json.RawMessage              `json:"requestBody"`
	Responses   map[string]json.RawMessage   `json:"responses"`
}

type contractParameter struct {
	Name     string          `json:"name"`
	In       string          `json:"in"`
	Required bool            `json:"required"`
	Schema   json.RawMessage `json:"schema"`
}

type contractComponents struct {
	Schemas   map[string]json.RawMessage `json:"schemas"`
	Responses map[string]json.RawMessage `json:"responses"`
}

// loadContract decodes the served document and pins the version a consumer
// generates against.
func loadContract(t *testing.T) contractDocument {
	t.Helper()
	var doc contractDocument
	if err := json.Unmarshal(OpenAPIDocument(), &doc); err != nil {
		t.Fatalf("decoding the served contract: %v", err)
	}
	if doc.OpenAPI != "3.1.0" {
		t.Fatalf("openapi = %q, want 3.1.0", doc.OpenAPI)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("the contract declares no paths")
	}
	return doc
}

// TestOpenAPIContract_ReferencesResolve pins that every local reference points
// at a component the document defines. An unresolved reference is a broken
// document for every generated client, and it survives a structural parse
// because JSON does not resolve pointers.
func TestOpenAPIContract_ReferencesResolve(t *testing.T) {
	doc := loadContract(t)
	refs := collectRefs(t)
	if len(refs) == 0 {
		t.Fatal("the contract declares no references")
	}
	for target := range refs {
		name, kind, ok := splitRef(target)
		if !ok {
			t.Errorf("reference %q is not a local component reference", target)
			continue
		}
		switch kind {
		case "schemas":
			if _, exists := doc.Components.Schemas[name]; !exists {
				t.Errorf("reference %q resolves to no schema", target)
			}
		case "responses":
			if _, exists := doc.Components.Responses[name]; !exists {
				t.Errorf("reference %q resolves to no response", target)
			}
		default:
			t.Errorf("reference %q targets an unsupported component kind", target)
		}
	}
}

// TestOpenAPIContract_ComponentsAreUsed pins that no component is dead weight:
// an unreferenced schema or response is a definition a reader has to decide
// about, and it usually means a route was edited without removing its old shape.
func TestOpenAPIContract_ComponentsAreUsed(t *testing.T) {
	doc := loadContract(t)
	refs := collectRefs(t)
	for name := range doc.Components.Schemas {
		if !refs["#/components/schemas/"+name] {
			t.Errorf("schema %s is defined but never referenced", name)
		}
	}
	for name := range doc.Components.Responses {
		if !refs["#/components/responses/"+name] {
			t.Errorf("response %s is defined but never referenced", name)
		}
	}
}

// TestOpenAPIContract_OperationsComplete pins the shape every consumer needs:
// an operationId to generate a client method from, at least one success
// response, and at least one error response so a caller can handle failure.
func TestOpenAPIContract_OperationsComplete(t *testing.T) {
	doc := loadContract(t)
	seenIDs := make(map[string]string)
	for path, operations := range doc.Paths {
		if !strings.HasPrefix(path, "/api/v1/") {
			t.Errorf("path %s bypasses the /api/v1 prefix", path)
		}
		for method, operation := range operations {
			where := strings.ToUpper(method) + " " + path
			if operation.OperationID == "" {
				t.Errorf("%s declares no operationId", where)
			}
			if previous, ok := seenIDs[operation.OperationID]; ok && operation.OperationID != "" {
				t.Errorf("operationId %q is used by both %s and %s", operation.OperationID, previous, where)
			}
			seenIDs[operation.OperationID] = where
			if !hasSuccess(operation.Responses) {
				t.Errorf("%s declares no 2xx response", where)
			}
			if !hasError(operation.Responses) {
				t.Errorf("%s declares no 4xx or 5xx response", where)
			}
			assertPathParameters(t, where, path, operation.Parameters)
		}
	}
}

// TestOpenAPIContract_PathParametersAreTyped pins that a path parameter carries
// a schema, so a generated client knows the value is a string rather than an
// untyped placeholder.
func TestOpenAPIContract_PathParametersAreTyped(t *testing.T) {
	doc := loadContract(t)
	checked := 0
	for path, operations := range doc.Paths {
		for method, operation := range operations {
			where := strings.ToUpper(method) + " " + path
			for _, parameter := range operation.Parameters {
				if parameter.In != "path" {
					continue
				}
				if parameter.Schema == nil {
					t.Errorf("%s declares path parameter %q without a schema", where, parameter.Name)
					continue
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no path parameter was checked")
	}
}
