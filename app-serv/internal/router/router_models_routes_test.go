// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_models_routes_test.go
// @for       Route-table tests for the same §7.6–§7.8 management routes. (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/domain, internal/handler, internal/registry,
// @reason    The route table and its auth/validation/verb cases are one group and the fixtures another; AGENTS.md §1.1 caps a file at 250 lines, so the fixtures moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-17
package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// routeProber answers the §7.7 combo test route healthily, so the route table
// can drive the route end to end without an engine.
type routeProber struct{}

func (routeProber) Ping(context.Context, string) (dataplane.Outcome, error) {
	return dataplane.Outcome{ProviderID: "openai", EndpointID: "ep-1", Model: "gpt-4o", LatencyMS: 12}, nil
}

// TestManagementRoutes_DuplicateComboNameIsConflict pins the uniqueness mapping
// on the combo route, so the panel gets the same code the gateway keys return.
func TestManagementRoutes_DuplicateComboNameIsConflict(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)
	body := `{"name":"seeded-combo","strategy":"fallback","models":[{"ref":"openai/gpt-4o","priority":1}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/combos", strings.NewReader(body))
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body: %s)", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"CONFLICT"`) {
		t.Fatalf("body = %s, want the CONFLICT code", recorder.Body.String())
	}
}

// TestComboDeleteThroughMux_ConflictWhileReferenced proves the §7.7 delete rule
// reaches the wire as a 409 with the conflict code.
func TestComboDeleteThroughMux_ConflictWhileReferenced(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)

	alias := httptest.NewRequest(http.MethodPut, "/api/v1/models/aliases",
		strings.NewReader(`{"aliases":[{"alias":"daily-alias","target":"seeded-combo"}]}`))
	alias.AddCookie(cookie)
	aliasRecorder := httptest.NewRecorder()
	mux.ServeHTTP(aliasRecorder, alias)
	if aliasRecorder.Code != http.StatusOK {
		t.Fatalf("seeding the alias = %d (body: %s)", aliasRecorder.Code, aliasRecorder.Body.String())
	}

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/combos/cmb_seeded", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body: %s)", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"CONFLICT"`) {
		t.Fatalf("body = %s, want the CONFLICT code", recorder.Body.String())
	}
}

// newManagementRouter builds the real mux with this vertical's handlers wired
// alongside the authenticated ones, over in-memory repositories and a small fixed
// registry. It reuses the auth fixture and the gateway-key stub rather than
// re-declaring them, so the session guard under test is the production one.
func newManagementRouter(t *testing.T) *Mux {
	t.Helper()
	_, authHandler := newAuthenticatedRouter(t)
	keyService, err := service.NewGatewayKeyService(service.GatewayKeyServiceDeps{Repo: newMemKeyRepo(), Prefix: "sk-"})
	if err != nil {
		t.Fatalf("key service: %v", err)
	}
	management := newCatalogFixture(t)
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Auth:          authHandler,
		GatewayKey:    handler.NewGatewayKeyHandler(keyService),
		Model:         management.model,
		Combo:         management.combo,
		ComboTest:     management.comboTest,
		VisionAdapter: management.vision,
		Proxy:         management.proxy,
		MediaProvider: management.media,
	})
}

// loginCookie logs in through the mux and returns the session cookie.
func loginCookie(t *testing.T, mux *Mux) *http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"password":"correct"}`)))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("test auth login: %d (%s)", recorder.Code, recorder.Body.String())
	}
	return recorder.Result().Cookies()[0]
}

// The remaining stubs implement the repository contracts in memory, mirroring
// the uniqueness the real schema enforces so a route test cannot accept a
// payload PostgreSQL would reject.

type memCatalogRepo struct {
	custom   map[string]domain.CustomModel
	aliases  []domain.ModelAlias
	disabled []domain.ModelRef
}

func newMemCatalogRepo() *memCatalogRepo {
	return &memCatalogRepo{custom: map[string]domain.CustomModel{}, disabled: []domain.ModelRef{}}
}

func (r *memCatalogRepo) Custom(context.Context) ([]domain.CustomModel, error) {
	out := make([]domain.CustomModel, 0, len(r.custom))
	for _, model := range r.custom {
		out = append(out, model)
	}
	return out, nil
}
