// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/provider_auth_hint_test.go
// @for       The auth_hint read field the panel renders as the credential
//
//	format hint on the provider page (draft 036 slice B).
//
// @uses      testing, encoding/json, internal/registry.
// @reason    The registry's auth_hint tells the operator what credential
//
//	shape a provider wants (Qoder's "Personal Access Token (pt-...)")
//	before any endpoint exists. The hint text is served byte-verbatim
//	, it names upstream URLs the operator must visit, so a transcription
//	error here would send them to the wrong place, and stays absent
//	from the wire for providers that declare none.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-09-27
package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// qoderEntry carries the registry's own hint text byte-for-byte, including
// the Vietnamese preposition the generator copied verbatim from the
// reference (registry.yaml:687, known oddity kept on purpose).
const qoderHint = "Personal Access Token (pt-...) từ https://qoder.com/account/integrations"

// TestProviderResponseFrom_CarriesAuthHintVerbatim pins the hint's verbatim
// round trip and its absence for a provider that declares none.
func TestProviderResponseFrom_CarriesAuthHintVerbatim(t *testing.T) {
	summary := ProviderStatusSummaryDTO{Total: 0}
	hinted := ProviderResponseFrom(registry.Provider{
		ID: "qoder", Category: "oauth", Priority: 30, AuthHint: qoderHint,
	}, summary)

	raw, err := json.Marshal(hinted)
	if err != nil {
		t.Fatalf("marshal hinted provider: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, `"auth_hint":`+jsonQuote(t, qoderHint)) {
		t.Fatalf("auth_hint not carried verbatim; body = %s", body)
	}

	plain := ProviderResponseFrom(registry.Provider{
		ID: "openai", Category: "apikey", Priority: 10,
	}, summary)
	rawPlain, err := json.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain provider: %v", err)
	}
	if strings.Contains(string(rawPlain), "auth_hint") {
		t.Fatalf("auth_hint must be absent when the registry declares none; body = %s", rawPlain)
	}
}

// TestProviderDetailFrom_InheritsAuthHint pins the detail body carrying the
// same field through the embedded list shape.
func TestProviderDetailFrom_InheritsAuthHint(t *testing.T) {
	detail := ProviderDetailFrom(registry.Provider{
		ID: "qoder", Category: "oauth", Priority: 30, AuthHint: qoderHint,
		Transport: registry.Transport{Format: "openai"},
	}, ProviderStatusSummaryDTO{}, nil)

	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	if !strings.Contains(string(raw), `"auth_hint":`+jsonQuote(t, qoderHint)) {
		t.Fatalf("detail body lost auth_hint; body = %s", raw)
	}
}

// jsonQuote renders want as the exact JSON string literal the marshaler
// produces, so the verbatim assertion does not hand-escape non-ASCII.
func jsonQuote(t *testing.T, want string) string {
	t.Helper()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("quote %q: %v", want, err)
	}
	return string(encoded)
}
