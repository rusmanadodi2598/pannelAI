// Package provider holds the per-provider plugin seam: the connector that decides
// how one provider is authenticated and addressed.
//
// @file      internal/provider/gemini_cloudcode_test.go
// @for       The Cloud Code Assist families: the envelope their endpoint reads, the URL that carries no model, and the client identification the vendor expects.
// @uses      encoding/json, net/http, strings, testing, internal/registry.
// @reason    Two Google families send the same generateContent payload inside a project-keyed envelope, and the endpoint they address names no model at all, so neither the plain Gemini connector nor the OpenAI fallback describes them. Pinned here: the envelope's exact membership including the member that must stay absent when the account stores no project, the wire model that arrives with a thinking suffix its reader has already consumed, and the two identity members the registry entry declares and had no reader until now. What one project's session id does across retries is pinned in gemini_cloudcode_requestid_test.go.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-18
package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

const cloudcodeBase = "https://cloudcode-pa.googleapis.com/v1internal"

func cloudcodeEntry(id string) registry.Provider {
	return registry.Provider{
		ID: id,
		Transport: registry.Transport{
			Format:     id,
			BaseURL:    cloudcodeBase,
			CLIVersion: "0.34.0",
			APIClient:  "google-genai-sdk/1.41.0 gl-node/v22.19.0",
			Auth: registry.AuthConfig{
				OAuth: &registry.AuthScheme{Header: "Authorization", Scheme: "bearer"},
			},
		},
	}
}

func cloudcodeCredential(t *testing.T, project string) Credential {
	t.Helper()
	input := CredentialInput{
		EndpointID: "ep-1", KeyID: "key-1", AccessToken: "ya29.tok", Family: FamilyOAuth,
		Account: "dev@example.com",
	}
	if project != "" {
		input.Metadata = map[string]string{MetadataProjectID: project}
	}
	credential, err := NewCredential(input)
	if err != nil {
		t.Fatalf("building the credential: %v", err)
	}
	return credential
}

func cloudcodeRequest(t *testing.T, entry registry.Provider, contents int, stream bool, project string) Request {
	t.Helper()
	body := `{"contents":[{"role":"user","parts":[{"text":"ping"}]}`
	for i := 1; i < contents; i++ {
		body += `,{"role":"model","parts":[{"text":"pong"}]}`
	}
	body += `],"generationConfig":{"temperature":0.5}}`

	return Request{
		Provider:   entry,
		Wire:       "gemini",
		Model:      registry.Model{ID: "gemini-2.5-flash"},
		Stream:     stream,
		Body:       []byte(body),
		Headers:    http.Header{},
		Credential: cloudcodeCredential(t, project),
	}
}

// cloudcodeWrapped runs the seam and reads the members it wrote, so each test
// asserts on the outbound body rather than on the connector's own opinion.
func cloudcodeWrapped(t *testing.T, connector *CloudCode, entry registry.Provider, contents int, project string) map[string]json.RawMessage {
	t.Helper()
	request := cloudcodeRequest(t, entry, contents, false, project)
	if err := connector.TransformRequest(&request); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(request.Body, &members); err != nil {
		t.Fatalf("the outbound body is not an object: %v (%s)", err, request.Body)
	}
	return members
}

func TestCloudCodeEndpoint_AddressesTheActionWithoutAModelSegment(t *testing.T) {
	entry := cloudcodeEntry("gemini-cli")
	connector := NewCloudCode(entry, CloudCodeGeminiCLI)

	cases := []struct {
		name   string
		stream bool
		want   string
	}{
		{name: "a JSON answer", want: cloudcodeBase + ":generateContent"},
		{name: "a stream", stream: true, want: cloudcodeBase + ":streamGenerateContent?alt=sse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := cloudcodeRequest(t, entry, 1, tc.stream, "proj-9")
			url, err := connector.Endpoint(request, request.Credential)
			if err != nil {
				t.Fatalf("Endpoint() error = %v", err)
			}
			if url != tc.want {
				t.Fatalf("url = %q, want %q: this endpoint names no model in the path", url, tc.want)
			}
		})
	}
}

func TestCloudCodeEnvelope_CarriesProjectModelAndInnerBody(t *testing.T) {
	entry := cloudcodeEntry("gemini-cli")
	members := cloudcodeWrapped(t, NewCloudCode(entry, CloudCodeGeminiCLI), entry, 2, "proj-9")

	if string(members["project"]) != `"proj-9"` {
		t.Fatalf("project = %s, want the account's stored project", members["project"])
	}
	if string(members["model"]) != `"gemini-2.5-flash"` {
		t.Fatalf("model = %s, want the resolved upstream id: the path does not carry it here", members["model"])
	}

	var inner map[string]any
	if err := json.Unmarshal(members["request"], &inner); err != nil {
		t.Fatalf("the envelope's request member is not an object: %v", err)
	}
	if _, ok := inner["contents"]; !ok {
		t.Fatalf("inner request = %v, want the translated Gemini payload inside", inner)
	}
	if cfg, ok := inner["generationConfig"].(map[string]any); !ok || cfg["temperature"] != 0.5 {
		t.Fatalf("inner generationConfig = %v, want the translated settings kept intact", inner["generationConfig"])
	}
	// The wrap happens after translation, so an OpenAI member reaching the inner
	// body would mean the envelope was built from the client's request instead.
	if _, ok := inner["messages"]; ok {
		t.Fatal("inner request carries `messages`, want the Gemini shape only")
	}
}

// TestCloudCodeEnvelope_OmitsAnUnknownProject holds the line the reference does not
// keep: it invents an identifier when the account has no project, and a fabricated
// one is a value the vendor never granted. Absent is the honest shape, and the
// refusal it earns is a signal rather than a made-up number.
func TestCloudCodeEnvelope_OmitsAnUnknownProject(t *testing.T) {
	entry := cloudcodeEntry("gemini-cli")
	members := cloudcodeWrapped(t, NewCloudCode(entry, CloudCodeGeminiCLI), entry, 1, "")

	if _, ok := members["project"]; ok {
		t.Fatalf("envelope = %v, want no project member for an account that stores none", members)
	}
}

func TestCloudCodeEnvelope_GeminiCLISendsNoAgentMembers(t *testing.T) {
	entry := cloudcodeEntry("gemini-cli")
	members := cloudcodeWrapped(t, NewCloudCode(entry, CloudCodeGeminiCLI), entry, 1, "proj-9")

	for _, member := range []string{"userAgent", "requestId", "requestType"} {
		if _, ok := members[member]; ok {
			t.Fatalf("envelope carries %q, which only the IDE product sends", member)
		}
	}
}

// TestCloudCodeEnvelope_AntigravityAddsItsAttributionMembers pins the two members
// the IDE product sends beside the payload and the id's shape: a conversation, a
// millisecond instant, a trajectory, and a step the conversation length decides.
// Nothing about the two ids is random, which is what lets one turn attribute
// identically across a retry.
func TestCloudCodeEnvelope_AntigravityAddsItsAttributionMembers(t *testing.T) {
	entry := cloudcodeEntry("antigravity")
	connector := NewCloudCode(entry, CloudCodeAntigravity)
	members := cloudcodeWrapped(t, connector, entry, 3, "proj-9")

	if string(members["userAgent"]) != `"antigravity"` {
		t.Fatalf("userAgent = %s, want the product the endpoint reads", members["userAgent"])
	}
	id := strings.Trim(string(members["requestId"]), `"`)
	parts := strings.Split(id, "/")
	if len(parts) != 5 || parts[0] != "agent" {
		t.Fatalf("requestId = %q, want agent/<conversation>/<ms>/<trajectory>/<step>", id)
	}
	if !isAllDigits(parts[2]) {
		t.Fatalf("requestId instant = %q, want milliseconds since the epoch", parts[2])
	}
	if parts[4] != "5" {
		t.Fatalf("requestId step = %q, want 5 for a three-turn conversation", parts[4])
	}
	if !looksLikeUUID(parts[1]) || !looksLikeUUID(parts[3]) {
		t.Fatalf("requestId = %q, want both ids in the uuid shape the endpoint parses", id)
	}

	retried := cloudcodeWrapped(t, connector, entry, 3, "proj-9")
	retryParts := strings.Split(strings.Trim(string(retried["requestId"]), `"`), "/")
	if len(retryParts) != 5 {
		t.Fatalf("retry requestId = %s, want the same five-part shape", retried["requestId"])
	}
	if retryParts[1] != parts[1] || retryParts[3] != parts[3] {
		t.Fatalf("ids moved between two calls of one account (%q vs %q): one turn would attribute as two conversations",
			id, retried["requestId"])
	}
}

// TestCloudCodeEnvelope_StripsTheThinkingLevelFromTheWireModel pins the vendor rule
// the registry data cannot carry: antigravity's declared upstream ids end in a
// "(level)" because that is how the catalog names the thinking tier, but Cloud Code
// Assist answers 404 for a model string that still has it. The level has already been
// applied to the request's thinking budget by the time the id reaches here.
func TestCloudCodeEnvelope_StripsTheThinkingLevelFromTheWireModel(t *testing.T) {
	entry := cloudcodeEntry("antigravity")
	request := cloudcodeRequest(t, entry, 1, false, "proj-9")
	request.Model = registry.Model{ID: "gemini-3.8-flash-high", UpstreamModelID: "gemini-3.8-flash-high(high)"}

	if err := NewCloudCode(entry, CloudCodeAntigravity).TransformRequest(&request); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	members := envelopeMembers(t, request.Body)
	if got := strings.Trim(string(members["model"]), `"`); got != "gemini-3.8-flash-high" {
		t.Fatalf("model = %q, want the parenthetical level stripped", got)
	}
}

// envelopeMembers reads one outbound body as top-level JSON members.
func envelopeMembers(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil {
		t.Fatalf("the outbound body is not an object: %v (%s)", err, body)
	}
	return members
}
