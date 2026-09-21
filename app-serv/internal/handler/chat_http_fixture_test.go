// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_http_fixture_test.go
// @for       The real ChatService fixture used by HTTP readiness tests.
// @uses      internal/dataplane, internal/domain, internal/provider,
// internal/registry, internal/repository, internal/service, internal/schema,
// net/http, net/http/httptest, context, sync, testing.
// @reason    F4 requires handler tests to cross the HTTP boundary into the real
// service and engine. This fixture keeps the upstream and storage doubles
// narrow while preserving production authentication, routing, transport,
// translation, and error mapping.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

type chatHTTPFixture struct {
	chat *service.ChatService
	key  string
}

// fixtureDeps are the optional collaborators a test wants wired. Every field is
// nil by default, so the plain fixture is the production pipeline without
// accounting, and a test that asserts on a write supplies only the seam it reads.
type fixtureDeps struct {
	Usage     service.UsageRecorder
	Logs      service.RequestLogRecorder
	Quotas    *service.QuotaCounter
	KeyUse    service.KeyUseRecorder
	RequestID service.RequestIDReader
	Settings  service.RequireAPIKeyReader
}

func newChatHTTPFixture(t *testing.T) chatHTTPFixture {
	t.Helper()
	return newChatHTTPFixtureWith(t, fixtureDeps{})
}

// newChatHTTPFixtureWith builds the fixture over the real ChatService, engine,
// selector, and transport, adding whichever optional seams the caller supplied.
func newChatHTTPFixtureWith(t *testing.T, deps fixtureDeps) chatHTTPFixture {
	t.Helper()
	upstream := &chatHTTPUpstream{}
	upstream.server = httptest.NewServer(http.HandlerFunc(upstream.serve))
	t.Cleanup(upstream.server.Close)

	engine, err := dataplane.NewEngine(dataplane.EngineDeps{
		Resolver:  newChatResolver(t, upstream.server.URL),
		Selector:  newChatSelector(t),
		Transport: newChatTransport(t),
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	key := domain.NewGatewayKey("playground-test", "sk-playground-test", "sk-...test", testInstant())
	settings := deps.Settings
	if settings == nil {
		settings = chatAPIKeySettings{}
	}
	chat, err := service.NewChatService(service.ChatServiceDeps{
		Engine:    engine,
		Keys:      chatKeyLookup{key: key},
		Settings:  settings,
		Usage:     deps.Usage,
		Logs:      deps.Logs,
		Quotas:    deps.Quotas,
		KeyUse:    deps.KeyUse,
		RequestID: deps.RequestID,
	})
	if err != nil {
		t.Fatalf("NewChatService() error = %v", err)
	}
	return chatHTTPFixture{chat: chat, key: "sk-playground-test"}
}
