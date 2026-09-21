// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_error_test.go
// @for       The per-plane error envelope rule on the served contract.
// @uses      encoding/json, strings, testing.
// @reason    SPEC-API-001 §4 and §8 give the two planes different error
//
//	envelopes, and the panel's error enum is closed on the management
//	one: a management route leaking the OpenAI shape would surface as an
//	unparsable error. That rule spans every operation, so it is asserted
//	here rather than left to review. Splitting it from the structural
//	checks keeps both files inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"strings"
	"testing"
)

// TestOpenAPIContract_ErrorEnvelopeMatchesPlane pins the rule that makes one
// error shape per plane meaningful: an operation guarded by the session cookie
// answers management errors, and an operation guarded by a gateway key answers
// the OpenAI envelope. Public operations answer management errors too, because
// §8 fixes their failures and only the data plane speaks the OpenAI shape.
func TestOpenAPIContract_ErrorEnvelopeMatchesPlane(t *testing.T) {
	doc := loadContract(t)
	checked := 0
	for path, operations := range doc.Paths {
		for method, operation := range operations {
			where := strings.ToUpper(method) + " " + path
			want := wantEnvelope(operation.Security)
			if want == "" {
				t.Errorf("%s declares no recognized security scheme", where)
				continue
			}
			for status, raw := range operation.Responses {
				if !isErrorStatus(status) {
					continue
				}
				got := responseEnvelope(t, doc, raw)
				if got != want {
					t.Errorf("%s %s uses the %s envelope, want %s", where, status, got, want)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no error response was checked")
	}
}

// TestOpenAPIContract_PublicOperationsDeclareNoSecurity pins that the routes the
// spec calls public stay public in the document: the health probe, the version
// read, the login and status reads, and the OAuth callback the provider's
// browser redirect lands on.
func TestOpenAPIContract_PublicOperationsDeclareNoSecurity(t *testing.T) {
	doc := loadContract(t)
	public := []string{
		"GET /api/v1/health", "GET /api/v1/version", "POST /api/v1/auth/login",
		"GET /api/v1/auth/status", "GET /api/v1/providers/{provider_id}/oauth/callback",
	}
	for _, where := range public {
		method, path, found := strings.Cut(where, " ")
		if !found {
			t.Fatalf("test entry %q carries no method", where)
		}
		operation, ok := doc.Paths[path][strings.ToLower(method)]
		if !ok {
			t.Fatalf("%s is missing from the contract", where)
		}
		if len(operation.Security) != 0 {
			t.Errorf("%s declares security, want none", where)
		}
	}
}

// TestOpenAPIContract_DataPlaneRequiresGatewayKey pins the other half of the
// credential rule: a data-plane operation must present the bearer scheme, so a
// generated client cannot call it without a key.
func TestOpenAPIContract_DataPlaneRequiresGatewayKey(t *testing.T) {
	doc := loadContract(t)
	dataPlane := []string{
		"POST /api/v1/chat/completions", "POST /api/v1/messages", "POST /api/v1/responses",
		"GET /api/v1/models", "POST /api/v1/embeddings", "POST /api/v1/messages/count_tokens",
		"POST /api/v1/audio/speech", "POST /api/v1/audio/transcriptions", "GET /api/v1/audio/voices",
		"POST /api/v1/images/generations", "POST /api/v1/videos/generations", "POST /api/v1/search",
	}
	for _, where := range dataPlane {
		method, path, found := strings.Cut(where, " ")
		if !found {
			t.Fatalf("test entry %q carries no method", where)
		}
		operation, ok := doc.Paths[path][strings.ToLower(method)]
		if !ok {
			t.Fatalf("%s is missing from the contract", where)
		}
		if !declaresScheme(operation.Security, "gatewayKey") {
			t.Errorf("%s does not declare the gatewayKey scheme", where)
		}
	}
}

// TestOpenAPIContract_ManagementRequiresSession pins that a management mutation
// cannot be documented as callable without the dashboard session.
func TestOpenAPIContract_ManagementRequiresSession(t *testing.T) {
	doc := loadContract(t)
	management := []string{
		"GET /api/v1/gateway-keys", "POST /api/v1/gateway-keys", "GET /api/v1/providers",
		"POST /api/v1/endpoints", "POST /api/v1/combos", "GET /api/v1/usage/summary",
		"GET /api/v1/logs/requests", "GET /api/v1/settings", "GET /api/v1/skills",
		"GET /api/v1/openapi.json", "GET /api/v1/changelog",
	}
	for _, where := range management {
		method, path, found := strings.Cut(where, " ")
		if !found {
			t.Fatalf("test entry %q carries no method", where)
		}
		operation, ok := doc.Paths[path][strings.ToLower(method)]
		if !ok {
			t.Fatalf("%s is missing from the contract", where)
		}
		if !declaresScheme(operation.Security, "sessionCookie") {
			t.Errorf("%s does not declare the sessionCookie scheme", where)
		}
	}
}
