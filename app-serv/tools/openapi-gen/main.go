// Package main generates the served OpenAPI document from the contract YAML.
//
// @file      tools/openapi-gen/main.go
// @for       Deterministic generation of the embedded OpenAPI JSON artifact.
// @uses      embed, flag, fmt, os, gopkg.in/yaml.v3.
// @reason    Schema-first contract work needs one repeatable command that turns
// the reviewed YAML into the file embedded by app-serv. Keeping generation in a
// small tool makes the source/artifact boundary explicit and lets CI reject a
// stale served document without making runtime code parse YAML.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-20
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// generationPaths are module-relative: the tool resolves the module root by
// walking up to the nearest go.mod, so it behaves the same from the module
// directory, from the repository root, and from a hook.
const (
	defaultInput  = "../docs/CONTRACT/001-CONTRACT-API-V1.yaml"
	defaultOutput = "internal/handler/openapi.json"
)

func main() {
	input := flag.String("input", defaultInput, "OpenAPI YAML source, relative to the module root")
	output := flag.String("output", defaultOutput, "generated OpenAPI JSON output, relative to the module root")
	check := flag.Bool("check", false, "fail when output differs from the generated document")
	flag.Parse()

	root, err := moduleRoot()
	if err != nil {
		fatal(err)
	}
	inputPath := filepath.Join(root, *input)
	outputPath := filepath.Join(root, *output)

	generated, err := generate(inputPath)
	if err != nil {
		fatal(err)
	}
	if *check {
		checkOutput(outputPath, generated)
		return
	}
	if err := os.WriteFile(outputPath, generated, 0o644); err != nil {
		fatal(fmt.Errorf("write %s: %w", outputPath, err))
	}
}

// moduleRoot walks up from the working directory to the nearest directory
// holding a go.mod, so the tool does not depend on where it was invoked.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolving the working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %s", dir)
		}
		dir = parent
	}
}

// generate parses the YAML and returns the stable JSON representation.
func generate(input string) ([]byte, error) {
	raw, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", input, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", input, err)
	}
	return encodeJSON(&document)
}

// checkOutput compares bytes, not decoded objects, because generated output is
// committed and deterministic formatting is part of the review surface.
func checkOutput(output string, expected []byte) {
	actual, err := os.ReadFile(output)
	if err != nil {
		fatal(fmt.Errorf("read %s: %w", output, err))
	}
	if !bytes.Equal(actual, expected) {
		fatal(fmt.Errorf("%s is stale; run go run ./tools/openapi-gen", output))
	}
}

// fatal reports a generation error and exits with a non-zero status.
func fatal(err error) {
	fmt.Fprintln(os.Stderr, "openapi-gen:", err)
	os.Exit(1)
}
