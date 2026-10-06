// Package dataplane implements the request-path data plane of app-serv.
//
// @file      internal/dataplane/transport_envelope_retry_test.go
// @for       The transport's behaviour when a wrapped provider refuses inside the first frame and serves on a later attempt.
// @uses      context, io, net/http, net/http/httptest, strings, testing.
// @reason    Qoder's free model was measured refusing the identical request twice and serving it on the third try (draft 036 §9.2). Whether the gateway repeats a refusal at all is decided by the transport, not the connector, and the non-idempotent POST cap stopped it one attempt short of the answer. That is the whole difference between the provider looking broken and working, so it is pinned end to end here: attempts counted, body handed on, and nothing piped before the answer.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
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

// flakyEnvelopeConnector answers the first `refuseFor` opens with a refusal carrying
// the vendor's nested capacity complaint, then unwraps normally.
type flakyEnvelopeConnector struct {
	provider.Base

	url       string
	refuseFor int
	opens     int
}

func (c *flakyEnvelopeConnector) Endpoint(provider.Request, provider.Credential) (string, error) {
	return c.url, nil
}

func (c *flakyEnvelopeConnector) ApplyAuth(*http.Request, provider.Credential) error { return nil }

func (c *flakyEnvelopeConnector) OpenStream(body io.ReadCloser) (io.ReadCloser, *provider.StreamFailure) {
	c.opens++
	if c.opens <= c.refuseFor {
		_ = body.Close()
		return nil, &provider.StreamFailure{
			Status:  http.StatusTooManyRequests,
			Message: "Workspace allocated quota exceeded, please increase your quota limit.",
		}
	}
	return io.NopCloser(strings.NewReader(
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"PONG\"}}]}\n\n")), nil
}

func TestTransportRetriesARefusalTheClientNeverSaw(t *testing.T) {
	cases := []struct {
		name      string
		refuseFor int
		wantOpen  int
		wantBody  string
		wantErr   bool
	}{
		{
			name: "one refusal is retried and served", refuseFor: 1, wantOpen: 2, wantBody: "PONG",
		},
		{
			// The measured case. It fails without the replayable exception, which
			// caps a POST at one retry and so stops at two opens.
			name:      "two refusals are retried and served on the third attempt",
			refuseFor: 2, wantOpen: 3, wantBody: "PONG",
		},
		{
			// The entry's own budget still bounds it: three refusals on a
			// three-attempt entry ends in the vendor's refusal, not a loop.
			name:      "an upstream that never serves runs out of attempts",
			refuseFor: 9, wantOpen: 3, wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data:{\"body\":\"wrapped\"}\n\n")
			}))
			t.Cleanup(server.Close)

			connector := &flakyEnvelopeConnector{
				Base:      provider.Base{ID: "flaky-envelope", Auth: "apikey", Format: "openai"},
				url:       server.URL,
				refuseFor: tc.refuseFor,
			}
			connectors, err := provider.NewConnectors(provider.DefaultFactory, connector)
			if err != nil {
				t.Fatalf("NewConnectors() error = %v", err)
			}
			transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: server.Client()})
			if err != nil {
				t.Fatalf("NewTransport() error = %v", err)
			}

			call := Call{
				Provider: registry.Provider{
					ID:        "flaky-envelope",
					Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}},
				},
				Model: registry.Model{ID: "qfmodel"}, Stream: true,
				Body:       []byte(`{"model":"flaky-envelope/qfmodel"}`),
				Credential: provider.StaticKey("ep_1", "k1", "pt-token"),
			}
			upstream, doErr := transport.Do(context.Background(), call)

			if connector.opens != tc.wantOpen {
				t.Fatalf("opens = %d, want %d", connector.opens, tc.wantOpen)
			}
			if tc.wantErr {
				if doErr == nil {
					t.Fatal("Do() succeeded, want the upstream refusal after the budget ran out")
				}
				if !strings.Contains(doErr.Error(), "Workspace allocated quota exceeded") {
					t.Fatalf("error = %v, want the vendor's own sentence carried to the caller", doErr)
				}
				return
			}
			if doErr != nil {
				t.Fatalf("Do() error = %v, want the served answer", doErr)
			}
			defer func() {
				// reason: the body is read in full below.
				_ = upstream.Body.Close()
			}()
			answer, readErr := io.ReadAll(upstream.Body)
			if readErr != nil {
				t.Fatalf("reading the answer: %v", readErr)
			}
			if !strings.Contains(string(answer), tc.wantBody) {
				t.Fatalf("answer = %q, want it to carry %q", string(answer), tc.wantBody)
			}
			if strings.Contains(string(answer), "wrapped") {
				t.Fatalf("the envelope reached the caller: %s", string(answer))
			}
		})
	}
}
