// Command app-serv adapts the connectivity probe port to HTTP.
//
// @file      cmd/app-serv/provider_probe_node_test.go
// @for       The node-probe tests: a custom node's base URL is dialed with the
//
//	operator's credential, and the composed path is the one the node's
//	type speaks.
//
// @uses      testing, context, net/http, net/http/httptest, sync/atomic, time,
//
//	internal/domain.
//
// @reason    A node has no registry entry, so the probe composes its target from
//
//	the base URL alone. That composition is the whole contract: a wrong
//	path reports a working node as broken, and a doubled one reports a
//	404 the operator reads as a credential problem. Split from the
//	endpoint suite to keep both files under AGENTS.md §1.1.
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
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestProbeNode_UsesTheNodeBaseURL(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	index, connectors, _ := probeFixture(t, "https://unused.test/models")
	prober := newHTTPEndpointProber(index, connectors, probeGuard(t, "127.0.0.1/32"))

	node, err := domain.NewProviderNode(
		"openai-compatible-chat-1", "My Corp", "mycorp",
		domain.NodeOpenAICompatible, domain.NodeAPIChat, server.URL, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewProviderNode() error = %v", err)
	}

	outcome, err := prober.ProbeNode(context.Background(), node, "sk-node")
	if err != nil {
		t.Fatalf("ProbeNode() error = %v", err)
	}
	if outcome.State != domain.EndpointTestOK {
		t.Fatalf("State = %q, want ok", outcome.State)
	}
	if gotPath != "/models" {
		t.Fatalf("path = %q, want /models", gotPath)
	}
	if gotAuth == "" {
		t.Fatal("the node's credential was not presented")
	}
}

// TestProbeNode_NoCredentialStillProbes covers a no_auth node: a credential-free
// upstream must be testable, so the probe sends no Authorization header and must
// not fail for lacking one.
func TestProbeNode_NoCredentialStillProbes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want it absent for a credential-free node", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	index, connectors, _ := probeFixture(t, "https://unused.test/models")
	prober := newHTTPEndpointProber(index, connectors, probeGuard(t, "127.0.0.1/32"))

	node, err := domain.NewProviderNode(
		"openai-compatible-chat-1", "Open", "open",
		domain.NodeOpenAICompatible, domain.NodeAPIChat, server.URL, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewProviderNode() error = %v", err)
	}
	outcome, err := prober.ProbeNode(context.Background(), node, "")
	if err != nil {
		t.Fatalf("ProbeNode() error = %v", err)
	}
	if outcome.State != domain.EndpointTestOK {
		t.Fatalf("State = %q, want ok", outcome.State)
	}
}

// TestProbeNode_RefusesADeniedAddress pins the egress policy on the probe: a
// node whose base_url is a denied address is refused before anything is sent,
// and the same address is probed once the operator allowlists it.
func TestProbeNode_RefusesADeniedAddress(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	nodeAt := func(t *testing.T, baseURL string) domain.ProviderNode {
		t.Helper()
		node, err := domain.NewProviderNode(
			"openai-compatible-chat-1", "My Corp", "mycorp",
			domain.NodeOpenAICompatible, domain.NodeAPIChat, baseURL, time.Now().UTC(),
		)
		if err != nil {
			t.Fatalf("NewProviderNode(%q) error = %v", baseURL, err)
		}
		return node
	}

	cases := []struct {
		name      string
		baseURL   string
		allowed   []string
		wantState string
		wantMsg   string
		wantHits  int32
	}{
		{
			name: "a loopback node is refused by default", baseURL: server.URL,
			wantState: domain.EndpointTestFail, wantMsg: "loopback",
		},
		{
			name: "the allowlisted loopback is probed", baseURL: server.URL,
			allowed: []string{"127.0.0.1/32"}, wantState: domain.EndpointTestOK, wantHits: 1,
		},
		{
			name: "a private node address is refused", baseURL: "http://10.0.0.5/v1",
			wantState: domain.EndpointTestFail, wantMsg: "private",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits.Store(0)
			index, connectors, _ := probeFixture(t, "https://unused.test/models")
			prober := newHTTPEndpointProber(index, connectors, probeGuard(t, tc.allowed...))

			outcome, err := prober.ProbeNode(context.Background(), nodeAt(t, tc.baseURL), "sk-node")
			if err != nil {
				t.Fatalf("ProbeNode() error = %v, want a probe answer", err)
			}
			if outcome.State != tc.wantState {
				t.Fatalf("State = %q (message %q), want %q", outcome.State, outcome.Message, tc.wantState)
			}
			if tc.wantMsg != "" && !strings.Contains(outcome.Message, tc.wantMsg) {
				t.Fatalf("Message = %q, want it to mention %q", outcome.Message, tc.wantMsg)
			}
			if got := hits.Load(); got != tc.wantHits {
				t.Fatalf("the upstream saw %d requests, want %d", got, tc.wantHits)
			}
		})
	}
}
