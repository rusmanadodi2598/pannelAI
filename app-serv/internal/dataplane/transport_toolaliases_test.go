// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_toolaliases_test.go
// @for       The request-scoped state the tool-name restore rides on: the connector's rename record reaching the answer.
// @uses      context, io, net/http, net/http/httptest, provider, registry, testing.
// @reason    The alias map is born while the connector shapes the outbound request and is read while the answer is translated, so it has to survive the transport. That hop is the seam's one piece of state, and a regression that drops it would silently serve aliases to the caller again, which is exactly the live failure this work fixes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// aliasingConnector is a connector whose request shape renames a caller tool
// and records the rename, as the OpenCode fingerprint pass does.
type aliasingConnector struct {
	provider.Base
	url string
}

func (c *aliasingConnector) Endpoint(provider.Request, provider.Credential) (string, error) {
	return c.url, nil
}

func (c *aliasingConnector) ApplyAuth(*http.Request, provider.Credential) error { return nil }

func (c *aliasingConnector) TransformRequest(req *provider.Request) error {
	req.ToolAliases = map[string]string{"read_client": "read"}
	return nil
}

// TestTransportCarriesTheToolAliasesOntoTheAnswer pins the hop: the map the
// connector records while shaping the request reaches the answer the core
// translates, and a connector that records none leaves the answer with no map,
// which is what keeps the restore a passthrough.
func TestTransportCarriesTheToolAliasesOntoTheAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"index":0,"message":{"tool_calls":[{"id":"c1","function":{"name":"read_client"}}]}}]}`)
	}))
	t.Cleanup(server.Close)

	connectors, err := provider.NewConnectors(provider.DefaultFactory,
		&aliasingConnector{Base: provider.Base{ID: "aliasing-test", Auth: "apikey", Format: "openai"}, url: server.URL},
		&plainConnector{Base: provider.Base{ID: "plain-test", Auth: "apikey", Format: "openai"}, url: server.URL},
	)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: server.Client()})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}

	call := Call{
		Provider:   registry.Provider{ID: "aliasing-test"},
		Model:      registry.Model{ID: "m"},
		Body:       []byte(`{"model":"m"}`),
		Credential: provider.StaticKey("ep_1", "key_1", "k"),
	}
	upstream, err := transport.Do(context.Background(), call)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() {
		// reason: the body is drained here; a close error is not actionable.
		_, _ = io.Copy(io.Discard, upstream.Body)
		_ = upstream.Body.Close()
	}()

	if upstream.ToolAliases["read_client"] != "read" {
		t.Fatalf("ToolAliases = %v, want the connector's rename record", upstream.ToolAliases)
	}

	call.Provider = registry.Provider{ID: "plain-test"}
	plain, err := transport.Do(context.Background(), call)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() {
		// reason: the body is drained here; a close error is not actionable.
		_, _ = io.Copy(io.Discard, plain.Body)
		_ = plain.Body.Close()
	}()
	if plain.ToolAliases != nil {
		t.Fatalf("ToolAliases = %v, want none for a connector that renames nothing", plain.ToolAliases)
	}
}
