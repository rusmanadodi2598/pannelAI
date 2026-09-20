// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_session_sweep_test.go
// @for       The whole-table session sweep: every registered pattern is either
//
//	session-gated or listed as an exclusion with a stated reason.
//
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    The per-area 401 tests only cover areas someone already touched, and
//
//	the gate lives on each registration line in router.go, so a new
//	route that forgets gateway() is invisible to them. Walking the
//	recorded table makes the gate a property of the table rather than of
//	review, and forces every public or data-plane exception to state why
//	it is one (draft 004 F4).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-20
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// excludedFromSessionSweep lists the registered patterns that are deliberately
// not session-gated, each with the reason. The sweep fails on a pattern that is
// neither here nor a 401, and it fails on an entry here that names no registered
// pattern, so the table cannot rot: a new public route has to arrive with its
// reason written down rather than silently passing.
//
// These routes are covered by their own area tests (the §7.15 data plane, the
// §7.10 media calls, the §7.1 probes, the §7.2 auth pair, the §7.4 callback);
// this table is the audit of *why* they are absent from the sweep, not a
// replacement for those tests.
var excludedFromSessionSweep = map[string]string{
	// §7.1: read by monitors, which hold no dashboard session.
	"GET /api/v1/health":  "liveness probe; a monitor cannot present a session cookie (§7.1)",
	"GET /api/v1/version": "version probe; public build info, no tenant data (§7.1)",

	// §7.2: the panel asks these before it can hold a cookie.
	"POST /api/v1/auth/login": "login is how a session is obtained, so it cannot require one (§7.2)",
	"GET /api/v1/auth/status": "read before login to decide whether to show the login form (§7.2)",

	// §7.4: the provider redirects the browser here, and a browser cannot
	// present the dashboard cookie on that cross-site redirect.
	"GET /api/v1/providers/{provider_id}/oauth/callback": "provider browser redirect; its replay guard is the single-use state, not a session (§7.4)",

	// §7.15 and §7.10 data plane: a CLI tool presents
	// `Authorization: Bearer <gateway key>`, and a dashboard session cookie is
	// not a credential a CLI tool can hold. The credential here is the gateway
	// key, checked by the handler under the §4 rule.
	"POST /api/v1/chat/completions":      "data plane: gateway key credential, not a dashboard session (§7.15)",
	"POST /api/v1/messages":              "data plane: gateway key credential, not a dashboard session (§7.15)",
	"POST /api/v1/responses":             "data plane: gateway key credential, not a dashboard session (§7.15)",
	"GET /api/v1/models":                 "data plane: the client's own model list, served to a gateway key (§7.15)",
	"POST /api/v1/embeddings":            "data plane: gateway key credential, not a dashboard session (§7.15)",
	"POST /api/v1/messages/count_tokens": "data plane: token estimate under the §4 key rule, no session (§7.15)",
	"POST /api/v1/audio/speech":          "media data plane: gateway key credential, not a dashboard session (§7.10)",
	"POST /api/v1/audio/transcriptions":  "media data plane: gateway key credential, not a dashboard session (§7.10)",
	"GET /api/v1/audio/voices":           "media data plane: gateway key credential, not a dashboard session (§7.10)",
	"POST /api/v1/images/generations":    "media data plane: gateway key credential, not a dashboard session (§7.10)",
	"POST /api/v1/videos/generations":    "media data plane: gateway key credential, not a dashboard session (§7.10)",
	"POST /api/v1/search":                "media data plane: gateway key credential, not a dashboard session (§7.10)",
}

// TestEveryManagementRouteRejectsAnonymousCallers walks the recorded route table
// and asserts that every pattern not listed as an exclusion answers 401 to an
// anonymous caller. A management route registered without gateway() reaches its
// handler anonymously and fails here, which is the one place in the suite that
// notices such a route no matter which area added it.
//
// The fixture registers the nil-guarded groups as zero-value handlers, which is
// what this test needs: the session gate answers before the handler is called,
// so a 401 is the production guard's own answer. Excluded patterns are not
// called at all — their handlers are stubs here, and a data-plane route may
// legitimately answer 401 for a missing gateway key, which would make "not 401"
// a false invariant.
func TestEveryManagementRouteRejectsAnonymousCallers(t *testing.T) {
	mux := newRouteTableRouter(t)
	routes := mux.Routes()
	if len(routes) == 0 {
		t.Fatal("the mux recorded no patterns")
	}

	matched := make(map[string]bool, len(excludedFromSessionSweep))
	gated := 0
	for _, pattern := range routes {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("pattern %q carries no method", pattern)
		}
		if _, excluded := excludedFromSessionSweep[pattern]; excluded {
			matched[pattern] = true
			continue
		}

		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(method, concretePath(path), nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s without a session = %d, want 401: either the route is ungated or it needs an exclusion with a stated reason (body: %s)",
				pattern, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"UNAUTHORIZED"`) {
			t.Fatalf("%s body = %s, want the UNAUTHORIZED code", pattern, recorder.Body.String())
		}
		gated++
	}

	for pattern, reason := range excludedFromSessionSweep {
		if !matched[pattern] {
			t.Fatalf("exclusion %q (%s) matches no registered pattern; drop it so the table stays an audit", pattern, reason)
		}
	}
	if want := len(routes) - len(excludedFromSessionSweep); gated != want {
		t.Fatalf("swept %d routes, want %d of %d (exclusions: %d)", gated, want, len(routes), len(excludedFromSessionSweep))
	}
	t.Logf("%d of %d registered patterns answer 401 without a session; %d exclusions carry a stated reason",
		gated, len(routes), len(excludedFromSessionSweep))
}

// concretePath replaces each {wildcard} segment with a literal, so a registered
// pattern becomes a path the mux can match. The value never reaches a handler:
// the session gate answers before the route's handler runs.
func concretePath(pattern string) string {
	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			segments[i] = "sweep"
		}
	}
	return strings.Join(segments, "/")
}
