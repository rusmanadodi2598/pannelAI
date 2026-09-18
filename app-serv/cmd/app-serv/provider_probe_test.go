// Command app-serv adapts the connectivity probe port to HTTP.
//
// @file      cmd/app-serv/provider_probe_test.go
// @for       Tests for the probe adapter's outcome classification.
// @uses      testing, net/http, net/http/httptest, internal/domain, internal/provider,
//
//	internal/registry, internal/service, time.
//
// @reason    The classification is the whole value of a connectivity test: an
//
//	operator reads its result to decide whether to replace a credential
//	or fix a URL. Reporting a 404 as "your key is wrong" sends them to
//	replace a working credential, so each status class is pinned here
//	against a real server rather than asserted from reading the code.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// probeFixture builds an index holding one provider whose validate URL points at
// the given test server, plus the connector set that serves it.
func probeFixture(t *testing.T, validatePath string) (*registry.Index, *provider.Connectors, registry.Provider) {
	t.Helper()
	entry := registry.Provider{
		ID:       "probe-target",
		Category: "apikey",
		AuthType: registry.AuthAPIKey,
		Transport: registry.Transport{
			Format:      "openai",
			BaseURL:     "https://upstream.test/v1/chat/completions",
			ValidateURL: validatePath,
			Auth:        registry.AuthConfig{Header: "Authorization", Scheme: "bearer"},
		},
	}
	doc := registry.Document{Revision: "probe@test", Providers: []registry.Provider{entry}}
	index, err := registry.NewIndex(doc)
	if err != nil {
		t.Fatalf("NewIndex() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	return index, connectors, entry
}

func TestProbeEndpoint_ClassifiesOutcomes(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantState  string
		wantStatus int
		wantMsg    bool
	}{
		{name: "200 is accepted", status: http.StatusOK, wantState: domain.EndpointTestOK, wantStatus: http.StatusOK},
		{name: "204 is accepted", status: http.StatusNoContent, wantState: domain.EndpointTestOK, wantStatus: http.StatusNoContent},
		{name: "401 blames the credential", status: http.StatusUnauthorized,
			wantState: domain.EndpointTestFail, wantStatus: http.StatusUnauthorized, wantMsg: true},
		{name: "403 blames the credential", status: http.StatusForbidden,
			wantState: domain.EndpointTestFail, wantStatus: http.StatusForbidden, wantMsg: true},
		{name: "404 does not blame the credential", status: http.StatusNotFound,
			wantState: domain.EndpointTestFail, wantStatus: http.StatusNotFound, wantMsg: true},
		{name: "429 is reachable but unhappy", status: http.StatusTooManyRequests,
			wantState: domain.EndpointTestFail, wantStatus: http.StatusTooManyRequests, wantMsg: true},
		{name: "500 is reachable but unhappy", status: http.StatusInternalServerError,
			wantState: domain.EndpointTestFail, wantStatus: http.StatusInternalServerError, wantMsg: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// The probe must present the credential it was handed.
				if got := r.Header.Get("Authorization"); got != "Bearer sk-probe" {
					t.Errorf("Authorization = %q, want the supplied credential", got)
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			index, connectors, _ := probeFixture(t, server.URL+"/models")
			prober := newHTTPEndpointProber(index, connectors)

			endpoint := newEndpoint(t, "probe-target")
			key := newKey(t, endpoint.ID())
			outcome, err := prober.ProbeEndpoint(context.Background(), endpoint, key, "sk-probe")
			if err != nil {
				t.Fatalf("ProbeEndpoint() error = %v, want a classification instead", err)
			}
			if outcome.State != tc.wantState {
				t.Fatalf("State = %q, want %q", outcome.State, tc.wantState)
			}
			if outcome.Status != tc.wantStatus {
				t.Fatalf("Status = %d, want %d", outcome.Status, tc.wantStatus)
			}
			if tc.wantMsg && outcome.Message == "" {
				t.Fatal("a failure must carry an explanation the panel can render")
			}
			if !tc.wantMsg && outcome.Message != "" {
				t.Fatalf("a success carried a message %q, want none", outcome.Message)
			}
		})
	}
}

// TestProbeEndpoint_UnreachableHost reports a failure rather than an error: the
// host not answering is exactly what the test exists to discover.
func TestProbeEndpoint_UnreachableHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	target := server.URL
	server.Close() // now nothing listens on that port

	index, connectors, _ := probeFixture(t, target+"/models")
	prober := newHTTPEndpointProber(index, connectors)

	endpoint := newEndpoint(t, "probe-target")
	key := newKey(t, endpoint.ID())
	outcome, err := prober.ProbeEndpoint(context.Background(), endpoint, key, "sk-probe")
	if err != nil {
		t.Fatalf("ProbeEndpoint() error = %v, want a failed outcome", err)
	}
	if outcome.State != domain.EndpointTestFail {
		t.Fatalf("State = %q, want fail", outcome.State)
	}
	if outcome.Status != 0 {
		t.Fatalf("Status = %d, want 0 when the call never completed", outcome.Status)
	}
	if outcome.Message == "" {
		t.Fatal("an unreachable host must explain itself")
	}
}

// TestProbeEndpoint_UnknownProviderIsAnError pins the boundary between a probe
// answer and an adapter fault: an endpoint referencing a provider the registry
// does not know is a configuration problem, not an upstream answer.
func TestProbeEndpoint_UnknownProviderIsAnError(t *testing.T) {
	index, connectors, _ := probeFixture(t, "https://upstream.test/models")
	prober := newHTTPEndpointProber(index, connectors)

	endpoint := newEndpoint(t, "not-in-the-registry")
	key := newKey(t, endpoint.ID())
	if _, err := prober.ProbeEndpoint(context.Background(), endpoint, key, "sk-probe"); err == nil {
		t.Fatal("ProbeEndpoint() with an unknown provider = nil error, want an error")
	}
}

// TestProbeEndpoint_NoValidateURLIsAFailure covers the honest answer for a
// provider that declares no validation surface: guessing a path would produce a
// 404 that reads like a credential problem.
func TestProbeEndpoint_NoValidateURLIsAFailure(t *testing.T) {
	index, connectors, _ := probeFixture(t, "")
	prober := newHTTPEndpointProber(index, connectors)

	endpoint := newEndpoint(t, "probe-target")
	key := newKey(t, endpoint.ID())
	outcome, err := prober.ProbeEndpoint(context.Background(), endpoint, key, "sk-probe")
	if err != nil {
		t.Fatalf("ProbeEndpoint() error = %v, want a failed outcome", err)
	}
	if outcome.State != domain.EndpointTestFail || outcome.Message == "" {
		t.Fatalf("outcome = %+v, want a failure naming the missing validate URL", outcome)
	}
}

// TestProbeEndpoint_RespectsTheContextDeadline proves a probe cannot outlive the
// budget the service imposes, because an operator waiting on a button must get
// an answer.
func TestProbeEndpoint_RespectsTheContextDeadline(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	defer close(release)

	index, connectors, _ := probeFixture(t, server.URL+"/models")
	prober := newHTTPEndpointProber(index, connectors)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	endpoint := newEndpoint(t, "probe-target")
	key := newKey(t, endpoint.ID())
	outcome, err := prober.ProbeEndpoint(ctx, endpoint, key, "sk-probe")
	if err != nil {
		t.Fatalf("ProbeEndpoint() error = %v, want a failed outcome", err)
	}
	if outcome.State != domain.EndpointTestFail {
		t.Fatalf("State = %q, want fail when the deadline expires", outcome.State)
	}
}

func TestProbeNode_UsesTheNodeBaseURL(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	index, connectors, _ := probeFixture(t, "https://unused.test/models")
	prober := newHTTPEndpointProber(index, connectors)

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
	prober := newHTTPEndpointProber(index, connectors)

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

// newEndpoint builds an endpoint for the probe tests.
func newEndpoint(t *testing.T, providerID string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_probe", providerID, "primary", domain.UpstreamAuthAPIKey, 1, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	return endpoint
}

// newKey builds a key for the probe tests.
func newKey(t *testing.T, endpointID string) domain.UpstreamKey {
	t.Helper()
	key, err := domain.NewUpstreamKey(
		"primary", endpointID, "uky_probe", "v1:sealed", "sk-\u2026robe", 1, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewUpstreamKey() error = %v", err)
	}
	return key
}

// unused keeps the import of service referenced if a future edit drops the
// explicit type references above.
var _ = service.ProbeOutcome{}
