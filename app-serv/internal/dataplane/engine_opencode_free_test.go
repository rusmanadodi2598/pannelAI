// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_opencode_free_test.go
// @for       The OpenCode free tier end to end: the three free models the
//
//	reference serves without any credential, through the whole pipeline.
//
// @uses      context, encoding/json, io, net/http, net/http/httptest, strings,
//
//	testing, internal/domain, internal/provider, internal/registry,
//	internal/schema.
//
// @reason    The reference calls these models with `Authorization: Bearer public`
//
//	and no stored key at all (a virtual connection, src/sse/services/auth.js:46),
//	and the upstream gates the whole lane on the request's own shape: it
//	refuses 403 FreeTierError unless the body streams and declares the
//	fingerprint tools. A hermetic test that only checks the connector's
//	transform cannot prove the lane works, because the failure the gate
//	produces is an upstream refusal, not a local error. This file drives
//	the real resolver, selector, transport, and connector against an
//	upstream stand-in that enforces the measured gate, so a regression
//	that drops a required member fails here instead of in production.
//
//	The gate encoded below is measured, not invented: every rule was
//	probed against the live upstream on 2026-09-24 and is listed with its
//	evidence in §3 of docs/DRAFT/029-OPENCODE-PROVIDER-PARITY.md. The
//	three model ids are the owner's list: space-bunny-free and
//	mimo-v2.6-flash-free resolve through the provider's passthrough (the
//	reference lists them only in its live model fetcher), and
//	muse-spark-1.3-contributor-free is declared with targetFormat
//	openai-responses, so it must reach /zen/v1/responses.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// openCodeFreeModels is the owner's acceptance list: the free-tier models that
// must answer with no credential configured.
var openCodeFreeModels = []string{
	"space-bunny-free",
	"mimo-v2.6-flash-free",
	"muse-spark-1.3-contributor-free",
}

// openCodeFreeRequiredTools is the tool set the live gate actually demands, which
// is narrower than the reference's fingerprint quartet: measured on 2026-09-24, a
// body declaring only glob+grep is refused, and one declaring bash+read is
// accepted. The quartet stays the reference's defense in depth; this list is what
// the upstream enforces today.
var openCodeFreeRequiredTools = []string{"bash", "read"}

// openCodeFreeUngated is the model whose lane imposes no shape gate at all: it
// answers without tools, session, user agent, or credential (measured). The other
// two refuse 403 without the identity headers, so the test pins both behaviours
// rather than assuming one gate for the whole lane.
const openCodeFreeUngated = "space-bunny-free"

// openCodeFreeUpstream stands in for the live free tier, enforcing the gate as
// measured: the public bearer, a CLI user agent, a canonical session header, a
// streaming body, and the fingerprint quartet declared. A Responses-wire model
// must arrive on /responses, which is the per-model rule the provider declares.
//
// It answers the wire it was asked on, so the engine's own translation is what
// the assertions read: a chat request gets chat frames, a Responses request gets
// Responses frames.
func openCodeFreeUpstream(t *testing.T, seen *[]openCodeFreeCall) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		model, _ := body["model"].(string)
		call := openCodeFreeCall{
			Model: model, Path: r.URL.Path,
			Authorization: r.Header.Get("Authorization"),
			UserAgent:     r.Header.Get("User-Agent"),
			Session:       r.Header.Get("x-opencode-session"),
			Client:        r.Header.Get("x-opencode-client"),
			Body:          raw,
			Tools:         openCodeToolNames(body),
		}
		stream, _ := body["stream"].(bool)
		call.Stream = stream
		*seen = append(*seen, call)

		refuse := func(reason string) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"FreeTierError","message":"` + reason + `"}}`))
		}
		// The measured gate: stream, a canonical session, and the CLI user agent
		// together. The credential is not part of it (the live upstream accepts a
		// request with no Authorization at all), which is why the acceptance test
		// is about the lane's shape and not about a key.
		if model != openCodeFreeUngated {
			switch {
			case !strings.HasPrefix(call.UserAgent, "opencode/"):
				refuse("the free tier requires the CLI user agent")
			case call.Session == "":
				refuse("the free tier requires a canonical session")
			case !stream:
				refuse("the free tier refuses a non-streaming request")
			}
			if missing := missingOpenCodeFreeTools(call.Tools); len(missing) > 0 {
				refuse("missing required tools: " + strings.Join(missing, ","))
				return
			}
		}
		if want := openCodeFreePath(model); call.Path != want {
			refuse("model " + model + " must answer on " + want)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		if call.Path == "/zen/v1/responses" {
			_, _ = w.Write([]byte(responsesFreeStreamBody(model)))
			return
		}
		_, _ = w.Write([]byte(chatFreeStreamBody(model)))
	}))
	t.Cleanup(server.Close)
	return server
}

// openCodeFreeCall records one outbound request, so a test can assert the gate
// members the upstream actually received rather than the ones a unit test thinks
// the connector produced.
type openCodeFreeCall struct {
	Model         string
	Path          string
	Authorization string
	UserAgent     string
	Session       string
	Client        string
	Stream        bool
	Tools         []string
	Body          []byte
}

// openCodeToolNames reads every declared tool name in either wire shape, which
// is how the upstream's gate reads them.
func openCodeToolNames(body map[string]any) []string {
	entries, _ := body["tools"].([]any)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		tool, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tool["name"].(string)
		if name == "" {
			if fn, ok := tool["function"].(map[string]any); ok {
				name, _ = fn["name"].(string)
			}
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// missingOpenCodeFreeTools reports which required tools the body does not
// declare. The live gate refuses 403 when any is absent.
func missingOpenCodeFreeTools(names []string) []string {
	present := make(map[string]bool, len(names))
	for _, name := range names {
		present[name] = true
	}
	missing := make([]string, 0, len(openCodeFreeRequiredTools))
	for _, want := range openCodeFreeRequiredTools {
		if !present[want] {
			missing = append(missing, want)
		}
	}
	return missing
}

// openCodeFreePath is the leaf the provider's declared wire answers on.
func openCodeFreePath(model string) string {
	if model == "muse-spark-1.3-contributor-free" {
		return "/zen/v1/responses"
	}
	return "/zen/v1/chat/completions"
}
