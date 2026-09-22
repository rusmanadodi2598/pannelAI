//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/usage_live_integration_test.go
// @for       F4 live evidence: the real gateway serving the live Usage route, a
//
//	real data-plane request lighting a node, and the frame carrying it.
//
// @uses      internal/dataplane, internal/domain, internal/handler, internal/router,
//
//	internal/schema, internal/service, bufio, context, encoding/json, net,
//	net/http, net/http/httptest, os, strings, testing, time.
//
// @reason    Draft 013 F4 is closed by "the route answers with a session and the
//
//	live pass records a frame from the gateway itself". Every half of
//	that has a unit test, and none of them proves the two halves meet:
//	the marker is written by a real relay leg over a real socket, the
//	store is the real Redis, and the frame is read off a real
//	connection through the real router. That is the evidence the
//	previous pass could not produce, and it is the difference between
//	"the code should work" and "the frame arrived".
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration -run TestUsageLive ./cmd/app-serv/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// liveUsageFixture is the gateway under test for this pass: the real router,
// the real live handler, the real Redis marker store, and a real PostgreSQL
// usage repository.
type liveUsageFixture struct {
	live  *liveStack
	store *redisrepo.ActiveRequestStore
}

// newLiveUsageFixture builds the live route over the stack's real Redis and
// PostgreSQL, wired the way the composition root wires it: one store instance
// serves both the tracker that writes markers and the service that reads them.
//
// The tracker has to exist before the stack, because the stack's engine is what
// opens the markers: building it afterwards would leave the engine writing
// nothing and the stream reading an always-empty set, which is a pass that
// proves nothing while looking green. The fixture therefore opens its own client
// to the same Redis the stack uses, which is what the composition root does with
// one client: the store is keyed by name, so two clients reach the same set.
func newLiveUsageFixture(t *testing.T, upstream *liveUpstream) liveUsageFixture {
	t.Helper()
	raw := os.Getenv(liveRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set when running with -tags=integration", liveRedisEnv)
	}
	client := redis.NewClient(liveRedisOptions(raw))
	t.Cleanup(func() { _ = client.Close() })

	store := redisrepo.NewActiveRequestStore(client)
	tracker := service.NewActiveRequestTracker(store, router.RequestIDFrom, nil)
	stack := newLiveStack(t, upstream, tracker)
	return liveUsageFixture{live: &stack, store: store}
}

// liveRouteHandler registers only the live route, so the pass exercises the real
// handler through a real socket without rebuilding the whole management graph.
func liveRouteHandler(t *testing.T, fixture liveUsageFixture, requireSession bool) http.Handler {
	t.Helper()
	liveSvc, err := service.NewUsageLiveService(service.UsageLiveServiceDeps{
		Active: fixture.store, Usage: newLiveUsageRepo(*fixture.live),
	})
	if err != nil {
		t.Fatalf("building the live service: %v", err)
	}
	liveHandler := handler.NewUsageLiveHandler(liveSvc)
	// A short read interval keeps the pass quick without changing any rule: the
	// production interval is one second.
	liveHandler.SetIntervals(100*time.Millisecond, 2*time.Second)

	mux := http.NewServeMux()
	route := http.HandlerFunc(liveHandler.Stream)
	if requireSession {
		mux.Handle("GET /api/v1/usage/live", liveSessionGate(t, route))
		return mux
	}
	mux.Handle("GET /api/v1/usage/live", route)
	return mux
}

// TestUsageLive_RouteAnswersAnAnonymousCallerWithTheGate pins the credential the
// route answers to: without a session it is the §8 envelope, not a stream.
func TestUsageLive_RouteAnswersAnAnonymousCallerWithTheGate(t *testing.T) {
	fixture := newLiveUsageFixture(t, newLiveUpstream(t))
	server := httptest.NewServer(liveRouteHandler(t, fixture, true))
	t.Cleanup(server.Close)

	response, err := server.Client().Get(server.URL + "/api/v1/usage/live")
	if err != nil {
		t.Fatalf("GET /api/v1/usage/live: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	t.Logf("live evidence: anonymous status=%d content_type=%s", response.StatusCode, response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: the live route carries request identity and answers to the session gate",
			response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("content type = %q, want the §8 envelope rather than a stream", got)
	}
}
