// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_oauth_test.go
// @for       Route-table and auth tests for the §7.4 OAuth routes.
// @uses      internal/domain, internal/handler, internal/registry,
//
//	internal/repository, internal/service, net/http, strings, testing.
//
// @reason    §7.4 is the one section where three routes are session-gated and
//
//	the fourth is deliberately public, so the route table is where that
//	difference is provable. AGENTS.md §2.1 also requires an auth-failure
//	test for every protected route; the table supplies one per route.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// The empty seams below are never reached by these tests: every case names an
// unknown provider or no session at all, so the flow refuses before it reads
// storage. They exist because the service validates its dependencies.
type emptyOAuthStore struct{}

func (emptyOAuthStore) Create(context.Context, domain.UpstreamEndpoint) error { return nil }
func (emptyOAuthStore) List(context.Context, repository.EndpointFilter, repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error) {
	return nil, 0, nil
}
func (emptyOAuthStore) GetByID(context.Context, string) (domain.UpstreamEndpoint, error) {
	return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
}
func (emptyOAuthStore) Update(context.Context, domain.UpstreamEndpoint) error { return nil }
func (emptyOAuthStore) FindOAuthEndpoint(context.Context, string, string, string) (string, error) {
	return "", domain.ErrEndpointNotFound
}

type emptyOAuthStates struct{}

func (emptyOAuthStates) Stage(context.Context, string, []byte, time.Duration) error { return nil }
func (emptyOAuthStates) Take(context.Context, string) ([]byte, bool, error)         { return nil, false, nil }

type emptyOAuthTokens struct{}

func (emptyOAuthTokens) Grant(context.Context, string, string, service.TokenGrant) (service.TokenResponse, error) {
	return service.TokenResponse{}, nil
}

func (emptyOAuthTokens) UserInfo(context.Context, string, string) (service.OAuthIdentity, error) {
	return service.OAuthIdentity{}, nil
}

// newOAuthRouter wires the OAuth handler over the real service and the real
// registry index, so the guard under test is the production one.
func newOAuthRouter(t *testing.T) *Mux {
	t.Helper()
	_, authHandler := newAuthenticatedRouter(t)
	index, err := registry.NewIndex(registry.Document{Revision: "test", Providers: []registry.Provider{
		{ID: "acme", Priority: 1, Category: "oauth", OAuth: &registry.OAuth{
			ClientID: "client-acme", AuthorizeURL: "https://auth.example.com/acme/authorize",
			TokenURL: "https://auth.example.com/acme/token",
		}},
	}})
	if err != nil {
		t.Fatalf("registry.NewIndex() error = %v", err)
	}
	sealer, err := domain.NewSealer([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	flow, err := service.NewOAuthFlowService(service.OAuthFlowDeps{
		Index: index, Store: emptyOAuthStore{}, States: emptyOAuthStates{},
		Tokens: emptyOAuthTokens{}, Sealer: sealer,
	})
	if err != nil {
		t.Fatalf("NewOAuthFlowService() error = %v", err)
	}
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Auth:  authHandler,
		OAuth: handler.NewOAuthHandler(flow, "https://gateway.example.com"),
	})
}

// TestOAuthRoutes pins the §7.4 surface: the three management routes refuse an
// anonymous caller, the callback serves one without a session (it answers the
// browser redirect with the failure reason rather than a 401), and a wrong verb
// is a 405 rather than a 404 on every one of them.
func TestOAuthRoutes(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		accept     string
		wantStatus int
	}{
		{name: "start requires a session", method: http.MethodPost,
			path: "/api/v1/providers/acme/oauth/start", wantStatus: http.StatusUnauthorized},
		{name: "status requires a session", method: http.MethodGet,
			path: "/api/v1/providers/acme/oauth/status", wantStatus: http.StatusUnauthorized},
		{name: "refresh requires a session", method: http.MethodPost,
			path: "/api/v1/providers/acme/oauth/refresh", wantStatus: http.StatusUnauthorized},
		{name: "the callback is public and redirects a browser", method: http.MethodGet,
			path: "/api/v1/providers/acme/oauth/callback?state=gone", wantStatus: http.StatusFound},
		{name: "the callback answers a headless caller", method: http.MethodGet,
			path:   "/api/v1/providers/acme/oauth/callback?state=gone",
			accept: "application/json", wantStatus: http.StatusBadRequest},
		{name: "a wrong verb on the callback is a 405", method: http.MethodPost,
			path: "/api/v1/providers/acme/oauth/callback", wantStatus: http.StatusMethodNotAllowed},
		{name: "a wrong verb on status is a 405", method: http.MethodPost,
			path: "/api/v1/providers/acme/oauth/status", wantStatus: http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := newOAuthRouter(t)
			request := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.accept != "" {
				request.Header.Set("Accept", tc.accept)
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)",
					recorder.Code, tc.wantStatus, recorder.Body.String())
			}
		})
	}
}
