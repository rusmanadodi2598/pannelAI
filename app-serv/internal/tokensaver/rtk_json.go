// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/rtk_json.go
// @for       The JSON helpers the walk uses: objects and arrays that keep their
//
//	members raw, and encoders that do not escape HTML.
//
// @uses      bytes, encoding/json.
// @reason    SPEC-API-002 §3 requires every member the engine does not rewrite to
//
//	survive byte for byte, which is why an untouched member is carried as
//	raw JSON rather than decoded. The encoders turn HTML escaping off
//	because a compressed blob is code, and json.Marshal would rewrite
//	its "<" and "&".
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"bytes"
	"encoding/json"
)

// object is one decoded JSON object whose members stay raw, so a member the
// engine does not rewrite is re-emitted byte for byte.
type object map[string]json.RawMessage

// decodeObject decodes a JSON object, reporting false for anything else. The
// opening byte is checked because json.Unmarshal accepts the null literal into
// a map, which would hand a walk an object it can write members into.
func decodeObject(raw []byte) (object, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false
	}
	decoded := object{}
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

// decodeArray decodes a JSON array of raw elements, reporting false for anything
// else. The opening byte is checked for the same reason as decodeObject: a null
// member would otherwise decode to an empty list, and a caller that appends to
// what it believes is a list would rewrite the null into one.
func decodeArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, false
	}
	var decoded []json.RawMessage
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

// memberString reads a string member.
func (o object) memberString(member string) (string, bool) {
	raw, ok := o[member]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// memberTrue reports whether a member is the JSON literal true.
func (o object) memberTrue(member string) bool {
	var value bool
	if err := json.Unmarshal(o[member], &value); err != nil {
		return false
	}
	return value
}

// hasMemberValue reports whether a member is a string equal to want.
func hasMemberValue(o object, member, want string) bool {
	value, ok := o.memberString(member)
	return ok && value == want
}

// encodable is the set of concrete JSON shapes the engine re-encodes. Naming the
// union keeps the encoder generic without an untyped value crossing the
// boundary.
type encodable interface {
	object | []json.RawMessage | string | textPart | systemMessage | headroomPayload
}

// marshalObject renders an object without HTML escaping, which json.Marshal
// applies by default and which would rewrite the "<" and "&" of a code blob.
func marshalObject(o object) ([]byte, error) { return marshalNoEscape(o) }

// marshalArray renders an array of raw elements.
func marshalArray(items []json.RawMessage) ([]byte, error) { return marshalNoEscape(items) }

// marshalString renders a JSON string.
func marshalString(value string) ([]byte, error) { return marshalNoEscape(value) }

// marshalNoEscape encodes a value with HTML escaping turned off.
func marshalNoEscape[T encodable](value T) ([]byte, error) {
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
