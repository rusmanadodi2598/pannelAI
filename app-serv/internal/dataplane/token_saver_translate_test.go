// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/token_saver_translate_test.go
// @for       Table-driven tests and shared helpers for Headroom translator tests.
// @uses      encoding/json, testing.
// @reason    SPEC-API-002 §8.2 must be pinned across every provider wire the
// dataplane dispatches, so translator Prepare and Restore assertions share
// helpers here to stay inside the source-file line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package dataplane

import (
	"encoding/json"
	"testing"
)

func translateObject(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode translated object: %v; raw=%s", err, raw)
	}
	return object
}

func translateString(t *testing.T, object map[string]json.RawMessage, member string) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(object[member], &value); err != nil {
		t.Fatalf("decode %s as string: %v", member, err)
	}
	return value
}

func translateArray(t *testing.T, object map[string]json.RawMessage, member string) []json.RawMessage {
	t.Helper()
	var values []json.RawMessage
	if err := json.Unmarshal(object[member], &values); err != nil {
		t.Fatalf("decode %s as array: %v", member, err)
	}
	return values
}

func translateContentText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		t.Fatalf("decode content as string or parts: %v", err)
	}
	out := ""
	for _, part := range parts {
		if part.Type == "text" && part.Text != "" {
			out += part.Text
		}
	}
	return out
}
