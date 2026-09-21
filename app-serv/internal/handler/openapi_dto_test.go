// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_dto_test.go
// @for       Contract-to-DTO agreement on property names.
// @uses      encoding/json, reflect, sort, strings, testing, internal/schema.
// @reason    AGENTS.md §2.4 CDD requires the published contract to be validated
//
//	against the typed structs, and the two can drift silently: a renamed
//	json tag changes the wire while the YAML keeps describing the old
//	field. This test pins the property set of every schema whose name
//	matches a schema-package struct, so a rename fails here instead of at
//	a client that reads a field the gateway stopped sending. The registry
//	itself lives in openapi_dto_registry_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestOpenAPIContract_PropertyNamesMatchDTOs compares the contract's property
// set with the json tags of the struct that renders it, in both directions.
func TestOpenAPIContract_PropertyNamesMatchDTOs(t *testing.T) {
	doc := loadContract(t)
	registry := dtoRegistry()
	if len(registry) == 0 {
		t.Fatal("no DTO is registered")
	}
	for name, structType := range registry {
		raw, ok := doc.Components.Schemas[name]
		if !ok {
			t.Errorf("schema %s is registered but missing from the contract", name)
			continue
		}
		var definition struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(raw, &definition); err != nil {
			t.Errorf("decoding schema %s: %v", name, err)
			continue
		}
		want := jsonFieldNames(structType)
		got := make(map[string]bool, len(definition.Properties))
		for property := range definition.Properties {
			got[property] = true
		}
		for _, field := range want {
			if !got[field] {
				t.Errorf("schema %s is missing property %q from %s", name, field, structType.Name())
			}
		}
		for property := range got {
			if !contains(want, property) {
				t.Errorf("schema %s declares property %q, which %s does not render", name, property, structType.Name())
			}
		}
	}
}

// jsonFieldNames returns the wire names of a struct's exported fields, skipping
// the ones the struct marks as never serialized.
func jsonFieldNames(structType reflect.Type) []string {
	names := make([]string, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" {
			name = field.Name
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// contains reports whether a sorted slice holds a value.
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
