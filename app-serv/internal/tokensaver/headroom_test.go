// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/headroom_test.go
// @for       The external client's cases: the payload it sends, the answer it
//
//	accepts, and every failure it must report rather than swallow.
//
// @uses      context, encoding/json, io, net/http, net/http/httptest, testing.
//
// @reason    SPEC-API-001 §7.9 makes the saver fail open, and fail-open is only
//
//	safe when the caller is told which calls failed. TDD.md §2.5 also
//	asks for the boundaries: an empty array, a non-array answer, a
//	refusal, an unreachable proxy, and a proxy that never answers.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// headroomServer is a proxy stub that records what it received and answers with
// the case's status and body.
type headroomServer struct {
	server   *httptest.Server
	path     string
	query    string
	body     string
	requests int
}

// newHeadroomServer starts a stub that answers every call the same way.
func newHeadroomServer(t *testing.T, status int, answer string) *headroomServer {
	t.Helper()
	stub := &headroomServer{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.requests++
		stub.path = r.URL.Path
		stub.query = r.URL.RawQuery
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the request: %v", err)
		}
		stub.body = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

// TestHeadroomClient_Compress drives the calls the client accepts, one proxy
// answer per case.
func TestHeadroomClient_Compress(t *testing.T) {
	compressed := `{"messages":[{"role":"user","content":"less"}],"tokens_before":100,"tokens_after":40,"tokens_saved":60}`
	cases := []struct {
		name            string
		basePath        string
		query           string
		status          int
		answer          string
		compressUsers   bool
		wantPath        string
		wantConfig      bool
		wantErr         bool
		wantMessages    string
		wantTokensSaved int
	}{
		{
			name:            "a compressed answer",
			status:          http.StatusOK,
			answer:          compressed,
			wantPath:        "/v1/compress",
			wantMessages:    `[{"role":"user","content":"less"}]`,
			wantTokensSaved: 60,
		},
		{
			name:            "the user-message flag is sent when it is on",
			status:          http.StatusOK,
			answer:          compressed,
			compressUsers:   true,
			wantPath:        "/v1/compress",
			wantConfig:      true,
			wantMessages:    `[{"role":"user","content":"less"}]`,
			wantTokensSaved: 60,
		},
		{
			name:            "a configured base path is kept",
			basePath:        "/proxy/",
			status:          http.StatusOK,
			answer:          compressed,
			wantPath:        "/proxy/v1/compress",
			wantMessages:    `[{"role":"user","content":"less"}]`,
			wantTokensSaved: 60,
		},
		{
			name:            "a configured query is kept",
			query:           "token=abc",
			status:          http.StatusOK,
			answer:          compressed,
			wantPath:        "/v1/compress",
			wantMessages:    `[{"role":"user","content":"less"}]`,
			wantTokensSaved: 60,
		},
		{
			name:         "an answer without the token accounting",
			status:       http.StatusOK,
			answer:       `{"messages":[]}`,
			wantPath:     "/v1/compress",
			wantMessages: `[]`,
		},
		{
			name:    "a proxy that refuses",
			status:  http.StatusInternalServerError,
			answer:  `{"error":"boom"}`,
			wantErr: true,
		},
		{
			name:    "a proxy that does not know the endpoint",
			status:  http.StatusNotFound,
			answer:  `not found`,
			wantErr: true,
		},
		{
			name:    "an answer that is not JSON",
			status:  http.StatusOK,
			answer:  `<html>proxy</html>`,
			wantErr: true,
		},
		{
			name:    "an answer without a messages member",
			status:  http.StatusOK,
			answer:  `{"tokens_saved":10}`,
			wantErr: true,
		},
		{
			name:    "an answer whose messages member is not an array",
			status:  http.StatusOK,
			answer:  `{"messages":{"role":"user"}}`,
			wantErr: true,
		},
		{
			name:    "an answer whose messages member is null",
			status:  http.StatusOK,
			answer:  `{"messages":null}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newHeadroomServer(t, tc.status, tc.answer)
			url := stub.server.URL + tc.basePath
			if tc.query != "" {
				url += "?" + tc.query
			}
			client := NewHeadroomClient(stub.server.Client())
			result, err := client.Compress(context.Background(), HeadroomRequest{
				URL:                  url,
				Model:                "m",
				Messages:             json.RawMessage(`[{"role":"user","content":"much longer"}]`),
				CompressUserMessages: tc.compressUsers,
			})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Compress() error = nil, want a refusal; result = %+v", result)
				}
				return
			}
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}
			if stub.path != tc.wantPath {
				t.Fatalf("path = %q, want %q", stub.path, tc.wantPath)
			}
			if tc.query != "" && stub.query != tc.query {
				t.Fatalf("query = %q, want %q", stub.query, tc.query)
			}
			if got := string(result.Messages); got != tc.wantMessages {
				t.Fatalf("messages = %s, want %s", got, tc.wantMessages)
			}
			if result.TokensSaved != tc.wantTokensSaved {
				t.Fatalf("tokens saved = %d, want %d", result.TokensSaved, tc.wantTokensSaved)
			}
			assertHeadroomPayload(t, stub.body, tc.wantConfig)
		})
	}
}

// assertHeadroomPayload checks the body the proxy received: the message array,
// the model, and whether the optional config member is there.
func assertHeadroomPayload(t *testing.T, body string, wantConfig bool) {
	t.Helper()
	payload := envelopeOf(t, []byte(body))
	if _, ok := payload.memberString("model"); !ok {
		t.Fatalf("the payload carries no model: %s", body)
	}
	if _, ok := decodeArray(payload["messages"]); !ok {
		t.Fatalf("the payload carries no message array: %s", body)
	}
	config, present := payload["config"]
	if present != wantConfig {
		t.Fatalf("config present = %v, want %v: %s", present, wantConfig, body)
	}
	if !wantConfig {
		return
	}
	options := envelopeOf(t, config)
	if !options.memberTrue("compress_user_messages") {
		t.Fatalf("config = %s, want compress_user_messages true", config)
	}
}
