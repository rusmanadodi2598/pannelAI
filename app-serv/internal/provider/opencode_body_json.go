// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_body_json.go
// @for       The JSON member helpers the OpenCode request transform writes with.
// @uses      encoding/json.
// @reason    The transform builds members by hand because it has to preserve the
//
//	client's own body: decoding into a typed struct would drop every
//	member the gateway does not model, which for a coding agent is most
//	of them. These helpers keep that work readable and keep the transform
//	itself inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import "encoding/json"

// stringMember reads a string member, returning "" when it is absent or is not a
// string. The empty string is the answer in both cases because neither is a name
// the caller can act on.
func stringMember(body map[string]json.RawMessage, key string) string {
	raw, present := body[key]
	if !present {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

// mustRaw marshals a member map the connector built itself. A map of already
// encoded values cannot fail to marshal, so a failure would be a programming
// error rather than a request-path condition, and null is the harmless answer.
func mustRaw(value map[string]json.RawMessage) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("null")
	}
	return encoded
}

// mustRawString encodes a string member the connector built itself.
func mustRawString(value string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}

// mustRawList encodes an array of already encoded items.
func mustRawList(values []json.RawMessage) json.RawMessage {
	encoded, err := json.Marshal(values)
	if err != nil {
		return json.RawMessage("[]")
	}
	return encoded
}
