// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_body_test.go
// @for       The agent body: what the vendor's catalogue contributes to it, and the
//
//	message reshaping it depends on.
//
// @uses      encoding/json, io, net/http, net/http/httptest, strings, testing, time.
// @reason    The endpoint refuses an OpenAI body and answers with a different model
//
//	when the configuration it is handed is wrong, so the two things this
//	file can pin without a live account are that the payload carries the
//	vendor's own object unchanged and that the identity fields are stable
//	for one prompt. The catalogue stub is the vendor's real shape, captured
//	live.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// qoderCatalogFixture is the vendor's model list, reduced to the two entries a test
// needs. The shape — a `chat` array of objects carrying key, is_reasoning and
// max_output_tokens — is what the live service answered (draft 036 §5).
const qoderCatalogFixture = `{"chat":[
 {"key":"ultimate","format":"openai","source":"system","is_vl":true,"is_reasoning":true,"max_output_tokens":16384,"vendor":"qoder"},
 {"key":"auto","format":"openai","source":"system","is_reasoning":false}
],"inline":[{"key":"other","is_reasoning":false}]}`

// newQoderStubVendor serves the catalogue and the token exchange, counting the
// catalogue reads so a cache can be proven rather than assumed.
func newQoderStubVendor(t *testing.T, catalog string) (*Qoder, *int, *int) {
	t.Helper()
	var lists, exchanges int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case qoderJobTokenExchangePath:
			exchanges++
			_, _ = io.WriteString(w, `{"token":"jt-issued","expires_at":"2099-01-01T00:00:00Z"}`)
		case "/algo" + qoderModelListPath:
			lists++
			if catalog == "" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = io.WriteString(w, catalog)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	entry := qoderEntry("qoder", server.URL, server.URL+"/algo/api/v2/service/pro/sse/agent_chat_generation?FetchKeys=llm_model_result&AgentId=agent_common")
	connector, err := NewQoder(entry, server.Client())
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}
	return connector, &lists, &exchanges
}

func qoderTestCredential() Credential {
	return Credential{EndpointID: "ep_1", APIKey: "pt-secret", Family: FamilyStaticKey,
		ProjectID: "user-7", Account: "dev@example.com",
		Metadata: map[string]string{MetadataMachineID: "machine-fixed"}}
}

func qoderTestRequest(t *testing.T, connector *Qoder, body string) *Request {
	t.Helper()
	return &Request{
		Provider:   connector.entry,
		Model:      registry.Model{ID: "ultimate"},
		Wire:       "openai",
		Stream:     true,
		Body:       []byte(body),
		Credential: qoderTestCredential(),
	}
}

// TestTransformRequestBuildsTheAgentPayload pins the parts the vendor routes on, and
// that the model configuration is the vendor's own object, fields and all.
func TestTransformRequestBuildsTheAgentPayload(t *testing.T) {
	connector, lists, _ := newQoderStubVendor(t, qoderCatalogFixture)

	request := qoderTestRequest(t, connector, `{"model":"qoder/ultimate","messages":[
		{"role":"system","content":"Be terse."},
		{"role":"user","content":[{"type":"text","text":"halo"}]}
	],"tools":[{"type":"function","function":{"name":"x"}}],"max_tokens":1000}`)

	if err := connector.TransformRequest(request); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	var built qoderAgentRequest
	if err := json.Unmarshal(request.Body, &built); err != nil {
		t.Fatalf("the built body is not the agent shape: %v (%s)", err, request.Body)
	}

	if built.ChatTask != "FREE_INPUT" || built.AgentID != "agent_common" || built.SessionType != "qodercli" {
		t.Fatalf("routing fields = %q / %q / %q", built.ChatTask, built.AgentID, built.SessionType)
	}
	if !built.Stream || built.Version != "3" || built.Source != 1 {
		t.Fatalf("flags = %+v, want a streamed v3 source-1 request", built)
	}
	if built.System != "Be terse." {
		t.Fatalf("system = %q, want the lifted system turn", built.System)
	}
	if len(built.Messages) != 1 || string(built.Messages[0].Content) != `"halo"` {
		t.Fatalf("messages = %s, want the text array flattened to a string", built.Messages)
	}
	if len(built.Tools) != 1 {
		t.Fatalf("tools = %s, want the client's tool passed through", built.Tools)
	}
	if built.Parameters.MaxTokens != 1000 {
		t.Fatalf("max_tokens = %d, want the client's ask under the model's 16384", built.Parameters.MaxTokens)
	}
	var config map[string]any
	if err := json.Unmarshal(built.ModelConfig, &config); err != nil {
		t.Fatalf("model_config is not the vendor's object: %v", err)
	}
	if config["vendor"] != "qoder" || config["is_vl"] != true {
		t.Fatalf("model_config lost vendor fields: %s", built.ModelConfig)
	}
	if built.ChatContext.Extra.ModelConfig.Key != "ultimate" || !built.ChatContext.Extra.ModelConfig.IsReasoning {
		t.Fatalf("chat_context model selection = %+v", built.ChatContext.Extra.ModelConfig)
	}
	if built.ChatContext.Text != "halo" || built.Business.Name != "halo" {
		t.Fatalf("echoed turn = %q / %q, want the last user text", built.ChatContext.Text, built.Business.Name)
	}
	if built.Business.Product != "cli" || built.Business.Stage != "start" {
		t.Fatalf("business = %+v", built.Business)
	}
	if built.ImageURLs != nil {
		t.Fatalf("image_urls = %v, want null as the vendor's client sends it", built.ImageURLs)
	}
	if *lists != 1 {
		t.Fatalf("the catalogue was read %d times, want once", *lists)
	}
}

// TestTransformRequestIDsAreStableAndSeparate pins the two identity rules at once:
// the same prompt keeps the record id the vendor dedupes on, and a different prompt
// or a different client ask does not.
func TestTransformRequestIDsAreStableAndSeparate(t *testing.T) {
	connector, _, _ := newQoderStubVendor(t, qoderCatalogFixture)
	first := qoderTestRequest(t, connector, `{"model":"ultimate","messages":[{"role":"user","content":"sama"}]}`)
	second := qoderTestRequest(t, connector, `{"model":"ultimate","messages":[{"role":"user","content":"sama"}]}`)
	third := qoderTestRequest(t, connector, `{"model":"ultimate","messages":[{"role":"user","content":"beda"}]}`)

	if err := connector.TransformRequest(first); err != nil {
		t.Fatalf("first TransformRequest() error = %v", err)
	}
	if err := connector.TransformRequest(second); err != nil {
		t.Fatalf("second TransformRequest() error = %v", err)
	}
	if err := connector.TransformRequest(third); err != nil {
		t.Fatalf("third TransformRequest() error = %v", err)
	}

	var a, b, c qoderAgentRequest
	_ = json.Unmarshal(first.Body, &a)
	_ = json.Unmarshal(second.Body, &b)
	_ = json.Unmarshal(third.Body, &c)

	if a.ChatRecordID != b.ChatRecordID || a.SessionID != b.SessionID {
		t.Fatalf("one prompt produced two identities: %q vs %q", a.ChatRecordID, b.ChatRecordID)
	}
	if a.ChatRecordID == c.ChatRecordID {
		t.Fatalf("two prompts share a record id %q", a.ChatRecordID)
	}
	if a.RequestID == b.RequestID {
		t.Fatalf("one request id reused across calls: %q", a.RequestID)
	}
	for _, id := range []string{a.ChatRecordID, a.SessionID} {
		if len(id) != 16 {
			t.Fatalf("id %q is not the vendor's 16-character form", id)
		}
	}
}

// TestTransformRequestCachesTheCatalogue pins that the vendor is asked once per model
// even across calls, and that an exchanged job token is not exchanged twice.
func TestTransformRequestCachesTheCatalogue(t *testing.T) {
	connector, lists, exchanges := newQoderStubVendor(t, qoderCatalogFixture)

	for i := 0; i < 3; i++ {
		request := qoderTestRequest(t, connector, `{"model":"ultimate","messages":[{"role":"user","content":"x"}]}`)
		if err := connector.TransformRequest(request); err != nil {
			t.Fatalf("TransformRequest() #%d error = %v", i, err)
		}
	}
	if *lists != 1 {
		t.Fatalf("the catalogue was read %d times, want once", *lists)
	}
	if *exchanges != 1 {
		t.Fatalf("the token was exchanged %d times, want once", *exchanges)
	}
}

// TestTransformRequestRefusals pins the three ways a call must be refused before it
// reaches the vendor: a body that is not JSON, a model the vendor does not list, and
// a catalogue that cannot be read.
func TestTransformRequestRefusals(t *testing.T) {
	t.Run("a body that is not json", func(t *testing.T) {
		connector, _, _ := newQoderStubVendor(t, qoderCatalogFixture)
		request := qoderTestRequest(t, connector, "{not json")
		if err := connector.TransformRequest(request); err == nil {
			t.Fatal("an unreadable body was accepted")
		}
	})
	t.Run("a model the vendor does not list", func(t *testing.T) {
		connector, _, _ := newQoderStubVendor(t, qoderCatalogFixture)
		request := qoderTestRequest(t, connector, `{"model":"ultimate","messages":[]}`)
		request.Model = registry.Model{ID: "not-a-model"}
		if err := connector.TransformRequest(request); err == nil {
			t.Fatal("a model with no configuration was sent")
		}
	})
	t.Run("a catalogue that cannot be read", func(t *testing.T) {
		connector, _, _ := newQoderStubVendor(t, "")
		request := qoderTestRequest(t, connector, `{"model":"ultimate","messages":[]}`)
		if err := connector.TransformRequest(request); err == nil {
			t.Fatal("a refused catalogue read still produced a body")
		}
	})
}
