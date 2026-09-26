// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_forced_scope_test.go
// @for       The scope of the forced-stream fold: which providers it applies to,
//
//	and which it must leave alone.
//
// @uses      testing, context, internal/domain, internal/registry.
// @reason    The fold changes how an answer is read, so applying it to a provider
//
//	that does not need it would turn a working non-streamed call into a
//	folded one for no reason. The seam is connector-declared precisely so
//	the blast radius is one provider, and these tests pin that boundary:
//	a provider whose connector declares nothing, and a provider with no
//	connector at all, both keep the pass-through behaviour.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-21
package dataplane

import (
	"context"
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// plainConnector is a connector that declares neither optional seam, which is
// what provider.Default is for every provider that needs no special handling.
type plainConnector struct {
	provider.Base
	url string
}

func (c *plainConnector) Endpoint(provider.Request, provider.Credential) (string, error) {
	return c.url, nil
}

func (c *plainConnector) ApplyAuth(*http.Request, provider.Credential) error { return nil }

// TestRelay_ForcedStreamDoesNotApplyToAPlainConnector pins the boundary: a
// connector that declares nothing receives the request the client sent, and its
// answer is read the non-streamed way. This is the control that keeps the fold
// from becoming a change to every provider.
func TestRelay_ForcedStreamDoesNotApplyToAPlainConnector(t *testing.T) {
	var sawStream bool
	server := newStreamingUpstream(t, chatStreamBody, &sawStream)
	defer server.Close()

	entry := registry.Provider{
		ID: "plain", Priority: 1, Category: "api", PassthroughModels: true,
		Transport: registry.Transport{BaseURL: server.URL, Format: registry.DefaultFormat},
	}
	engine := newForcedStreamEngine(t, server.URL, entry, &plainConnector{
		Base: provider.Base{ID: entry.ID, Auth: "no_auth", Format: registry.DefaultFormat},
		url:  server.URL,
	})

	in := relayRequest("plain/big-pickle")
	in.Stream = false

	outcome, err := engine.Relay(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if sawStream {
		t.Fatal("the outbound request asked for a stream, want the client's own flag kept")
	}
	if outcome.Streamed {
		t.Fatal("Outcome.Streamed = true, want the non-streamed path")
	}
	if len(outcome.Body) == 0 {
		t.Fatal("Outcome.Body is empty, want the upstream's own body forwarded")
	}
}

// TestTransport_ForcesStreamIsDeclaredByTheConnector pins the seam's shape: the
// transport answers from the connector, not from the registry entry, so a
// provider whose entry declares a forced stream but whose connector does not is
// left alone. That is the deliberate scope decision: the registry field is
// declared by three providers this change does not touch, and reading it here
// would silently change how their answers are read.
func TestTransport_ForcesStreamIsDeclaredByTheConnector(t *testing.T) {
	entry := registry.Provider{
		ID: "declared", Priority: 1,
		Transport: registry.Transport{BaseURL: "https://example.test/v1", ForceStream: true},
	}
	plain := &plainConnector{
		Base: provider.Base{ID: entry.ID, Auth: "no_auth", Format: registry.DefaultFormat},
		url:  entry.Transport.BaseURL,
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, plain)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: http.DefaultClient})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	if transport.ForcesStream(entry) {
		t.Fatal("ForcesStream() = true, want the connector's declaration to decide, not the entry's")
	}

	// The same entry with a connector that does declare it is forced.
	forced := &forcedStreamConnector{
		Base: provider.Base{ID: entry.ID, Auth: "no_auth", Format: registry.DefaultFormat},
		url:  entry.Transport.BaseURL,
	}
	connectors, err = provider.NewConnectors(provider.DefaultFactory, forced)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err = NewTransport(TransportDeps{Connectors: connectors, Client: http.DefaultClient})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	if !transport.ForcesStream(entry) {
		t.Fatal("ForcesStream() = false, want the connector's declaration honoured")
	}
}

// TestForcedStream_ProviderWithoutAConnectorUsesTheDefault pins that a registry
// entry with no specialized connector is served by provider.Default and therefore
// declares nothing, so registering the OpenCode connector did not change how any
// other provider's answer is read.
func TestForcedStream_ProviderWithoutAConnectorUsesTheDefault(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("loading the embedded registry: %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}

	// Every entry whose registry document declares a forced stream, none of
	// which has a connector in this build. The list is read from the document
	// rather than hand-written, so the 2026-09-26 curation cannot leave this
	// test asserting a removed provider.
	declared := 0
	for _, entry := range index.All() {
		if !entry.Transport.ForceStream {
			continue
		}
		declared++
		id := entry.ID
		if connectors.Registered(id) {
			t.Fatalf("provider %q now has a connector; update this test's premise", id)
		}
		plugin := connectors.For(entry)
		if _, ok := plugin.(provider.StreamForcer); ok {
			t.Fatalf("provider %q declares a forced stream through the default connector, want the registry field left unread", id)
		}
	}
	if declared == 0 {
		t.Fatal("no embedded provider declares force_stream; this test needs an example")
	}
}

// TestForcedStream_RegisteredConnectorDeclaresTheSeam pins the other half of the
// scope: the connector this change registers does declare the forced stream, and
// a provider it serves is no longer reported as unsupported. A provider that is
// listed but unanswerable is the silent trap the boot log exists to surface.
func TestForcedStream_RegisteredConnectorDeclaresTheSeam(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("loading the embedded registry: %v", err)
	}
	entry, ok := index.Provider("opencode")
	if !ok {
		t.Fatal("opencode is missing from the embedded registry")
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, provider.NewOpenCode(entry))
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}

	connector := connectors.For(entry)
	if _, ok := connector.(provider.StreamForcer); !ok {
		t.Fatal("the opencode connector does not declare a forced stream")
	}
	if _, ok := connector.(provider.Transformer); !ok {
		t.Fatal("the opencode connector does not declare a request transform")
	}
	for _, id := range connectors.Unsupported(index) {
		if id == "opencode" {
			t.Fatal("opencode is reported as unsupported, want it served by its connector")
		}
	}
}
