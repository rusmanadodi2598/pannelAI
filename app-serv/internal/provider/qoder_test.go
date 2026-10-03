// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_test.go
// @for       The Qoder connector's two decisions: which host serves a credential,
//
//	and what a signed request puts on the wire.
//
// @uses      context, encoding/json, io, net/http, net/http/httptest, strings,
//
//	testing, time, internal/registry.
//
// @reason    These are the two places the gateway stops treating Qoder like an
//
//	OpenAI-compatible vendor. A host chosen wrong is a 403 from a working
//	credential, and a signature computed over bytes that are not the ones
//	sent is a rejection that looks like an auth failure — so both are
//	pinned against a stub that records exactly what arrived.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestQoderApplyAuthExchangesThenSigns pins the credential path end to end inside the
// connector: the Personal Access Token is exchanged, the signature is drawn as the
// job token, and the PAT itself appears in no header of the outbound request.
func TestQoderApplyAuthExchangesThenSigns(t *testing.T) {
	var exchanges int
	connector := newQoderTestConnector(t,
		qoderEntry("qoder", "https://openapi.qoder.sh", qoderChatURLIntl), &exchanges)

	body := []byte(`{"model":"ultimate","messages":[]}`)
	request, err := http.NewRequest(http.MethodPost, qoderChatURLCN, strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	request.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(string(body))), nil }

	if err := connector.ApplyAuth(request, qoderPATCredential()); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	if exchanges != 1 {
		t.Fatalf("the exchange ran %d times, want once", exchanges)
	}
	if strings.Contains(request.Header.Get("Authorization"), "pt-secret") {
		t.Fatal("the personal access token reached the wire")
	}
	if !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer COSY.") {
		t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
	}
	if request.Header.Get("Cosy-User") != "user-7" {
		t.Fatalf("Cosy-User = %q, want the account's user id", request.Header.Get("Cosy-User"))
	}
	if request.Header.Get("Cosy-Machineid") != "machine-fixed" {
		t.Fatalf("Cosy-Machineid = %q, want the id the login minted", request.Header.Get("Cosy-Machineid"))
	}
	if request.Header.Get("Accept-Encoding") != "identity" {
		t.Fatal("a signed call must ask for identity encoding")
	}
	// The hash covers the bytes the transport will send, not a pre-shaped body.
	if got := request.Header.Get("Cosy-Bodyhash"); got != md5Hex(body) {
		t.Fatalf("Cosy-Bodyhash = %q, want the digest of the body that goes out", got)
	}
	if got := request.Header.Get("Cosy-Bodylength"); got != "34" {
		t.Fatalf("Cosy-Bodylength = %q, want %d", got, len(body))
	}
}

// TestQoderApplyAuthSignsADeviceTokenWithoutExchanging pins the other credential: an
// account that already holds a device token is signed as stored, and the exchange is
// never contacted.
func TestQoderApplyAuthSignsADeviceTokenWithoutExchanging(t *testing.T) {
	var exchanges int
	connector := newQoderTestConnector(t,
		qoderEntry("qoder", "https://openapi.qoder.sh", qoderChatURLIntl), &exchanges)

	request, err := http.NewRequest(http.MethodGet, qoderChatURLIntl, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	cred := Credential{AccessToken: "dt-device", Family: FamilyOAuth, ProjectID: "user-7",
		Metadata: map[string]string{MetadataMachineID: "machine-fixed"}}

	if err := connector.ApplyAuth(request, cred); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	if exchanges != 0 {
		t.Fatalf("a device token exchanged %d times, want none", exchanges)
	}
	if !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer COSY.") {
		t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
	}
}

// TestQoderRefusals pins what the connector refuses before it can serve: an entry
// with no oauth block, one with no openapi base to exchange against, one with no chat
// URL to call, an account with no credential, and a request that does not exist.
func TestQoderRefusals(t *testing.T) {
	t.Run("no oauth block", func(t *testing.T) {
		if _, err := NewQoder(registry.Provider{ID: "qoder"}, http.DefaultClient); err == nil {
			t.Fatal("an entry without an oauth block was accepted")
		}
	})
	t.Run("nil egress client", func(t *testing.T) {
		// SSRF §2.1: the connector must not build its own unguarded client. A nil
		// client is a boot-time refusal, not a silent fallback (draft 042 R07).
		if _, err := NewQoder(qoderEntry("qoder", "https://openapi.qoder.sh", qoderChatURLIntl), nil); err == nil {
			t.Fatal("a nil egress client was accepted")
		}
	})
	t.Run("no openapi base", func(t *testing.T) {
		entry := registry.Provider{ID: "qoder", OAuth: &registry.OAuth{}}
		if _, err := NewQoder(entry, http.DefaultClient); err == nil {
			t.Fatal("an entry with no openapi base was accepted")
		}
	})
	t.Run("no chat url", func(t *testing.T) {
		connector, err := NewQoder(qoderEntry("qoder", "https://openapi.example.com", ""), http.DefaultClient)
		if err != nil {
			t.Fatalf("NewQoder() error = %v", err)
		}
		if _, err := connector.Endpoint(Request{}, qoderPATCredential()); err == nil {
			t.Fatal("an entry with no chat url produced an endpoint")
		}
	})
	t.Run("no credential", func(t *testing.T) {
		connector := newQoderTestConnector(t,
			qoderEntry("qoder", "https://openapi.qoder.sh", qoderChatURLIntl), nil)
		if _, err := connector.Endpoint(Request{}, Credential{}); err == nil {
			t.Fatal("an account with no credential produced an endpoint")
		}
		request, err := http.NewRequest(http.MethodPost, qoderChatURLIntl, strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		if err := connector.ApplyAuth(request, Credential{}); err == nil {
			t.Fatal("an account with no credential signed a request")
		}
		if err := connector.ApplyAuth(nil, qoderPATCredential()); err == nil {
			t.Fatal("a missing request was signed")
		}
	})
	t.Run("an exchange that fails", func(t *testing.T) {
		connector, err := NewQoder(qoderEntry("qoder", "https://openapi.invalid", qoderChatURLIntl), http.DefaultClient)
		if err != nil {
			t.Fatalf("NewQoder() error = %v", err)
		}
		request, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
			qoderChatURLIntl, strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		if err := connector.ApplyAuth(request, qoderPATCredential()); err == nil {
			t.Fatal("a failed exchange still signed the request")
		}
	})
}
