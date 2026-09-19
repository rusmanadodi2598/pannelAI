// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi.go
// @for       The served machine-readable contract (SPEC-API-001 §7.17).
// @uses      embed, net/http.
// @reason    A self-hosted gateway's panel cannot assume internet access, so
//
//	§7.17 serves the contract from the binary instead of pointing at
//	an external docs site. The document is static; the router test that
//	pins every registered pattern onto it is what turns a forgotten
//	route into a build failure rather than a doc lag.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"embed"
	"net/http"
)

// embeddedDocs carries the OpenAPI document inside the binary.
//
//go:embed openapi.json
var embeddedDocs embed.FS

// openAPIDocumentFile is the embedded document's name.
const openAPIDocumentFile = "openapi.json"

// OpenAPIHandler serves GET /api/v1/openapi.json.
type OpenAPIHandler struct{}

// NewOpenAPIHandler returns the contract handler. It has no dependencies: the
// document is embedded at build.
func NewOpenAPIHandler() *OpenAPIHandler { return &OpenAPIHandler{} }

// OpenAPIDocument returns the embedded contract bytes. It is exported for the
// router test that walks the mux's registered patterns and pins each one onto
// the document, which is §7.17's "a missing route is a build failure" rule.
func OpenAPIDocument() []byte {
	raw, err := embeddedDocs.ReadFile(openAPIDocumentFile)
	if err != nil {
		// The file is embedded at compile time, so a read failure here is a
		// build misconfiguration, not a runtime condition a caller could handle.
		panic("openapi: reading embedded " + openAPIDocumentFile + ": " + err.Error())
	}
	return raw
}

// Document serves GET /api/v1/openapi.json. The response is the document
// itself, so a reader can diff the served contract against the spec that
// produced it; the §8 error envelope would defeat that purpose.
func (h *OpenAPIHandler) Document(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(OpenAPIDocument())
}
