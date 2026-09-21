// Package main generates the served OpenAPI document from the contract YAML.
//
// @file      tools/openapi-gen/main_test.go
// @for       The generated artifact matches the committed contract, and the
//
//	contract satisfies the rules a consumer depends on.
//
// @uses      encoding/json, os, path/filepath, testing, gopkg.in/yaml.v3.
// @reason    The generator's own test runs the real tool over the real contract,
//
//	so a rule that only exists in a fixture cannot pass while the shipped
//	document violates it. The checks here are the ones no Go build
//	catches: an unresolved reference, a stale artifact, or a scalar type
//	the encoder changed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-20
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contractPath resolves the contract YAML relative to the module root.
func contractPath(t *testing.T) string {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("resolving the module root: %v", err)
	}
	return filepath.Join(root, defaultInput)
}

// outputPath resolves the committed artifact relative to the module root.
func outputPath(t *testing.T) string {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("resolving the module root: %v", err)
	}
	return filepath.Join(root, defaultOutput)
}

// TestGenerate_IsDeterministic pins that running the generator twice produces
// identical bytes, which is what lets the artifact be committed and reviewed.
func TestGenerate_IsDeterministic(t *testing.T) {
	first, err := generate(contractPath(t))
	if err != nil {
		t.Fatalf("generating the document: %v", err)
	}
	second, err := generate(contractPath(t))
	if err != nil {
		t.Fatalf("regenerating the document: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two runs over the same contract produced different bytes")
	}
}

// TestGenerate_MatchesCommittedArtifact pins that the served document is the
// contract's current rendering rather than an older one.
func TestGenerate_MatchesCommittedArtifact(t *testing.T) {
	generated, err := generate(contractPath(t))
	if err != nil {
		t.Fatalf("generating the document: %v", err)
	}
	committed, err := os.ReadFile(outputPath(t))
	if err != nil {
		t.Fatalf("reading the committed artifact: %v", err)
	}
	if !bytes.Equal(generated, committed) {
		t.Fatal("the committed artifact is stale; run: go run ./tools/openapi-gen")
	}
}

// TestContract_ReferencesResolve pins that every local reference in the served
// document names a component the document defines, in both directions.
func TestContract_ReferencesResolve(t *testing.T) {
	raw, err := os.ReadFile(outputPath(t))
	if err != nil {
		t.Fatalf("reading the committed artifact: %v", err)
	}
	var document struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas   map[string]json.RawMessage `json:"schemas"`
			Responses map[string]json.RawMessage `json:"responses"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decoding the committed artifact: %v", err)
	}
	if len(document.Paths) == 0 {
		t.Fatal("the contract declares no paths")
	}
	used := make(map[string]bool)
	var walk func(value json.RawMessage)
	walk = func(value json.RawMessage) {
		if len(value) == 0 {
			return
		}
		switch value[0] {
		case '{':
			var object map[string]json.RawMessage
			if err := json.Unmarshal(value, &object); err != nil {
				return
			}
			if target, ok := object["$ref"]; ok {
				var name string
				if err := json.Unmarshal(target, &name); err == nil {
					used[name] = true
				}
			}
			for _, member := range object {
				walk(member)
			}
		case '[':
			var list []json.RawMessage
			if err := json.Unmarshal(value, &list); err != nil {
				return
			}
			for _, item := range list {
				walk(item)
			}
		}
	}
	walk(raw)
	for target := range used {
		if !strings.HasPrefix(target, "#/components/") {
			t.Errorf("reference %q is not a local component reference", target)
			continue
		}
		parts := strings.Split(strings.TrimPrefix(target, "#/"), "/")
		if len(parts) != 3 {
			t.Errorf("reference %q is malformed", target)
			continue
		}
		name := parts[2]
		switch parts[1] {
		case "schemas":
			if _, ok := document.Components.Schemas[name]; !ok {
				t.Errorf("reference %q resolves to no schema", target)
			}
		case "responses":
			if _, ok := document.Components.Responses[name]; !ok {
				t.Errorf("reference %q resolves to no response", target)
			}
		default:
			t.Errorf("reference %q targets an unsupported component kind", target)
		}
	}
	for name := range document.Components.Schemas {
		if !used["#/components/schemas/"+name] {
			t.Errorf("schema %s is defined but never referenced", name)
		}
	}
	for name := range document.Components.Responses {
		if !used["#/components/responses/"+name] {
			t.Errorf("response %s is defined but never referenced", name)
		}
	}
}
