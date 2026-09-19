// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/skills_test.go
// @for       The §7.16 catalog's invariants and the URL derivation.
// @uses      net/http/httptest, encoding/json, strings, testing.
// @reason    The catalog is static data, so the tests pin what a consumer
//
//	relies on: the entry skill first, ids that map one-to-one onto
//	repository paths, endpoints that are §7.15 paths the gateway really
//	serves, and the two URL forms built from the declared repository.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// dataPlaneEndpoints are the §7.15 paths a catalog entry may teach. An entry
// pointing anywhere else advertises a capability the gateway does not serve.
var dataPlaneEndpoints = map[string]bool{
	"/chat/completions":      true,
	"/messages":              true,
	"/responses":             true,
	"/models":                true,
	"/embeddings":            true,
	"/messages/count_tokens": true,
	"/audio/speech":          true,
	"/audio/transcriptions":  true,
	"/audio/voices":          true,
	"/images/generations":    true,
	"/videos/generations":    true,
	"/search":                true,
}

func TestSkillCatalog_Invariants(t *testing.T) {
	if !skillCatalog[0].Entry {
		t.Fatalf("first entry %q is not the index skill", skillCatalog[0].ID)
	}
	seen := make(map[string]bool, len(skillCatalog))
	for _, entry := range skillCatalog {
		if seen[entry.ID] {
			t.Fatalf("skill id %q is declared twice", entry.ID)
		}
		seen[entry.ID] = true
		if entry.RawURL != skillsRawBase+"/"+entry.ID+"/SKILL.md" {
			t.Fatalf("raw_url %q does not follow the declared repository layout", entry.RawURL)
		}
		if !strings.HasPrefix(entry.BlobURL, "https://github.com/"+skillsRepo+"/blob/") {
			t.Fatalf("blob_url %q does not point at the declared repository", entry.BlobURL)
		}
		if entry.Endpoint != nil && !dataPlaneEndpoints[*entry.Endpoint] {
			t.Fatalf("skill %q teaches %q, which the §7.15 data plane does not serve", entry.ID, *entry.Endpoint)
		}
		if entry.Endpoint == nil && !entry.Entry {
			t.Fatalf("skill %q names no endpoint but is not the entry skill", entry.ID)
		}
	}
}

func TestSkillsList_Shape(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/skills", nil)
	NewSkillsHandler().List(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body struct {
		Data []skillEntry `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	if len(body.Data) != len(skillCatalog) {
		t.Fatalf("served %d entries, catalog carries %d", len(body.Data), len(skillCatalog))
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("content type = %q, want JSON", contentType)
	}
}
