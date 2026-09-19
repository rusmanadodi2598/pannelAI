// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/token_saver_translate_json.go
// @for       The raw JSON boundaries used while restoring compressed saver output.
// @uses      bytes, encoding/json, internal/schema.
// @reason    The Headroom adapter must replace selected members without dropping
//
// fields the upstream wire carries but the typed translator does not model.
// These helpers keep that boundary explicit and keep the main pivot file
// below the source-file line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"bytes"
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// responsesInputIsSafe rejects any Responses item that is not a message.
func responsesInputIsSafe(raw json.RawMessage) bool {
	if !isTokenSaverArray(raw) {
		return false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return false
	}
	for _, item := range items {
		object, ok := decodeTokenSaverObject(item)
		if !ok {
			return false
		}
		kind, ok := stringTokenSaverMember(object, "type")
		if !ok || kind != schema.ResponsesItemMessage {
			return false
		}
	}
	return true
}

// decodeTokenSaverObject is this adapter's typed JSON boundary.
func decodeTokenSaverObject(raw []byte) (map[string]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, false
	}
	return object, true
}

// isTokenSaverArray reports whether raw is a JSON array.
func isTokenSaverArray(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return false
	}
	var array []json.RawMessage
	return json.Unmarshal(trimmed, &array) == nil
}

// stringTokenSaverMember reads a string discriminator from a raw object.
func stringTokenSaverMember(object map[string]json.RawMessage, member string) (string, bool) {
	value, ok := object[member]
	if !ok {
		return "", false
	}
	var decoded string
	if err := json.Unmarshal(value, &decoded); err != nil {
		return "", false
	}
	return decoded, true
}

// marshalTokenSaverObject keeps the no-HTML-escape rule used by the native
// engine, so an untouched sibling containing source code remains byte-safe.
func marshalTokenSaverObject(object map[string]json.RawMessage) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(object); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}
