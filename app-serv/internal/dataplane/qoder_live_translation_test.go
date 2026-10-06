//go:build integration && live

// Package dataplane implements the request-path data plane of app-serv.
//
// @file      internal/dataplane/qoder_live_translation_test.go
// @for       One real Qoder stream run through the gateway's own translation, to pin the model name the caller ends up seeing.
// @uses      bytes, context, io, net/http, os, strings, testing, time, internal/provider.
// @reason    The bug lived between two packages: Qoder answers every model as `auto`, and the translation step adopted that echo. A unit test on each side could pass while a client still read the wrong name, so this runs the whole leg once against the vendor, sign, send, unwrap, translate, and asks the only question a client can: what does the answer call itself?
//
//	PANNELAI_QODER_PAT='pt-…' \
//	  go test -tags=integration,live ./internal/dataplane/ -run QoderLiveTranslation
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package dataplane

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestQoderLiveTranslationNamesTheRequestedModel streams Qwen3.8-Flash for real and
// feeds the unwrapped answer through the state that frames a client's stream.
func TestQoderLiveTranslationNamesTheRequestedModel(t *testing.T) {
	pat := strings.TrimSpace(os.Getenv("PANNELAI_QODER_PAT"))
	if pat == "" {
		t.Fatal("PANNELAI_QODER_PAT must be set for the live translation proof")
	}

	index, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	entry, ok := index.Provider("qoder")
	if !ok {
		t.Fatal("the embedded registry carries no qoder entry")
	}
	connector, err := provider.NewQoder(entry, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}

	// A token-only credential: this is a PAT pasted into the panel, which stores
	// nothing but the token, and the model the caller asked for is the one that has
	// to come back.
	credential := provider.StaticKey("ep_live", "", pat)
	request := &provider.Request{
		Model: registry.Model{ID: "qfmodel"}, Wire: "openai", Stream: true,
		Body: []byte(`{"model":"qoder/qfmodel","messages":[{"role":"user",` +
			`"content":"Reply with exactly: PONG"}],"max_tokens":48}`),
		Credential: credential,
	}
	request.Provider = entry
	if err := connector.TransformRequest(request); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	url, err := connector.Endpoint(*request, credential)
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}

	outbound, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, url, bytes.NewReader(request.Body))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	outbound.Header.Set("Content-Type", "application/json")
	outbound.Header.Set("Accept", "text/event-stream")
	if err := connector.ApplyAuth(outbound, credential); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}

	response, err := (&http.Client{Timeout: 90 * time.Second}).Do(outbound)
	if err != nil {
		t.Fatalf("the chat call failed: %v", err)
	}
	defer func() {
		// reason: the answer is read in full below.
		_ = response.Body.Close()
	}()

	unwrapped, failure := connector.OpenStream(response.Body)
	if failure != nil {
		if failure.Status == http.StatusTooManyRequests {
			// The free pool runs short and recovers, measured 2026-09-28, the same
			// request refused here was served minutes later on another account. That
			// is the vendor's capacity to lend, not this chain's correctness to get
			// right, and the retry that works around it is pinned in
			// transport_envelope_retry_test.go. This case still fails on a refused
			// signature or a mis-shaped stream; it only reports that no answer came
			// to name.
			t.Logf("the vendor declined to serve qfmodel just now: %s", failure.Message)
			return
		}
		t.Fatalf("the vendor refused the signed call: status=%d message=%s", failure.Status, failure.Message)
	}
	defer func() {
		// reason: the stream is read in full below.
		_ = unwrapped.Close()
	}()

	stream, err := io.ReadAll(unwrapped)
	if err != nil {
		t.Fatalf("reading the unwrapped stream: %v", err)
	}

	// The state is seeded exactly as the engine seeds it: the resolved model id,
	// which for qfmodel is the vendor's own key.
	state := NewStreamState("", "qfmodel", time.Now().Unix(), false)
	frames := 0
	seenModels := map[string]bool{}
	for _, event := range strings.Split(string(stream), "\n\n") {
		payload := sseData([]byte(event))
		if len(payload) == 0 {
			continue
		}
		translated := state.Frames(TargetOpenAI, payload)
		frames += len(translated)
		for _, frame := range translated {
			if name := modelOfFrame(t, frame); name != "" {
				seenModels[name] = true
			}
		}
	}
	if frames == 0 {
		t.Fatalf("the translated stream produced no frames from %d bytes", len(stream))
	}
	if len(seenModels) != 1 || !seenModels["qfmodel"] {
		t.Fatalf("the client would read the answer as %v, want only qfmodel", seenModels)
	}
	t.Logf("%d frames of a real Qwen3.8-Flash answer, all naming qfmodel", frames)
}

// modelOfFrame reads the member a client reads the model from.
func modelOfFrame(t *testing.T, frame []byte) string {
	t.Helper()
	payload := sseData(frame)
	if len(payload) == 0 {
		return ""
	}
	decoded, ok := decodeObject(payload)
	if !ok {
		t.Fatalf("a translated frame is not a json object: %s", payload)
	}
	return stringField(decoded, "model")
}
