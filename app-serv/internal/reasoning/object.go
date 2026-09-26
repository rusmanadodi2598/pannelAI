// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/object.go
// @for       The decoded-object helpers the thinking application mutates a body
//
//	through.
//
// @uses      encoding/json.
// @reason    SPEC-API-001 §7.15 applies a thinking config to whichever wire the
//
//	upstream speaks, and the formats disagree about where the field lives:
//	a top-level member, a nested generationConfig, or the request envelope
//	the CLI payloads wrap theirs in. One set of helpers is what keeps the
//	format switch to the field each format actually writes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "encoding/json"

// decodeObject reads a request body as an object of raw members. A body that is
// not an object (or not JSON) is refused, which the caller reads as "nothing to
// do" rather than as a failure.
func decodeObject(raw []byte) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil || body == nil {
		return nil, false
	}
	return body, true
}

// objectField returns the object stored at key.
func objectField(body map[string]json.RawMessage, key string) (map[string]json.RawMessage, bool) {
	raw, ok := body[key]
	if !ok {
		return nil, false
	}
	var child map[string]json.RawMessage
	if err := json.Unmarshal(raw, &child); err != nil || child == nil {
		return nil, false
	}
	return child, true
}

// ensureObject returns the object at key, or an empty one when the key is
// absent, so a setter can create the path it writes.
func ensureObject(body map[string]json.RawMessage, key string) map[string]json.RawMessage {
	if child, ok := objectField(body, key); ok {
		return child
	}
	return map[string]json.RawMessage{}
}

// stringField returns the string stored at key, or "" when it is absent or not
// a string.
func stringField(body map[string]json.RawMessage, key string) string {
	raw, ok := body[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

// intField returns the number stored at key. The second result distinguishes an
// absent key from a zero value, which the budget formats read differently.
func intField(body map[string]json.RawMessage, key string) (int, bool) {
	raw, ok := body[key]
	if !ok {
		return 0, false
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, false
	}
	return value, true
}

// setValue stores a marshalled value at key.
func setValue(body map[string]json.RawMessage, key string, value any) {
	if encoded, err := json.Marshal(value); err == nil {
		body[key] = encoded
	}
}

// deleteField removes a member. It is the reference's `delete body.x`: an
// absent member is not an error, so a format that strips a field it never
// wrote is a no-op.
func deleteField(body map[string]json.RawMessage, key string) {
	delete(body, key)
}

// updateObject mutates the object at key when it exists, writing it back.
func updateObject(body map[string]json.RawMessage, key string, mutate func(map[string]json.RawMessage)) {
	child, ok := objectField(body, key)
	if !ok {
		return
	}
	mutate(child)
	body[key] = mustJSON(child)
}

// mustJSON marshals a value the caller built, so a member can be written
// without repeating the error branch. The values written here are strings,
// numbers, and objects of raw members, none of which can fail to marshal.
func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}
