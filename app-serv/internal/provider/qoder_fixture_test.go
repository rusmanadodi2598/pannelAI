// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_fixture_test.go
// @for       The fixed-clock signer, the account it signs as, and the readers the Qoder signature tests share.
// @uses      crypto/rand, crypto/rsa, encoding/base64, encoding/json, net/http, strings, testing, time.
// @reason    A signature over a random key and a random clock cannot be asserted against a literal. These fixtures make both deterministic and read the credential header back the way a server would, which is what lets the two test files beside them check composition instead of guessing it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// cosyTestNow is the instant a fixed signer stamps, so a signature can be
// recomputed by a test.
var cosyTestNow = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

// cosyTestIDs are the ids a fixed signer draws, in order: the AES key's source, the
// payload request id, the machine id when the account carries none, and the
// X-Request-Id.
var cosyTestIDs = []string{
	"11111111-1111-4111-8111-111111111111",
	"22222222-2222-4222-8222-222222222222",
	"33333333-3333-4333-8333-333333333333",
	"44444444-4444-4444-8444-444444444444",
}

// newCosyTestSigner returns a signer with a fixed clock and fixed ids, pointed at a
// generated key pair so a wrapped AES key can be unwrapped back.
func newCosyTestSigner(t *testing.T) (cosySigner, *rsa.PrivateKey) {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating the test key: %v", err)
	}
	cursor := 0
	signer := cosySigner{
		now:       func() time.Time { return cosyTestNow },
		newID:     func() string { id := cosyTestIDs[cursor%len(cosyTestIDs)]; cursor++; return id },
		publicKey: &private.PublicKey,
	}
	return signer, private
}

func cosyTestIdentity() cosyIdentity {
	return cosyIdentity{
		UserID: "019f-account", AuthToken: "jt-jobtoken", Name: "Dodi",
		Email: "dodi@example.com", MachineID: "machine-fixed",
	}
}

const (
	cosyTestModelListURL = "https://api2.qoder.sh/algo/api/v2/model/list"
	cosyTestChatURL      = "https://api3.qoder.sh/algo/api/v2/service/pro/sse/agent_chat_generation"
)

// TestCosyHeadersCarryTheVendorFingerprint pins the static part of the set: the
// casing the vendor reads, the machine id in two headers, the stripped path, and the
// hash and length of the bytes that will actually go out.
// decodeCosyPayload reads the JSON envelope out of the credential header.
func decodeCosyPayload(t *testing.T, header http.Header) cosyPayload {
	t.Helper()
	payloadB64, _ := cosyAuthParts(t, header)
	raw, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		t.Fatalf("decoding the payload: %v", err)
	}
	var payload cosyPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decoding the payload json: %v", err)
	}
	return payload
}

// cosyAuthParts splits the credential header into the payload it carries and the
// signature that closes it: two parts once the prefix is gone, and a base64 payload
// that never contains a dot.
func cosyAuthParts(t *testing.T, header http.Header) (string, string) {
	t.Helper()
	credential := header.Get("Authorization")
	parts := strings.Split(strings.TrimPrefix(credential, "Bearer COSY."), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Fatalf("Authorization = %q, want Bearer COSY.<payload>.<signature>", credential)
	}
	return parts[0], parts[1]
}
