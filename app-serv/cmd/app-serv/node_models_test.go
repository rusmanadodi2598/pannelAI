// Command app-serv adapts a provider node's model list to HTTP.
//
// @file      cmd/app-serv/node_models_test.go
// @for       The parsing, the fallback, the credential redaction, and the egress
//
//	refusal of the node model-list adapter.
//
// @uses      internal/domain, internal/netguard, internal/provider,
//
//	internal/registry, internal/service, context, encoding/json,
//	net/http, net/http/httptest, strings, testing, time.
//
// @reason    SPEC-API-001 §7.4 serves a node's models, and the adapter is the one
//
//	place that dials an operator-supplied URL for that read. Two rules are
//	asserted here rather than left to review: the warning never carries the
//	credential (OWASP A09), and a destination the guard refuses produces a
//	fallback rather than a request (OWASP A01).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// nodeModelFixture is one adapter under test, its destination server, and the
// credential it was handed.
type nodeModelFixture struct {
	source     *nodeModelSource
	server     *httptest.Server
	credential string
}

// newNodeModelFixture builds an adapter pointed at a test server, with the guard
// allowlisted for loopback so the server is reachable. The allowlist is what
// makes the guard refusal a separate case below: a default guard refuses
// loopback, which is the correct production behaviour.
func newNodeModelFixture(t *testing.T, handler http.HandlerFunc, credential string) nodeModelFixture {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	guard, err := netguard.NewGuard([]string{"127.0.0.1/32", "::1/128"})
	if err != nil {
		t.Fatalf("netguard.NewGuard() error = %v", err)
	}
	source := newNodeModelSource(
		staticNodeLookup{id: "openai-compatible-01TEST", baseURL: server.URL, format: "openai", apiType: "chat"}.lookup,
		testConnectors(t),
		guard,
		staticCredential(credential),
		time.Minute,
	)
	source.clock = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	return nodeModelFixture{source: source, server: server, credential: credential}
}

// TestNodeModelSource_ParsesTheShapesTheReferenceAccepts covers every response
// shape `parseOpenAIStyleModels` accepts, plus the empty and malformed answers
// that must fall back.
func TestNodeModelSource_ParsesTheShapesTheReferenceAccepts(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		wantSource  string
		wantModels  []string
		wantWarning bool
	}{
		{
			name: "an OpenAI-style list is parsed", status: http.StatusOK,
			body:       `{"data":[{"id":"a","name":"A"},{"id":"b"}]}`,
			wantSource: service.ModelSourceUpstream, wantModels: []string{"a", "b"},
		},
		{
			name: "a bare array is parsed", status: http.StatusOK,
			body:       `[{"id":"a"}]`,
			wantSource: service.ModelSourceUpstream, wantModels: []string{"a"},
		},
		{
			name: "a models key is parsed", status: http.StatusOK,
			body:       `{"models":[{"id":"a"}]}`,
			wantSource: service.ModelSourceUpstream, wantModels: []string{"a"},
		},
		{
			name: "a results key is parsed", status: http.StatusOK,
			body:       `{"results":[{"id":"a"}]}`,
			wantSource: service.ModelSourceUpstream, wantModels: []string{"a"},
		},
		{
			name: "an entry with no id is dropped", status: http.StatusOK,
			body:       `{"data":[{"name":"nameless"},{"id":"a"}]}`,
			wantSource: service.ModelSourceUpstream, wantModels: []string{"a"},
		},
		{
			name: "a name defaults to the id", status: http.StatusOK,
			body:       `{"data":[{"id":"a"}]}`,
			wantSource: service.ModelSourceUpstream, wantModels: []string{"a"},
		},
		{
			name: "a 401 falls back with a warning", status: http.StatusUnauthorized,
			body: ``, wantSource: service.ModelSourceRegistry, wantWarning: true,
		},
		{
			name: "a 500 falls back with a warning", status: http.StatusInternalServerError,
			body: ``, wantSource: service.ModelSourceRegistry, wantWarning: true,
		},
		{
			name: "a non-JSON body falls back with a warning", status: http.StatusOK,
			body: `<html>`, wantSource: service.ModelSourceRegistry, wantWarning: true,
		},
		{
			name: "an empty list falls back with a warning", status: http.StatusOK,
			body: `{"data":[]}`, wantSource: service.ModelSourceRegistry, wantWarning: true,
		},
		{
			name: "an unknown shape falls back with a warning", status: http.StatusOK,
			body: `{"unexpected":true}`, wantSource: service.ModelSourceRegistry, wantWarning: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newNodeModelFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}, "sk-live-abcdef")

			list, err := fixture.source.ListNodeModels(context.Background(), "openai-compatible-01TEST")
			if err != nil {
				t.Fatalf("ListNodeModels() error = %v", err)
			}
			if list.Source != tc.wantSource {
				t.Fatalf("Source = %q, want %q", list.Source, tc.wantSource)
			}
			if tc.wantWarning && list.Warning == "" {
				t.Fatal("a fallback must carry a warning; without one a client cannot tell it from an upstream answer")
			}
			if !tc.wantWarning && list.Warning != "" {
				t.Fatalf("an upstream answer carried a warning: %q", list.Warning)
			}
			if strings.Contains(list.Warning, fixture.credential) {
				t.Fatalf("the warning carries the credential: %q", list.Warning)
			}
			ids := make([]string, 0, len(list.Models))
			for _, model := range list.Models {
				ids = append(ids, model.ID)
			}
			if !equalStrings(ids, tc.wantModels) {
				t.Fatalf("models = %v, want %v", ids, tc.wantModels)
			}
			for _, model := range list.Models {
				if model.Name == "" {
					t.Fatalf("model %q has no display name; the panel would render a blank cell", model.ID)
				}
			}
		})
	}
}

// TestNodeModelSource_NoCredentialIsStillARequest pins that a node with no
// stored key is asked anyway: an upstream that needs no credential is a
// legitimate node, and skipping the request would report a fallback for it.
func TestNodeModelSource_NoCredentialIsStillARequest(t *testing.T) {
	var sentAuth string
	fixture := newNodeModelFixture(t, func(w http.ResponseWriter, r *http.Request) {
		sentAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"a"}]}`))
	}, "")

	list, err := fixture.source.ListNodeModels(context.Background(), "openai-compatible-01TEST")
	if err != nil {
		t.Fatalf("ListNodeModels() error = %v", err)
	}
	if list.Source != service.ModelSourceUpstream {
		t.Fatalf("Source = %q, want %q", list.Source, service.ModelSourceUpstream)
	}
	if sentAuth != "" {
		t.Fatalf("an empty credential sent an Authorization header: %q", sentAuth)
	}
}
