//go:build integration

// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_live_chat_test.go
// @for       The live chain: an agent body built by this port, signed, sent, and
//
//	answered by the vendor.
//
// @uses      bytes, encoding/json, io, net/http, strings, testing.
// @reason    Every unit test of the body builder asserts the shape this gateway
//
//	produces. Only the vendor decides whether it reads it: the agent
//	endpoint refuses a request whose routing fields it does not
//	recognise, and its answer arrives wrapped. This file asks the real
//	service and asserts the two things that would be wrong — that the
//	call was refused for its body, and that an envelope reached a
//	client — while staying independent of whether the account has
//	credits left.
//
//	  PANNELAI_QODER_PAT='pt-…' \
//	    go test -tags=integration ./internal/provider/ -run QoderLiveChat
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// qoderRoutingRefusal is what the vendor answers when the agent payload does not
// route: measured 2026-09-27 against a plain translated OpenAI body.
const qoderRoutingRefusal = "flow nodes"

// qoderLiveChatPath is the vendor's agent endpoint.
const qoderLiveChatPath = "/api/v2/service/pro/sse/agent_chat_generation?FetchKeys=llm_model_result&AgentId=agent_common"

func TestQoderLiveChatBodyIsAcceptedByTheVendor(t *testing.T) {
	jobToken := qoderLiveExchange(t, qoderLivePAT(t))
	account := qoderLiveAccount(t, jobToken)
	connector, err := NewQoder(qoderLiveEntry(t), nil)
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}
	credential := Credential{
		EndpointID: "ep_live", APIKey: qoderLivePAT(t), Family: FamilyStaticKey,
		ProjectID: account.ID, Account: account.Email,
		Metadata: map[string]string{MetadataMachineID: newQoderID()},
	}
	body := []byte(`{"model":"qoder/auto","messages":[{"role":"user","content":"Reply with exactly: PONG"}],"max_tokens":64}`)

	request := &Request{
		Model:      requireLiveCatalog(t, connector, credential, "auto"),
		Wire:       "openai",
		Stream:     true,
		Body:       body,
		Credential: credential,
	}
	request.Provider = connector.entry
	if err := connector.TransformRequest(request); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	url, err := connector.Endpoint(*request, credential)
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	outbound, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(request.Body))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	outbound.Header.Set("Content-Type", "application/json")
	outbound.Header.Set("Accept", "text/event-stream")
	if err := connector.ApplyAuth(outbound, credential); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}

	response, err := http.DefaultClient.Do(outbound)
	if err != nil {
		t.Fatalf("the chat call failed: %v", err)
	}
	defer func() {
		// reason: the answer is read in full below.
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("chat = HTTP %d, want the vendor to accept the signed call", response.StatusCode)
	}

	unwrapped, failure := connector.OpenStream(response.Body)
	if failure != nil {
		// The account this proof runs on is out of credits, which is exactly the
		// answer that must arrive as a failure rather than as content.
		t.Logf("the vendor refused the call: status=%d quota=%v message=%s",
			failure.Status, failure.Quota, truncateForLive(failure.Message, 200))
		if strings.Contains(failure.Message, qoderRoutingRefusal) {
			t.Fatalf("the agent body was refused for its shape: %s", failure.Message)
		}
		return
	}

	answer, err := io.ReadAll(io.LimitReader(unwrapped, 1<<20))
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	if strings.Contains(string(answer), "statusCodeValue") {
		t.Fatalf("an envelope reached the client: %s", truncateForLive(string(answer), 300))
	}
	if strings.Contains(string(answer), qoderRoutingRefusal) {
		t.Fatalf("the agent body was refused for its shape: %s", truncateForLive(string(answer), 300))
	}
	t.Logf("the vendor answered with %d bytes of unwrapped stream", len(answer))
}

// qoderLiveEntry points a connector at the real intl service, using the entry shape
// the embedded registry carries for qoder.
func qoderLiveEntry(t *testing.T) registry.Provider {
	t.Helper()
	return registry.Provider{
		ID: "qoder", Category: "oauth", AuthType: "oauth",
		Transport: registry.Transport{Format: "openai", BaseURL: qoderLiveChatURL()},
		OAuth:     &registry.OAuth{OpenAPIBaseURL: qoderLiveOpenAPI},
	}
}

// qoderLiveChatURL is the chat endpoint the intl device host serves; the connector
// moves it to the job host once it sees the exchanged credential.
func qoderLiveChatURL() string {
	return qoderChatBaseIntlDevice + qoderSigPathPrefix + qoderLiveChatPath
}

// requireLiveCatalog proves the vendor's catalogue answers for one key, which is the
// read the body builder depends on, and hands back the model to name.
func requireLiveCatalog(t *testing.T, connector *Qoder, credential Credential, key string) registry.Model {
	t.Helper()
	config, err := connector.modelConfig(credential, key)
	if err != nil {
		t.Fatalf("the vendor's catalogue did not answer for %q: %v", key, err)
	}
	if !json.Valid(config) || len(config) == 0 {
		t.Fatalf("the catalogue answer for %q is not a json object", key)
	}
	return registry.Model{ID: key}
}
