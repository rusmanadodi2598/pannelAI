// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_envelope_test.go
// @for       The transport's use of the envelope seam: an unwrapped body piped to the client, and a refusal inside a 200 reported as a failure.
// @uses      io, net/http, net/http/httptest, provider, strings, testing.
// @reason    A provider that hides its real status in the body only fails properly if the transport reads the first frame before piping. That decision is worth a test of its own: without it, a spent account would stream an error envelope to the client and be recorded as a served answer.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package dataplane

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// envelopeConnector is a provider whose stream arrives wrapped, and whose unwrap is
// scripted by the test.
type envelopeConnector struct {
	provider.Base

	url       string
	unwrapped string
	failure   *provider.StreamFailure
	opens     int
}

func (c *envelopeConnector) Endpoint(_ provider.Request, _ provider.Credential) (string, error) {
	return c.url, nil
}

func (c *envelopeConnector) ApplyAuth(_ *http.Request, _ provider.Credential) error { return nil }

func (c *envelopeConnector) OpenStream(body io.ReadCloser) (io.ReadCloser, *provider.StreamFailure) {
	c.opens++
	if c.failure != nil {
		_ = body.Close()
		return nil, c.failure
	}
	return io.NopCloser(strings.NewReader(c.unwrapped)), nil
}

// newEnvelopeTransport serves `upstream` and routes it through the envelope
// connector, so what a test reads back is what the transport handed on.
func newEnvelopeTransport(t *testing.T, upstream string) (*Transport, *envelopeConnector, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, upstream)
	}))
	t.Cleanup(server.Close)

	connector := &envelopeConnector{
		Base:      provider.Base{ID: "envelope-test", Auth: "apikey", Format: "openai"},
		url:       server.URL,
		unwrapped: "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"clean\"}}]}\n\n",
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, connector)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: server.Client()})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	return transport, connector, server
}

func envelopeCall(stream bool) Call {
	return Call{
		Provider:   registry.Provider{ID: "envelope-test"},
		Model:      registry.Model{ID: "m"},
		Stream:     stream,
		Body:       []byte(`{"model":"m"}`),
		Credential: provider.StaticKey("ep_1", "key_1", "k"),
	}
}

// TestTransportPipesTheUnwrappedBody pins that a provider declaring the seam gets its
// unwrap applied: the client reads what the connector produced, never the envelope.
func TestTransportPipesTheUnwrappedBody(t *testing.T) {
	transport, connector, _ := newEnvelopeTransport(t, "data:{\"body\":\"wrapped\"}\n\n")

	upstream, err := transport.Do(context.Background(), envelopeCall(true))
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() {
		// reason: the test reads the body in full; a close error is not actionable.
		_ = upstream.Body.Close()
	}()

	body, readErr := io.ReadAll(upstream.Body)
	if readErr != nil {
		t.Fatalf("reading the body: %v", readErr)
	}
	if connector.opens != 1 {
		t.Fatalf("the seam ran %d times, want once per streamed call", connector.opens)
	}
	if !strings.Contains(string(body), "clean") || strings.Contains(string(body), "wrapped") {
		t.Fatalf("the client received %q, want the unwrapped frame only", body)
	}
}

// TestTransportReportsAnEnvelopeRefusalAsFailure is the accounting-critical case: the
// HTTP status said 200, the body said the account is spent, and the call must come
// back as an upstream failure a router can park the account for.
func TestTransportReportsAnEnvelopeRefusalAsFailure(t *testing.T) {
	transport, connector, _ := newEnvelopeTransport(t, "data:{\"statusCodeValue\":403}\n\n")
	connector.failure = &provider.StreamFailure{
		Status: http.StatusForbidden, Message: "quota exhausted", Quota: true,
	}

	upstream, err := transport.Do(context.Background(), envelopeCall(true))
	if upstream != nil {
		t.Fatal("a refused envelope still produced a body to serve")
	}
	failure, isUpstream := AsUpstreamError(err)
	if !isUpstream {
		t.Fatalf("err = %v, want an upstream failure the router can classify", err)
	}
	if failure.Status != http.StatusForbidden {
		t.Fatalf("status = %d, want the provider's 403", failure.Status)
	}
	if !strings.Contains(failure.Message, "quota exhausted") {
		t.Fatalf("message = %q, want the provider's reason", failure.Message)
	}
}

// TestTransportLeavesAProviderWithoutTheSeamAlone pins the pass-through: a connector
// that does not declare StreamEnvelope keeps the behaviour every provider had before,
// byte for byte.
func TestTransportLeavesAProviderWithoutTheSeamAlone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "data: plain\n\n")
	}))
	t.Cleanup(server.Close)

	plain := &plainConnector{
		Base: provider.Base{ID: "plain-test", Auth: "apikey", Format: "openai"},
		url:  server.URL,
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, plain)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: server.Client()})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}

	call := envelopeCall(true)
	call.Provider = registry.Provider{ID: "plain-test"}
	upstream, err := transport.Do(context.Background(), call)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() {
		// reason: the body is read in full here.
		_ = upstream.Body.Close()
	}()

	body, readErr := io.ReadAll(upstream.Body)
	if readErr != nil {
		t.Fatalf("reading the body: %v", readErr)
	}
	if string(body) != "data: plain\n\n" {
		t.Fatalf("body = %q, want it untouched", body)
	}
}
