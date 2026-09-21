//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/playground_live_doubles_test.go
// @for       The live evidence's narrow doubles: the local upstream, the
//
//	provider lookup, the model lookup, and the request helpers.
//
// @uses      internal/dataplane, internal/domain, internal/registry,
// internal/repository, internal/repository/postgres, encoding/json,
// net/http, net/http/httptest, strings, testing.
// @reason    The upstream and the lookups are narrow stand-ins for a provider the
//
//	evidence must not need, and the request helpers are how every case
//	reaches the mux. They are separate from the stack builder so both
//	files stay inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
)

// liveUpstream is the stand-in provider. It answers a non-streamed completion
// and an SSE stream, so both client shapes are exercised against a real socket
// rather than a function double.
type liveUpstream struct {
	server *httptest.Server
}

func newLiveUpstream(t *testing.T) *liveUpstream {
	t.Helper()
	upstream := &liveUpstream{}
	upstream.server = httptest.NewServer(http.HandlerFunc(upstream.serve))
	t.Cleanup(upstream.server.Close)
	return upstream
}

func (u *liveUpstream) serve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Stream bool `json:"stream"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-live\",\"object\":\"chat.completion.chunk\",\"model\":\"live-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"pong\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"chatcmpl-live","object":"chat.completion","created":1,"model":"live-model","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`))
}

// liveRedisOptions parses the test address. An address without credentials is
// the common case; `user:password@host:port` is accepted so a password-protected
// server needs no extra variable, which is the same shape the repository tests
// take.
func liveRedisOptions(raw string) *redis.Options {
	options := &redis.Options{Addr: raw}
	at := strings.LastIndex(raw, "@")
	if at < 0 {
		return options
	}
	options.Addr = raw[at+1:]
	user, password, hasUser := strings.Cut(raw[:at], ":")
	if hasUser {
		options.Username = user
		options.Password = password
		return options
	}
	options.Password = raw[:at]
	return options
}

// post sends one request through the real mux and returns the status, the body,
// and the request id the gateway echoed.
func (s liveStack) post(t *testing.T, body, key, requestID string) (int, string, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	if key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
	}
	if requestID != "" {
		request.Header.Set(router.RequestIDHeader, requestID)
	}
	recorder := httptest.NewRecorder()
	s.mux.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String(), recorder.Header().Get(router.RequestIDHeader)
}

// liveCodeOf reads the machine code from a data-plane error envelope, and ""
// when the body is a success or a stream.
func liveCodeOf(t *testing.T, body string) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		return ""
	}
	return envelope.Error.Code
}

// liveRegistry answers the provider lookup with the one live provider.
type liveRegistry struct{ provider registry.Provider }

func (r liveRegistry) Provider(name string) (registry.Provider, bool) {
	if name == r.provider.ID {
		return r.provider, true
	}
	return registry.Provider{}, false
}
func (liveRegistry) Model(string, string) (registry.Model, bool) { return registry.Model{}, false }
func (r liveRegistry) All() []registry.Provider                  { return []registry.Provider{r.provider} }

// liveModelLookup answers "no combo, no alias" so the model resolves as
// provider/model, which is what the evidence sends.
type liveModelLookup struct{}

func (liveModelLookup) Combo(context.Context, string) (domain.Combo, bool, error) {
	return domain.Combo{}, false, nil
}
func (liveModelLookup) Alias(context.Context, string) (string, bool, error) { return "", false, nil }
func (liveModelLookup) Disabled(context.Context, string, string) (bool, error) {
	return false, nil
}
func (liveModelLookup) DisabledPairs(context.Context) ([]domain.ModelRef, error) { return nil, nil }
func (liveModelLookup) ComboNames(context.Context) ([]string, error)             { return nil, nil }

var _ repository.EndpointRepository = (*postgres.EndpointRepository)(nil)
