// Package main generates the served OpenAPI document from the contract YAML.
//
// @file      tools/openapi-gen/encode.go
// @for       YAML node tree to JSON, preserving the order the contract declares.
// @uses      bytes, encoding/json, gopkg.in/yaml.v3, strconv.
// @reason    The contract is the source of truth (AGENTS.md §2.4 CDD), and a
// generated artifact has to be byte-stable: a JSON object rebuilt from a Go map
// would reorder members on every run and turn a one-line contract edit into a
// whole-file diff. Walking the YAML node tree keeps the author's order, which is
// the order a reader of the served document sees.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-20
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// indentUnit is one level of the generated JSON's indentation.
const indentUnit = "  "

// encodeJSON renders the contract as indented JSON with a trailing newline.
func encodeJSON(document *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeNode(&buf, document, 0); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// writeNode dispatches on the YAML node kind. Alias nodes are resolved in
// place, so a shared definition cannot be emitted as a pointer a JSON reader
// would not understand.
func writeNode(buf *bytes.Buffer, node *yaml.Node, depth int) error {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return fmt.Errorf("line %d: a document must hold exactly one root node", node.Line)
		}
		return writeNode(buf, node.Content[0], depth)
	case yaml.MappingNode:
		return writeMapping(buf, node, depth)
	case yaml.SequenceNode:
		return writeSequence(buf, node, depth)
	case yaml.ScalarNode:
		return writeScalar(buf, node)
	case yaml.AliasNode:
		if node.Alias == nil {
			return fmt.Errorf("line %d: alias %q has no target", node.Line, node.Value)
		}
		return writeNode(buf, node.Alias, depth)
	default:
		return fmt.Errorf("line %d: unsupported YAML node kind %d", node.Line, node.Kind)
	}
}

// writeMapping writes one JSON object in declaration order.
func writeMapping(buf *bytes.Buffer, node *yaml.Node, depth int) error {
	if len(node.Content) == 0 {
		buf.WriteString("{}")
		return nil
	}
	buf.WriteString("{\n")
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if key.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: an object key must be a scalar", key.Line)
		}
		writeIndent(buf, depth+1)
		encoded, err := marshalString(key.Value)
		if err != nil {
			return fmt.Errorf("line %d: encoding key %q: %w", key.Line, key.Value, err)
		}
		buf.Write(encoded)
		buf.WriteString(": ")
		if err := writeNode(buf, node.Content[i+1], depth+1); err != nil {
			return err
		}
		if i+2 < len(node.Content) {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeIndent(buf, depth)
	buf.WriteByte('}')
	return nil
}

// writeSequence writes one JSON array in declaration order.
func writeSequence(buf *bytes.Buffer, node *yaml.Node, depth int) error {
	if len(node.Content) == 0 {
		buf.WriteString("[]")
		return nil
	}
	buf.WriteString("[\n")
	for i, item := range node.Content {
		writeIndent(buf, depth+1)
		if err := writeNode(buf, item, depth+1); err != nil {
			return err
		}
		if i+1 < len(node.Content) {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeIndent(buf, depth)
	buf.WriteByte(']')
	return nil
}

// writeScalar writes one JSON literal. The YAML tag decides the JSON type, so a
// quoted "1" in the contract stays a string and an unquoted 1 stays a number,
// which is the distinction a generated client depends on.
func writeScalar(buf *bytes.Buffer, node *yaml.Node) error {
	switch node.ShortTag() {
	case "!!null":
		buf.WriteString("null")
	case "!!bool":
		value, err := strconv.ParseBool(node.Value)
		if err != nil {
			return fmt.Errorf("line %d: %q is not a boolean", node.Line, node.Value)
		}
		buf.WriteString(strconv.FormatBool(value))
	case "!!int":
		return writeInteger(buf, node)
	case "!!float":
		value, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return fmt.Errorf("line %d: %q is not a number", node.Line, node.Value)
		}
		buf.WriteString(strconv.FormatFloat(value, 'g', -1, 64))
	default:
		encoded, err := marshalString(node.Value)
		if err != nil {
			return fmt.Errorf("line %d: encoding a string: %w", node.Line, err)
		}
		buf.Write(encoded)
	}
	return nil
}

// writeInteger writes a decimal integer. YAML accepts 0x and 0o prefixes that
// JSON does not, so the literal is parsed and re-rendered rather than copied.
func writeInteger(buf *bytes.Buffer, node *yaml.Node) error {
	if signed, err := strconv.ParseInt(node.Value, 0, 64); err == nil {
		buf.WriteString(strconv.FormatInt(signed, 10))
		return nil
	}
	unsigned, err := strconv.ParseUint(node.Value, 0, 64)
	if err != nil {
		return fmt.Errorf("line %d: %q is not an integer", node.Line, node.Value)
	}
	buf.WriteString(strconv.FormatUint(unsigned, 10))
	return nil
}

// writeIndent writes depth levels of indentation.
func writeIndent(buf *bytes.Buffer, depth int) {
	for i := 0; i < depth; i++ {
		buf.WriteString(indentUnit)
	}
}

// marshalString encodes one JSON string without the HTML escaping Go applies by
// default, so an angle bracket in a description stays readable in the diff.
func marshalString(value string) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
