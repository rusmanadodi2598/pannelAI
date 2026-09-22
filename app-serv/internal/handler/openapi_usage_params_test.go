// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_usage_params_test.go
// @for       Query-parameter parity for the §7.12 usage reads and the shared
//
//	pagination parameters, as the served contract declares them.
//
// @uses      encoding/json, strings, testing.
// @reason    Draft 010 F7: the YAML declared from/to as bare strings, the
//
//	enum parameters (group_by, granularity, status) as bare strings,
//	and per_page with a maximum but no minimum or default, so a
//	consumer could not learn from the contract what the boundary
//	enforces. F6's owner decision (D3 = refuse) makes the pagination
//	bounds part of the documented behaviour, and these tests pin the
//	served document so a regeneration that loses a bound fails here
//	rather than at a client.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"encoding/json"
	"testing"
)

// usageParamSchema is the part of a query parameter's schema these tests read.
// Description is carried for the semantics tests (draft 010 F3/F8), which pin
// the sentences a consumer reads rather than a bound.
type usageParamSchema struct {
	Type        string   `json:"type"`
	Format      string   `json:"format"`
	Enum        []string `json:"enum"`
	Minimum     *int     `json:"minimum"`
	Maximum     *int     `json:"maximum"`
	Default     *int     `json:"default"`
	Description string   `json:"description"`
}

// decodeParamSchema reads one parameter's schema, failing when it is absent so
// a parameter that lost its schema cannot pass a bound check by omission.
func decodeParamSchema(t *testing.T, where string, param contractParameter) usageParamSchema {
	t.Helper()
	if param.Schema == nil {
		t.Fatalf("%s declares %q without a schema", where, param.Name)
	}
	var schema usageParamSchema
	if err := json.Unmarshal(param.Schema, &schema); err != nil {
		t.Fatalf("decoding the %q schema on %s: %v", param.Name, where, err)
	}
	return schema
}

// findUsageParam walks one operation's parameters and returns the named query
// parameter, failing when it is absent so a rename cannot pass silently.
func findUsageParam(t *testing.T, path, method, name string) (contractParameter, usageParamSchema) {
	t.Helper()
	doc := loadContract(t)
	operation, ok := doc.Paths[path][method]
	if !ok {
		t.Fatalf("%s %s is missing from the contract", method, path)
	}
	for _, param := range operation.Parameters {
		if param.Name == name && param.In == "query" {
			return param, decodeParamSchema(t, method+" "+path, param)
		}
	}
	t.Fatalf("%s %s declares no %q query parameter", method, path, name)
	return contractParameter{}, usageParamSchema{}
}

// TestOpenAPIContract_UsageWindowParamsAreDateTimes pins the RFC3339 rule the
// boundary enforces (draft 010 F7): a consumer reading the contract must learn
// that from/to are timestamps, not bare strings.
func TestOpenAPIContract_UsageWindowParamsAreDateTimes(t *testing.T) {
	reads := []string{
		"/api/v1/usage/summary",
		"/api/v1/usage/timeseries",
		"/api/v1/usage/records",
		"/api/v1/logs/requests",
	}
	for _, path := range reads {
		for _, name := range []string{"from", "to"} {
			t.Run(name+" on "+path, func(t *testing.T) {
				param, schema := findUsageParam(t, path, "get", name)
				if schema.Type != "string" {
					t.Fatalf("%s type = %q, want string", name, schema.Type)
				}
				if schema.Format != "date-time" {
					t.Fatalf("%s format = %q, want date-time (RFC3339, SPEC-API-001 §4)", name, schema.Format)
				}
				if param.Required {
					t.Fatalf("%s is required, want optional (the boundary applies a default window)", name)
				}
			})
		}
	}
}

// TestOpenAPIContract_UsageEnumParams pins the closed sets the boundary
// enforces: group_by, granularity on the reads that accept them, and the
// status set shared with the logs route (draft 010 F2, already closed).
func TestOpenAPIContract_UsageEnumParams(t *testing.T) {
	cases := []struct {
		path string
		name string
		want []string
	}{
		{path: "/api/v1/usage/summary", name: "group_by", want: []string{"provider", "model", "endpoint", "gateway_key"}},
		{path: "/api/v1/usage/summary", name: "granularity", want: []string{"hour", "day"}},
		{path: "/api/v1/usage/summary", name: "status", want: []string{"success", "error"}},
		{path: "/api/v1/usage/timeseries", name: "group_by", want: []string{"provider", "model", "endpoint", "gateway_key"}},
		{path: "/api/v1/usage/timeseries", name: "granularity", want: []string{"hour", "day"}},
		{path: "/api/v1/usage/timeseries", name: "status", want: []string{"success", "error"}},
		{path: "/api/v1/usage/records", name: "status", want: []string{"success", "error"}},
		{path: "/api/v1/logs/requests", name: "status", want: []string{"success", "error"}},
	}
	for _, tc := range cases {
		t.Run(tc.name+" on "+tc.path, func(t *testing.T) {
			_, schema := findUsageParam(t, tc.path, "get", tc.name)
			if len(schema.Enum) != len(tc.want) {
				t.Fatalf("%s enum = %v, want %v", tc.name, schema.Enum, tc.want)
			}
			for _, member := range tc.want {
				if !contains(schema.Enum, member) {
					t.Fatalf("%s enum = %v, want it to contain %q", tc.name, schema.Enum, member)
				}
			}
		})
	}
}

// TestOpenAPIContract_PaginationParamsAreBounded pins the pagination bounds on
// every management list route that declares page/per_page, so the contract
// states the same range the boundary enforces (draft 010 F6, owner decision
// D3: refusal, with the documented default).
func TestOpenAPIContract_PaginationParamsAreBounded(t *testing.T) {
	doc := loadContract(t)
	checked := 0
	for path, operations := range doc.Paths {
		for method, operation := range operations {
			for _, param := range operation.Parameters {
				if param.In != "query" {
					continue
				}
				where := method + " " + path
				switch param.Name {
				case "page":
					schema := decodeParamSchema(t, where, param)
					if schema.Type != "integer" {
						t.Errorf("%s: page type = %q, want integer", where, schema.Type)
					}
					if schema.Minimum == nil || *schema.Minimum != 1 {
						t.Errorf("%s: page minimum = %v, want 1", where, schema.Minimum)
					}
					checked++
				case "per_page":
					schema := decodeParamSchema(t, where, param)
					if schema.Minimum == nil || *schema.Minimum != 1 {
						t.Errorf("%s: per_page minimum = %v, want 1", where, schema.Minimum)
					}
					if schema.Maximum == nil || *schema.Maximum != 100 {
						t.Errorf("%s: per_page maximum = %v, want 100", where, schema.Maximum)
					}
					if schema.Default == nil || *schema.Default != 25 {
						t.Errorf("%s: per_page default = %v, want 25", where, schema.Default)
					}
					checked++
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no pagination parameter was checked")
	}
}
