// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_connector_fixture_test.go
// @for       The registry entries, credentials and stub vendor the Qoder connector tests share.
// @uses      encoding/json, net/http, net/http/httptest, testing, time, internal/registry.
// @reason    Two connector test files read the same fixtures, an intl entry with its device host, a CN entry with one gateway, and a stub that answers the token exchange, and a connector's behaviour depends on which of those it was built for, so the shapes live in one place rather than drifted apart in two.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

const (
	qoderChatURLIntl = "https://api3.qoder.sh/algo/api/v2/service/pro/sse/agent_chat_generation" +
		"?FetchKeys=llm_model_result&AgentId=agent_common"
	qoderChatURLCN = "https://gateway.qoder.com.cn/algo/api/v2/service/pro/sse/agent_chat_generation"
)

// qoderEntry builds a registry entry shaped like the qoder one: an openapi base for
// the exchange and a full chat URL as the transport base.
func qoderEntry(id, openAPIBase, chatURL string) registry.Provider {
	return registry.Provider{
		ID:        id,
		Category:  "oauth",
		AuthType:  "oauth",
		AuthModes: []string{"oauth", "apikey"},
		Transport: registry.Transport{Format: "openai", BaseURL: chatURL},
		OAuth:     &registry.OAuth{OpenAPIBaseURL: openAPIBase},
	}
}

// newQoderTestConnector wires a connector to a stub vendor whose exchange endpoint
// answers every exchange with a fresh job token.
func newQoderTestConnector(t *testing.T, entry registry.Provider, exchanges *int) *Qoder {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != qoderJobTokenExchangePath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if exchanges != nil {
			*exchanges++
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "jt-issued",
			"expires_at": time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		})
	}))
	t.Cleanup(server.Close)

	connector, err := NewQoder(entry, server.Client())
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}
	if entry.OAuth != nil {
		// The stub is the openapi base for the test, so the exchange goes there.
		connector.tokens, _ = NewQoderJobTokenClient(server.URL, server.Client())
	}
	return connector
}

func qoderPATCredential() Credential {
	return Credential{endpointID: "ep_1", apiKey: "pt-secret", family: FamilyStaticKey,
		projectID: "user-7", account: "dev@example.com",
		metadata: map[string]string{MetadataMachineID: "machine-fixed"}}
}
