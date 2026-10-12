// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/vertex_jwt_test.go
// @for       The service-account key parse and the RS256 assertion the token endpoint reads.
// @uses      crypto, crypto/rand, crypto/rsa, crypto/sha256, crypto/x509, encoding/base64, encoding/json, encoding/pem, strings, testing, time.
// @reason    The assertion is the one artifact this gateway signs on behalf of a customer's key, so its shape is pinned by verifying the signature against the key's public half and decoding the claim set back, rather than by comparing against a second copy of the same construction. Every parse refusal is pinned too, because each one keeps a bad key from reaching Google as an opaque 401.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package provider

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

// vertexTestKey generates the RSA key one test's service account signs with,
// and the two PEM shapes a Google key is issued in.
func vertexTestKey(t *testing.T) (*rsa.PrivateKey, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating an RSA key: %v", err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling PKCS#8: %v", err)
	}
	pkcs1 := x509.MarshalPKCS1PrivateKey(key)
	return key,
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})),
		string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pkcs1}))
}

// vertexTestJSON builds the service-account JSON an operator would paste.
func vertexTestJSON(t *testing.T, privateKey string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   "pannelai-prod",
		"client_email": "veo@pannelai-prod.iam.gserviceaccount.com",
		"private_key":  privateKey,
	})
	if err != nil {
		t.Fatalf("building the service-account JSON: %v", err)
	}
	return string(raw)
}

// TestParseVertexServiceAccount_ReadsBothKeyShapes pins that the console's
// PKCS#8 keys and the older PKCS#1 keys both parse.
func TestParseVertexServiceAccount_ReadsBothKeyShapes(t *testing.T) {
	_, pkcs8, pkcs1 := vertexTestKey(t)
	for name, pemText := range map[string]string{"pkcs8": pkcs8, "pkcs1": pkcs1} {
		t.Run(name, func(t *testing.T) {
			account, key, err := parseVertexServiceAccount(vertexTestJSON(t, pemText))
			if err != nil {
				t.Fatalf("parseVertexServiceAccount() error = %v", err)
			}
			if account.ClientEmail != "veo@pannelai-prod.iam.gserviceaccount.com" {
				t.Fatalf("client_email = %q", account.ClientEmail)
			}
			if key == nil || key.N == nil {
				t.Fatal("the private key was not parsed")
			}
		})
	}
}

// TestParseVertexServiceAccount_Refusals pins each wrong shape an operator can
// paste, so none of them reaches the token endpoint as an opaque refusal.
func TestParseVertexServiceAccount_Refusals(t *testing.T) {
	_, validPEM, _ := vertexTestKey(t)
	valid := vertexTestJSON(t, validPEM)

	cases := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: "  "},
		{name: "a raw API key", raw: "AIzaSyD-example-key-value"},
		{name: "invalid JSON", raw: `{"type": "service_account",`},
		{name: "another key type", raw: strings.Replace(valid, `"service_account"`, `"authorized_user"`, 1)},
		{name: "no private key", raw: strings.Replace(valid, `"private_key":"`, `"private_key_unused":"`, 1)},
		{name: "a private key that is not PEM", raw: vertexTestJSON(t, "not-a-pem-key")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseVertexServiceAccount(tc.raw); err == nil {
				t.Fatal("parseVertexServiceAccount() = nil error, want a refusal")
			}
		})
	}
}

// TestVertexAssertion_SignsTheClaimSet verifies the assertion the way Google
// does: the signature against the key's public half, and the claim set decoded
// back out.
func TestVertexAssertion_SignsTheClaimSet(t *testing.T) {
	key, validPEM, _ := vertexTestKey(t)
	account, parsedKey, err := parseVertexServiceAccount(vertexTestJSON(t, validPEM))
	if err != nil {
		t.Fatalf("parsing the service account: %v", err)
	}
	issuedAt := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)

	assertion, err := vertexAssertion(account, parsedKey, issuedAt)
	if err != nil {
		t.Fatalf("vertexAssertion() error = %v", err)
	}
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		t.Fatalf("the assertion has %d parts, want 3", len(parts))
	}

	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decoding the header: %v", err)
	}
	var header vertexJWTHeader
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		t.Fatalf("decoding the header JSON: %v", err)
	}
	if header.Alg != "RS256" || header.Typ != "JWT" {
		t.Fatalf("header = %+v, want RS256/JWT", header)
	}

	claimsRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decoding the claims: %v", err)
	}
	var claims vertexClaims
	if err := json.Unmarshal(claimsRaw, &claims); err != nil {
		t.Fatalf("decoding the claims JSON: %v", err)
	}
	if claims.Scope != vertexTokenScope {
		t.Fatalf("scope = %q, want %q", claims.Scope, vertexTokenScope)
	}
	if claims.Iss != "veo@pannelai-prod.iam.gserviceaccount.com" {
		t.Fatalf("iss = %q", claims.Iss)
	}
	if claims.Aud != vertexTokenURL {
		t.Fatalf("aud = %q, want the token endpoint", claims.Aud)
	}
	if claims.Iat != issuedAt.Unix() || claims.Exp != issuedAt.Unix()+3600 {
		t.Fatalf("iat/exp = %d/%d, want one hour from the issue instant", claims.Iat, claims.Exp)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decoding the signature: %v", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("the signature does not verify against the key's public half: %v", err)
	}
}
