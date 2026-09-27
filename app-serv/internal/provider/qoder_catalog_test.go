// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_catalog_test.go
// @for       The vendor catalogue read: its signature, and the lookup the body builder needs.
//
// @uses      encoding/json, io, net/http, net/http/httptest, strings, testing.
// @reason    The configuration the chat body carries comes from an authenticated read, so
//
//	what matters is that the read is signed as the account and that the
//	lookup finds a model wherever the vendor grouped it. Both are pinned here
//	against a stub serving the captured answer shape.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestQoderCatalogReadIsSigned proves the catalogue read is not a plain GET: it
// carries the same signature the chat call would, under the account's identity.
func TestQoderCatalogReadIsSigned(t *testing.T) {
	var seen http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case qoderJobTokenExchangePath:
			_, _ = io.WriteString(w, `{"token":"jt-issued","expires_at":"2099-01-01T00:00:00Z"}`)
		case "/algo" + qoderModelListPath:
			header := r.Header.Clone()
			seen = header
			_, _ = io.WriteString(w, qoderCatalogFixture)
		}
	}))
	t.Cleanup(server.Close)

	connector, err := NewQoder(qoderEntry("qoder", server.URL, server.URL+"/algo/x"), server.Client())
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}
	if _, err := connector.modelConfig(qoderTestCredential(), "auto"); err != nil {
		t.Fatalf("modelConfig() error = %v", err)
	}
	if seen == nil {
		t.Fatal("the catalogue was read without a request")
	}
	header := seen.Clone()
	if !strings.HasPrefix(header.Get("Authorization"), "Bearer COSY.") {
		t.Fatalf("Authorization = %q, want a signed read", header.Get("Authorization"))
	}
	if seen.Get("Cosy-User") != "user-7" || seen.Get("Cosy-Machineid") != "machine-fixed" {
		t.Fatalf("signature identity = %q / %q", seen.Get("Cosy-User"), seen.Get("Cosy-Machineid"))
	}
	if seen.Get("Accept-Encoding") != "identity" {
		t.Fatal("the catalogue read did not ask for identity encoding")
	}
}

// TestFindQoderModelConfig pins the lookup across the vendor's groups, and the order
// that keeps one answer stable.
func TestFindQoderModelConfig(t *testing.T) {
	raw := json.RawMessage(qoderCatalogFixture)

	config, found := findQoderModelConfig(raw, "ultimate")
	if !found || !strings.Contains(string(config), `"is_vl":true`) {
		t.Fatalf("ultimate = %q, found=%v", config, found)
	}
	if _, found := findQoderModelConfig(raw, "other"); !found {
		t.Fatal("a model in a non-chat group was not found")
	}
	if _, found := findQoderModelConfig(raw, "nope"); found {
		t.Fatal("an unknown key matched")
	}
	if _, found := findQoderModelConfig(json.RawMessage(`"not an object"`), "ultimate"); found {
		t.Fatal("a non-object answer matched")
	}
}
