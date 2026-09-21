// Package main generates the served OpenAPI document from the contract YAML.
//
// @file      tools/openapi-gen/encode_test.go
// @for       Table-driven checks for deterministic YAML-to-JSON encoding.
// @uses      strings, testing, gopkg.in/yaml.v3.
// @reason    The generator is the boundary between the schema-first YAML and
//
//	the JSON embedded by app-serv. A parser that silently changes scalar
//	types, object order, or aliases would produce a contract different from
//	the reviewed source, so the encoder is tested with varied YAML values
//	and nested structures rather than one fixture.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-20
package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEncodeJSON_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "scalar types",
			yaml: "name: contract\ncount: 3\nenabled: true\nmissing: null\n",
			want: "{\n  \"name\": \"contract\",\n  \"count\": 3,\n  \"enabled\": true,\n  \"missing\": null\n}\n",
		},
		{
			name: "nested sequence",
			yaml: "paths:\n  - /api/v1/health\n  - /api/v1/version\nmeta:\n  version: v1\n",
			want: "{\n  \"paths\": [\n    \"/api/v1/health\",\n    \"/api/v1/version\"\n  ],\n  \"meta\": {\n    \"version\": \"v1\"\n  }\n}\n",
		},
		{
			name: "quoted number remains string",
			yaml: "number_as_text: \"42\"\nnumber: 42\n",
			want: "{\n  \"number_as_text\": \"42\",\n  \"number\": 42\n}\n",
		},
		{
			name: "empty collections",
			yaml: "paths: {}\nservers: []\n",
			want: "{\n  \"paths\": {},\n  \"servers\": []\n}\n",
		},
		{
			name: "declared alias resolves",
			yaml: "base: &base\n  type: string\ncopy: *base\n",
			want: "{\n  \"base\": {\n    \"type\": \"string\"\n  },\n  \"copy\": {\n    \"type\": \"string\"\n  }\n}\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var document yaml.Node
			if err := yaml.Unmarshal([]byte(test.yaml), &document); err != nil {
				t.Fatalf("parsing YAML fixture: %v", err)
			}
			got, err := encodeJSON(&document)
			if err != nil {
				t.Fatalf("encoding YAML fixture: %v", err)
			}
			if string(got) != test.want {
				t.Fatalf("encoded JSON = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEncodeJSON_ProducesValidJSON(t *testing.T) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte("description: |\n  line one\n  line two\n"), &document); err != nil {
		t.Fatalf("parsing block scalar: %v", err)
	}
	encoded, err := encodeJSON(&document)
	if err != nil {
		t.Fatalf("encoding block scalar: %v", err)
	}
	if !strings.Contains(string(encoded), `"description": "line one\nline two\n"`) {
		t.Fatalf("encoded block scalar = %q, want the JSON string with newlines", encoded)
	}
}
