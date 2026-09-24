// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_contract_bool_test.go
// @for       The closed-set rule of boolean query parameters in the served
//
//	OpenAPI document.
//
// @uses      encoding/json, strings, testing.
// @reason    Draft 025 F5: the boundary refuses `?active=yes` with a
//
//	VALIDATION_ERROR, and the document a generated client reads must
//	carry the same closed set — a bare boolean would let a client fill
//	`1` and learn the rule only from a 400. Enumerating both spellings
//	on the parameter is what makes the two agree mechanically. The
//	enum is decoded after the type is read, because every string
//	parameter's enum holds strings and the one-struct decode would
//	fail on those instead of skipping them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-24
package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestOpenAPIContract_BooleanQueryParametersAreClosed pins that every boolean
// query parameter declares both spellings, so the document a generated client
// reads agrees with the boundary's closed set (draft 025 F5): a parameter the
// gateway refuses on `yes` must not be documented as a bare boolean a client
// could fill with `1`.
func TestOpenAPIContract_BooleanQueryParametersAreClosed(t *testing.T) {
	doc := loadContract(t)
	checked := 0
	for path, operations := range doc.Paths {
		for method, operation := range operations {
			where := strings.ToUpper(method) + " " + path
			for _, parameter := range operation.Parameters {
				if parameter.In != "query" || parameter.Schema == nil {
					continue
				}
				// The type is read first and the enum second: a string
				// parameter's enum holds strings, so decoding both in one
				// struct would fail on every other filter in the document
				// rather than skipping it.
				var kind struct {
					Type string `json:"type"`
				}
				if err := json.Unmarshal(parameter.Schema, &kind); err != nil {
					t.Errorf("%s parameter %q has an undecodable schema: %v", where, parameter.Name, err)
					continue
				}
				if kind.Type != "boolean" {
					continue
				}
				var schema struct {
					Enum []bool `json:"enum"`
				}
				if err := json.Unmarshal(parameter.Schema, &schema); err != nil {
					t.Errorf("%s boolean parameter %q has an undecodable enum: %v", where, parameter.Name, err)
					continue
				}
				checked++
				if len(schema.Enum) != 2 {
					t.Errorf("%s boolean parameter %q declares %d enum values, want true and false",
						where, parameter.Name, len(schema.Enum))
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no boolean query parameter was checked")
	}
}
