// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider_detail_thinking_test.go
// @for       The §7.4 detail route's reasoning level union on a custom node.
// @uses      internal/domain, internal/registry, internal/service, context, encoding/json, net/http, net/http/httptest, reflect, testing, time.
// @reason    SPEC-API-001 §7.14 makes the detail body the union of the levels a provider's models accept, and a synthesized custom node has no registry models, so the panel's picker on that screen exists only if this route carries the levels its declared rows accept. The owner reported the missing control on 2026-09-27; this file pins the wire answer, and the second case pins that a node with no declared rows answers no field at all rather than an empty one, which is what hides the picker.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-27
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// detailCustomLister is the declared-rows seam double the provider service
// reads for the level union.
type detailCustomLister struct{ rows []domain.CustomModel }

func (l detailCustomLister) Custom(context.Context, string) ([]domain.CustomModel, error) {
	return l.rows, nil
}

// declaredDetailRow builds one row the operator declared.
func declaredDetailRow(t *testing.T, providerID, modelID string) domain.CustomModel {
	t.Helper()
	row, err := domain.NewCustomModel("mdl_"+modelID, providerID, modelID, modelID, nil,
		time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewCustomModel(%q) error = %v", modelID, err)
	}
	return row
}

// getDetail drives the detail route and decodes the level union it answered.
func getDetail(t *testing.T, handler *ProviderHandler, providerID string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+providerID, nil)
	req.SetPathValue("provider_id", providerID)
	rr := httptest.NewRecorder()
	handler.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rr.Code, rr.Body.String())
	}
	var body struct {
		ThinkingLevels []string `json:"thinking_levels"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	return body.ThinkingLevels
}

// TestProviderHandler_Get_ANodeCarriesItsDeclaredLevels pins the wire answer the
// panel's reasoning picker reads on a custom node's screen.
func TestProviderHandler_Get_ANodeCarriesItsDeclaredLevels(t *testing.T) {
	node := registry.Provider{ID: "openai-compatible-01TEST", Category: "apikey", Custom: true,
		Transport: registry.Transport{Format: "openai"}}
	svc, err := service.NewProviderService(service.ProviderServiceDeps{
		Index:  searchIndex{entries: []registry.Provider{node}},
		Custom: detailCustomLister{rows: []domain.CustomModel{declaredDetailRow(t, node.ID, "claude-opus-4-8")}},
	})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}

	got := getDetail(t, NewProviderHandler(svc), node.ID)
	want := []string{"low", "medium", "high", "max"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("thinking_levels = %v, want %v", got, want)
	}
}

// TestProviderHandler_Get_ANodeWithNoDeclaredRowsOmitsTheField pins the other
// half: the picker is hidden by the field's absence, so an empty array would be
// a different answer and the panel would render an "auto"-only control.
func TestProviderHandler_Get_ANodeWithNoDeclaredRowsOmitsTheField(t *testing.T) {
	node := registry.Provider{ID: "openai-compatible-01TEST", Category: "apikey", Custom: true,
		Transport: registry.Transport{Format: "openai"}}
	svc, err := service.NewProviderService(service.ProviderServiceDeps{
		Index:  searchIndex{entries: []registry.Provider{node}},
		Custom: detailCustomLister{},
	})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}

	if got := getDetail(t, NewProviderHandler(svc), node.ID); got != nil {
		t.Fatalf("thinking_levels = %v, want the field absent", got)
	}
}
