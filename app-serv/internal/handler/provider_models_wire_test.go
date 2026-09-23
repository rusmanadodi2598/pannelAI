// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider_models_wire_test.go
// @for       The §7.4 model-list body: the origin it names, and the constant
//
//	field it no longer carries.
//
// @uses      internal/registry, internal/service, net/http, net/http/httptest,
//
//	strings, testing.
//
// @reason    Draft 017 §4.5 measured `suggested: true` on every row of a route
//
//	whose `?suggested` parameter filtered nothing, and §4.9 recorded that
//	the reference's envelope names the list's origin instead. Both facts
//	are about the wire, so they are asserted on the wire: a field that is
//	constant is a field a client cannot act on, and its removal is only
//	real if the response no longer contains it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-23
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// modelsWireHandler wires one provider handler over a registry provider and a
// custom node, so the two origins are both reachable.
func modelsWireHandler(t *testing.T) *ProviderHandler {
	t.Helper()
	index, err := registry.NewIndex(registry.Document{Revision: "test", Providers: []registry.Provider{
		{ID: "openai", Category: "apikey", Priority: 1, Transport: registry.Transport{Format: "openai"},
			Models: []registry.Model{{ID: "gpt-4o", Name: "GPT-4o", Kind: "llm"}}},
		{ID: "openai-compatible-01TEST", Category: "apikey", Priority: 100, Custom: true,
			Transport: registry.Transport{Format: "openai"},
			Models:    []registry.Model{{ID: "declared-1", Name: "Declared One"}}},
	}})
	if err != nil {
		t.Fatalf("registry.NewIndex() error = %v", err)
	}
	svc, err := service.NewProviderService(service.ProviderServiceDeps{Index: index})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	return NewProviderHandler(svc)
}

// serveModels calls the handler with the path value a mux would install.
func serveModels(t *testing.T, h *ProviderHandler, providerID, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+providerID+"/models"+query, nil)
	req.SetPathValue("provider_id", providerID)
	rr := httptest.NewRecorder()
	h.Models(rr, req)
	return rr
}

// TestProviderModelsWire_NamesTheOriginAndDropsSuggested pins the two wire facts
// together, because they are one change: the envelope gained the field that
// carries information and lost the one that did not.
func TestProviderModelsWire_NamesTheOriginAndDropsSuggested(t *testing.T) {
	h := modelsWireHandler(t)
	rr := serveModels(t, h, "openai", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()

	if !strings.Contains(body, `"source":"registry"`) {
		t.Fatalf("the body does not name the origin of a registry provider's list: %s", body)
	}
	if strings.Contains(body, `"suggested"`) {
		t.Fatalf("the body still carries the constant `suggested` field: %s", body)
	}
	if strings.Contains(body, `"warning"`) {
		t.Fatalf("a registry provider's list carries a warning it has no reason for: %s", body)
	}
}

// TestProviderModelsWire_NodeWithNoSourceFallsBack pins that a node whose list
// could not be read answers its declared models with the origin stated, rather
// than an empty list or a 5xx.
func TestProviderModelsWire_NodeWithNoSourceFallsBack(t *testing.T) {
	h := modelsWireHandler(t)
	rr := serveModels(t, h, "openai-compatible-01TEST", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"declared-1"`) {
		t.Fatalf("a node with no wired source did not answer its declared model: %s", body)
	}
	if !strings.Contains(body, `"source":"registry"`) {
		t.Fatalf("the node's fallback list did not name its origin: %s", body)
	}
}

// TestProviderModelsWire_RemovedParameterChangesNothing is the second half of
// §4.5: the parameter §7.4 used to document is gone from the contract, so the
// route answers the same list whether or not a client still sends it.
func TestProviderModelsWire_RemovedParameterChangesNothing(t *testing.T) {
	h := modelsWireHandler(t)
	withParam := serveModels(t, h, "openai", "?suggested=true")
	without := serveModels(t, h, "openai", "")
	if withParam.Code != http.StatusOK {
		t.Fatalf("status with the removed parameter = %d, want 200 (body: %s)", withParam.Code, withParam.Body.String())
	}
	if withParam.Body.String() != without.Body.String() {
		t.Fatalf("the removed parameter changed the answer:\n with: %s\n without: %s",
			withParam.Body.String(), without.Body.String())
	}
}
