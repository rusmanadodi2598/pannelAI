// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_helpers_test.go
// @for       The shared walkers the structural and per-plane contract tests use:
//
//	reference collection, envelope resolution, and scheme detection.
//
// @uses      encoding/json, testing.
// @reason    These helpers answer questions about the document rather than assert
//
//	about it, and three test files ask them. Keeping them apart means a
//	rule can be read without scrolling past the walkers it stands on,
//	and both files stay inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// collectRefs walks every raw message in the document and returns the set of
// reference targets it uses.
func collectRefs(t *testing.T) map[string]bool {
	t.Helper()
	refs := make(map[string]bool)
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
			if raw, ok := object["$ref"]; ok {
				var target string
				if err := json.Unmarshal(raw, &target); err == nil {
					refs[target] = true
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
	walk(json.RawMessage(OpenAPIDocument()))
	return refs
}

// wantEnvelope names the error envelope a security requirement implies. An
// operation with no requirement is one of the public management routes (§7.1,
// §7.2, and the OAuth callback in §7.4), and §8 fixes their failures in the
// management envelope: only the data plane speaks the OpenAI shape.
func wantEnvelope(security []map[string]json.RawMessage) string {
	if len(security) == 0 {
		return "ManagementError"
	}
	for _, requirement := range security {
		if _, ok := requirement["gatewayKey"]; ok {
			return "DataPlaneError"
		}
		if _, ok := requirement["sessionCookie"]; ok {
			return "ManagementError"
		}
	}
	return ""
}

// declaresScheme reports whether a security requirement list names a scheme.
func declaresScheme(security []map[string]json.RawMessage, scheme string) bool {
	for _, requirement := range security {
		if _, ok := requirement[scheme]; ok {
			return true
		}
	}
	return false
}

// responseEnvelope follows a response to the schema name behind it.
func responseEnvelope(t *testing.T, doc contractDocument, raw json.RawMessage) string {
	t.Helper()
	var response map[string]json.RawMessage
	if err := json.Unmarshal(raw, &response); err != nil {
		return ""
	}
	if ref, ok := response["$ref"]; ok {
		var target string
		if err := json.Unmarshal(ref, &target); err != nil {
			return ""
		}
		name, kind, ok := splitRef(target)
		if !ok || kind != "responses" {
			return ""
		}
		resolved, exists := doc.Components.Responses[name]
		if !exists {
			return ""
		}
		return responseEnvelope(t, doc, resolved)
	}
	var content map[string]struct {
		Schema struct {
			Ref string `json:"$ref"`
		} `json:"schema"`
	}
	if err := json.Unmarshal(response["content"], &content); err != nil {
		return ""
	}
	for _, media := range content {
		if media.Schema.Ref == "" {
			continue
		}
		name, _, ok := splitRef(media.Schema.Ref)
		if ok {
			return name
		}
	}
	return ""
}

// hasSuccess reports whether the operation declares a 2xx response.
func hasSuccess(responses map[string]json.RawMessage) bool {
	for status := range responses {
		if strings.HasPrefix(status, "2") {
			return true
		}
	}
	return false
}

// hasError reports whether the operation declares a 4xx or 5xx response.
func hasError(responses map[string]json.RawMessage) bool {
	for status := range responses {
		if isErrorStatus(status) {
			return true
		}
	}
	return false
}

// isErrorStatus reports whether a status string is a client or server error.
func isErrorStatus(status string) bool {
	return strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5")
}

// splitRef splits "#/components/<kind>/<name>" into its parts.
func splitRef(target string) (name, kind string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(target, "#/"), "/")
	if len(parts) != 3 || parts[0] != "components" {
		return "", "", false
	}
	return parts[2], parts[1], true
}

// assertPathParameters pins that every path template placeholder is declared as
// a required path parameter, and that no declared path parameter is absent from
// the template.
func assertPathParameters(t *testing.T, where, path string, parameters []contractParameter) {
	t.Helper()
	declared := make(map[string]bool)
	for _, parameter := range parameters {
		if parameter.In != "path" {
			continue
		}
		if !parameter.Required {
			t.Errorf("%s declares path parameter %q as optional", where, parameter.Name)
		}
		declared[parameter.Name] = true
	}
	for _, segment := range strings.Split(path, "/") {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		if !declared[name] {
			t.Errorf("%s does not declare path parameter %q", where, name)
		}
		delete(declared, name)
	}
	for name := range declared {
		t.Errorf("%s declares path parameter %q, which the path does not use", where, name)
	}
}
