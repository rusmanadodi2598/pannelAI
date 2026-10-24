// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_cloudcode_wire_test.go
// @for       One Cloud Code Assist call as it actually leaves the gateway: the envelope on the wire and the model-less address.
// @uses      context, encoding/json, io, net/http, net/http/httptest, strings, testing, internal/provider, internal/registry, internal/schema.
// @reason    The connector's own test can show the envelope it builds, but only a wire test shows the seam ran before the address was chosen and that the answer inside the vendor's `response` wrapper reached a client as text with its tokens counted. Those are the two orders this family can get wrong while still answering 200.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-18
package dataplane

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// cloudcodeWireAnswer is what the OAuth endpoint sends back: the same candidate
// answer a plain generateContent call would, nested in the product's wrapper.
const cloudcodeWireAnswer = `{"response":{"candidates":[{"content":{"role":"model",` +
	`"parts":[{"text":"pong"}]},"finishReason":"STOP"}],` +
	`"usageMetadata":{"promptTokenCount":13,"candidatesTokenCount":4,"totalTokenCount":17}}}`

type cloudcodeWireCall struct {
	path   string
	body   string
	header http.Header
}

func cloudcodeWireUpstream(t *testing.T, captured *cloudcodeWireCall) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("the upstream body could not be read: %v", err)
		}
		captured.path = r.URL.Path + r.URL.RawQuery
		captured.body = string(raw)
		captured.header = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cloudcodeWireAnswer))
	}))
	t.Cleanup(server.Close)
	return server
}

func cloudcodeWireEntry(url, id string) registry.Provider {
	return registry.Provider{
		ID: id, Priority: 20, Category: "oauth", PassthroughModels: true,
		Transport: registry.Transport{
			BaseURL:    url + "/v1internal",
			Format:     id,
			CLIVersion: "0.34.0",
			APIClient:  "google-genai-sdk/1.41.0",
			Auth: registry.AuthConfig{
				OAuth: &registry.AuthScheme{Header: "Authorization", Scheme: "bearer"},
			},
		},
	}
}

// cloudcodeWireEngine wires one family's connector over an OAuth account that
// stores a project, which is what the envelope is keyed to and what an api_key
// account never has.
func cloudcodeWireEngine(t *testing.T, serverURL string, entry registry.Provider, product provider.CloudCodeProduct) *Engine {
	t.Helper()
	resolver, err := NewResolver(
		forcedStreamRegistry{entry: entry},
		relayLookup{combos: map[string]domain.Combo{}},
	)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, provider.NewCloudCode(entry, product))
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}

	endpoint, err := domain.NewUpstreamEndpoint(
		"ep-"+entry.ID, entry.ID, "account", domain.UpstreamAuthOAuth, 1, now)
	if err != nil {
		t.Fatalf("building the oauth endpoint: %v", err)
	}
	endpoint.SetOAuth(domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{
		AccessTokenEncrypted: "at-cipher", ProjectID: "proj-9",
	}), now)
	endpoint.SetAccount(domain.RehydrateEndpointAccount(domain.EndpointAccountInput{
		Email: "dev@example.com",
	}), now)

	repo := newMemEndpointRepo()
	repo.byProvider[entry.ID] = []domain.UpstreamEndpoint{endpoint}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: http.DefaultClient})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	engine, err := NewEngine(EngineDeps{Resolver: resolver, Selector: selector, Transport: transport})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return engine
}

// TestCloudCodeWire_EnvelopeAndAddressLeaveTogether pins both halves of this
// family's request in one call: the seam must have wrapped the payload before the
// address was chosen, and the address must not name the model the body now carries.
func TestCloudCodeWire_EnvelopeAndAddressLeaveTogether(t *testing.T) {
	var captured cloudcodeWireCall
	server := cloudcodeWireUpstream(t, &captured)
	entry := cloudcodeWireEntry(server.URL, "gemini-cli")
	engine := cloudcodeWireEngine(t, server.URL, entry, provider.CloudCodeGeminiCLI)

	if _, err := engine.Relay(context.Background(), geminiWireRequest(t, "gemini-cli/gemini-2.5-flash", false), nil); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	if want := "/v1internal:generateContent"; captured.path != want {
		t.Fatalf("upstream path = %q, want %q: this endpoint takes no model segment", captured.path, want)
	}

	var envelope struct {
		Project string          `json:"project"`
		Model   string          `json:"model"`
		Request json.RawMessage `json:"request"`
	}
	if err := json.Unmarshal([]byte(captured.body), &envelope); err != nil {
		t.Fatalf("the outbound body is not an envelope: %v (%s)", err, captured.body)
	}
	if envelope.Model != "gemini-2.5-flash" {
		t.Fatalf("envelope model = %q, want the resolved id carried in the body", envelope.Model)
	}
	if !strings.Contains(string(envelope.Request), `"contents"`) {
		t.Fatalf("envelope request = %s, want the translated Gemini payload inside", envelope.Request)
	}
	if strings.Contains(captured.body, `"messages"`) {
		t.Fatalf("envelope = %s, want no OpenAI member anywhere on the wire", captured.body)
	}

	// The client identification the entry declares finally has a reader.
	if got := captured.header.Get("X-Goog-Api-Client"); got != entry.Transport.APIClient {
		t.Fatalf("X-Goog-Api-Client = %q, want the declared SDK client", got)
	}
	if got := captured.header.Get("User-Agent"); got != "gemini-cli/0.34.0" {
		t.Fatalf("User-Agent = %q, want the declared CLI version", got)
	}
	if got := captured.header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
		t.Fatalf("Authorization = %q, want the bearer this family authenticates with", got)
	}
}

// TestCloudCodeWire_WrappedAnswerReachesTheClient is the half a request-side test
// cannot show: the answer arrives nested, so a reader that only knows the flat
// shape would hand the caller an empty completion and bill it at zero.
func TestCloudCodeWire_WrappedAnswerReachesTheClient(t *testing.T) {
	var captured cloudcodeWireCall
	server := cloudcodeWireUpstream(t, &captured)
	entry := cloudcodeWireEntry(server.URL, "gemini-cli")
	engine := cloudcodeWireEngine(t, server.URL, entry, provider.CloudCodeGeminiCLI)

	outcome, err := engine.Relay(context.Background(), geminiWireRequest(t, "gemini-cli/gemini-2.5-flash", false), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	var answer schema.ChatCompletionResponse
	if err := json.Unmarshal(outcome.Body, &answer); err != nil {
		t.Fatalf("the answer is not a chat completion: %v (body=%s)", err, outcome.Body)
	}
	if len(answer.Choices) != 1 || answer.Choices[0].Message.Content.Text != "pong" {
		t.Fatalf("answer = %s, want the candidate inside the vendor's wrapper", outcome.Body)
	}
	if outcome.Usage == nil || outcome.Usage.PromptTokens != 13 || outcome.Usage.TotalTokens != 17 {
		t.Fatalf("usage = %+v, want the numbers that arrived inside the wrapper", outcome.Usage)
	}
}
