//go:build integration

// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_identity_live_test.go
// @for       The live proof that a Personal Access Token with nothing else stored can
//
//	be signed, served, and answered.
//
// @uses      bytes, io, net/http, strings, testing, time.
// @reason    The panel's connection row for a pasted PAT carries a token and no
//
//	identity: no user id, no email, no machine id, because nothing in the
//	key-add path asks for them. The connector used to sign whatever it was
//	handed, so every request on such a connection died in shaping and the
//	provider looked broken. This is the state that bug shipped in, and it is
//	the one only the vendor can confirm — a stub cannot prove the signed
//	identity is the one the account answers.
//
//	  PANNELAI_QODER_PAT='pt-…' \
//	    go test -tags=integration ./internal/provider/ -run QoderLivePAT
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-28
package provider

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestQoderLivePATWithoutStoredIdentity signs, sends, and reads an answer with a
// credential that stores only the token, through the entry the binary embeds rather
// than a hand-built one.
func TestQoderLivePATWithoutStoredIdentity(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	entry, ok := index.Provider("qoder")
	if !ok {
		t.Fatal("the embedded registry carries no qoder entry")
	}
	connector, err := NewQoder(entry, nil)
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}

	credential := Credential{
		EndpointID: "ep_pat_only", APIKey: qoderLivePAT(t), Family: FamilyStaticKey,
	}

	request := &Request{
		Model:      registry.Model{ID: "qfmodel"},
		Wire:       "openai",
		Stream:     true,
		Body:       []byte(`{"model":"qoder/qfmodel","messages":[{"role":"user","content":"Reply with exactly: PONG"}],"max_tokens":64}`),
		Credential: credential,
	}
	request.Provider = entry
	if err := connector.TransformRequest(request); err != nil {
		t.Fatalf("TransformRequest() with a token-only credential: %v", err)
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
	// The header is where the resolved identity lands, and an empty one is what the
	// vendor refuses: a signature without it would look like this test passing while
	// the gateway still failed.
	if user := outbound.Header.Get("Cosy-User"); user == "" {
		t.Fatal("Cosy-User is empty, so the identity was never resolved")
	}

	response, err := (&http.Client{Timeout: 60 * time.Second}).Do(outbound)
	if err != nil {
		t.Fatalf("the chat call failed: %v", err)
	}
	defer func() {
		// reason: the answer is read in full below.
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("chat = HTTP %d, want the vendor to accept a token-only signature", response.StatusCode)
	}

	unwrapped, failure := connector.OpenStream(response.Body)
	if failure != nil {
		// The vendor's own serving layer can refuse ("All backends failed") with no
		// connection to how the request was signed. What this case is about is that a
		// token-only credential reaches that layer at all rather than dying in
		// shaping, so a refusal above it is reported and a refusal of the signature
		// still fails.
		if qoderVendorUnserved(failure) {
			t.Logf("the signed call reached serving and the vendor declined to serve it: %s",
				truncateForLive(failure.Message, 200))
			return
		}
		t.Fatalf("the vendor refused a token-only call: status=%d quota=%v message=%s",
			failure.Status, failure.Quota, truncateForLive(failure.Message, 300))
	}
	defer func() {
		// reason: the stream is read in full below.
		_ = unwrapped.Close()
	}()
	answer, err := io.ReadAll(io.LimitReader(unwrapped, 1<<20))
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	if !strings.Contains(string(answer), "chat.completion.chunk") {
		t.Fatalf("no chunk arrived on a token-only credential: %s", truncateForLive(string(answer), 400))
	}
	t.Logf("a token-only credential was served: Cosy-User=%s, %d bytes of stream",
		outbound.Header.Get("Cosy-User"), len(answer))
}

// qoderVendorUnserved names the one refusal this gateway cannot cause or fix: the
// vendor answered its own serving layer with no backends, or with a rate limit that
// carries no credential complaint in it. An auth refusal looks different — the
// vendor rejects the signature, not the capacity.
func qoderVendorUnserved(failure *StreamFailure) bool {
	if failure.Status != http.StatusTooManyRequests && failure.Status != http.StatusBadGateway &&
		failure.Status != http.StatusServiceUnavailable {
		return false
	}
	return strings.Contains(failure.Message, "backends failed") ||
		strings.Contains(failure.Message, "provider_error")
}
