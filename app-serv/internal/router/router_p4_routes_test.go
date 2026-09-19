// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_p4_routes_test.go
// @for       Route-table tests for the §7.16 to §7.18 static routes, and the
//
//	§7.17 contract-coverage pin.
//
// @uses      internal/handler, internal/registry, internal/schema,
//
//	internal/service, encoding/json, net/http, net/http/httptest, strings, testing.
//
// @reason    The routes' session gate and verb table are the mux's job, so they
//
//	are pinned through the real mux with the production auth fixture.
//	The coverage pin walks Mux.Routes against the served document,
//	which is what turns a route missing from openapi.json into a
//	build failure rather than a doc lag.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-20
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// p4Paths are the three §7.16 to §7.18 paths, all read-only GETs with no
// payload, so their validation slot is the verb the mux refuses.
var p4Paths = []string{"/api/v1/skills", "/api/v1/openapi.json", "/api/v1/changelog"}

// TestP4Routes_SessionGated pins that the static routes are management routes:
// an unauthenticated request is a 401 in the §8 envelope, never the data.
func TestP4Routes_SessionGated(t *testing.T) {
	mux, _ := newAuthenticatedRouter(t)
	for _, path := range p4Paths {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s without a session = %d, want 401 (body: %s)", path, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"UNAUTHORIZED"`) {
			t.Fatalf("body = %s, want the UNAUTHORIZED code", recorder.Body.String())
		}
	}
}

// TestP4Routes_ThroughMux drives the three reads through the real mux with a
// session, and the verb none of them registers.
func TestP4Routes_ThroughMux(t *testing.T) {
	mux := newP4Router(t)
	cookie := loginCookie(t, mux)

	get := func(path string) string {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(cookie)
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s = %d (body: %s)", path, recorder.Code, recorder.Body.String())
		}
		return recorder.Body.String()
	}

	skills := get("/api/v1/skills")
	if !strings.Contains(skills, `"entry":true`) || !strings.Contains(skills, `"raw_url"`) {
		t.Fatalf("skills body = %s, want the catalog with its entry skill and derived URLs", skills)
	}
	var contract struct {
		OpenAPI string `json:"openapi"`
	}
	if err := json.Unmarshal([]byte(get("/api/v1/openapi.json")), &contract); err != nil {
		t.Fatalf("the served contract is not valid JSON: %v", err)
	}
	if contract.OpenAPI == "" {
		t.Fatal("the served contract declares no OpenAPI version")
	}
	notes := get("/api/v1/changelog")
	if !strings.Contains(notes, `"version":"v0.4.0"`) {
		t.Fatalf("changelog body = %s, want the newest release first", notes)
	}

	verb := httptest.NewRecorder()
	verbRequest := httptest.NewRequest(http.MethodPost, "/api/v1/skills", strings.NewReader(`{}`))
	verbRequest.AddCookie(cookie)
	mux.ServeHTTP(verb, verbRequest)
	if verb.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post skills = %d, want 405", verb.Code)
	}
	if !strings.Contains(verb.Body.String(), `"METHOD_NOT_ALLOWED"`) {
		t.Fatalf("body = %s, want the METHOD_NOT_ALLOWED code", verb.Body.String())
	}
}

// TestOpenAPICoversEveryRegisteredRoute pins §7.17's rule in both directions:
// every pattern the mux registers is named in the served document, and every
// route the document advertises is one the mux actually registers. A route
// added without updating openapi.json fails here, and so does a stale entry.
//
// The mux carries zero-value handlers for the nil-guarded groups (media, chat,
// embeddings, token counting, OAuth) so their patterns register too; the test
// never invokes them, it reads the recorded table.
func TestOpenAPICoversEveryRegisteredRoute(t *testing.T) {
	mux := newRouteTableRouter(t)

	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(handler.OpenAPIDocument(), &doc); err != nil {
		t.Fatalf("the served contract is not valid JSON: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("the served contract declares no paths")
	}

	registered := make(map[string]bool, len(mux.Routes()))
	for _, pattern := range mux.Routes() {
		method, path, found := strings.Cut(pattern, " ")
		if !found {
			t.Fatalf("pattern %q carries no method", pattern)
		}
		registered[strings.ToLower(method)+" "+path] = true
		operations, ok := doc.Paths[path]
		if !ok {
			t.Fatalf("openapi.json has no path entry for %s", path)
		}
		if _, ok := operations[strings.ToLower(method)]; !ok {
			t.Fatalf("openapi.json does not name %s on %s", method, path)
		}
	}
	for path, operations := range doc.Paths {
		for method := range operations {
			if !registered[method+" "+path] {
				t.Fatalf("openapi.json advertises %s %s, which the router does not register", strings.ToUpper(method), path)
			}
		}
	}
}

// newP4Router builds the real mux with the three static handlers wired
// alongside the auth fixture.
func newP4Router(t *testing.T) *Mux {
	t.Helper()
	_, authHandler := newAuthenticatedRouter(t)
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Auth:      authHandler,
		Skills:    handler.NewSkillsHandler(),
		OpenAPI:   handler.NewOpenAPIHandler(),
		Changelog: handler.NewChangelogHandler(),
	})
}

// newRouteTableRouter registers the complete table: the nil-guarded handler
// groups are zero values, present only so their patterns reach the recorder.
func newRouteTableRouter(t *testing.T) *Mux {
	t.Helper()
	_, authHandler := newAuthenticatedRouter(t)
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Auth:       authHandler,
		OAuth:      &handler.OAuthHandler{},
		Media:      &handler.MediaHandler{},
		Skills:     handler.NewSkillsHandler(),
		OpenAPI:    handler.NewOpenAPIHandler(),
		Changelog:  handler.NewChangelogHandler(),
		Chat:       &handler.ChatHandler{},
		Embeddings: &handler.EmbeddingsHandler{},
		TokenCount: &handler.TokenCountHandler{},
	})
}
