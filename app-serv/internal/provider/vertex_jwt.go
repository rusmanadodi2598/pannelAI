// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/vertex_jwt.go
// @for       Turning a Google service-account JSON into the signed RS256 assertion the token endpoint exchanges for an access token.
// @uses      crypto/rand, crypto/rsa, crypto/sha256, crypto/x509, encoding/base64, encoding/json, encoding/pem, fmt, strings, time.
// @reason    A service-account credential authenticates by signing, not by presenting itself: the private key never crosses the wire, and what does is a one-hour assertion scoped to the cloud-platform API. Keeping the parse and the signing in one file means the key material has exactly one reader and the claim set has one definition, which is what the reference's own tokenRefresh.js does for the same reason.
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
	"fmt"
	"strings"
	"time"
)

// The assertion's own constants. The token endpoint and the scope are Google's
// fixed names, not configuration: a different audience would be a different
// grant, and the cloud-platform scope is what Vertex AI reads.
const (
	vertexTokenURL      = "https://oauth2.googleapis.com/token"
	vertexTokenScope    = "https://www.googleapis.com/auth/cloud-platform"
	vertexAssertionTTL  = time.Hour
	vertexAssertionAlg  = "RS256"
	vertexAssertionKind = "JWT"
)

// vertexServiceAccount is the subset of a service-account JSON the assertion
// needs. The project id is deliberately not read here: the endpoint's own
// column is the address its calls use, so a key pasted under the wrong project
// is a fact the operator set rather than one this file silently corrects.
type vertexServiceAccount struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
}

// vertexJWTHeader and vertexClaims are the two JSON members of the assertion,
// typed rather than maps so a misspelled claim cannot ship.
type vertexJWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type vertexClaims struct {
	Scope string `json:"scope"`
	Iss   string `json:"iss"`
	Aud   string `json:"aud"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

// parseVertexServiceAccount reads the key material out of a pasted JSON key.
//
// Each refusal names the fact that failed rather than the vendor's own error,
// because the two mistakes an operator makes here are loud in different ways: a
// raw API key pasted by mistake is not JSON at all, and a JSON of the wrong
// type is a key for something else. Neither reaches the token endpoint, so
// neither can be reported as "Google said 401".
func parseVertexServiceAccount(raw string) (vertexServiceAccount, *rsa.PrivateKey, error) {
	var account vertexServiceAccount
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return account, nil, fmt.Errorf("provider: a service-account credential needs the JSON key an operator pasted")
	}
	if !strings.HasPrefix(trimmed, "{") {
		return account, nil, fmt.Errorf("provider: the service-account credential is not a JSON key")
	}
	if err := json.Unmarshal([]byte(trimmed), &account); err != nil {
		return account, nil, fmt.Errorf("provider: the service-account JSON could not be decoded")
	}
	if account.Type != "service_account" {
		return account, nil, fmt.Errorf("provider: the JSON key is of type %q, not a service_account", account.Type)
	}
	if strings.TrimSpace(account.ClientEmail) == "" || strings.TrimSpace(account.PrivateKey) == "" {
		return account, nil, fmt.Errorf("provider: the service-account key is missing client_email or private_key")
	}
	key, err := parseVertexPrivateKey(account.PrivateKey)
	if err != nil {
		return account, nil, err
	}
	return account, key, nil
}

// parseVertexPrivateKey accepts the two PEM shapes a Google key is issued in:
// PKCS#8, which the console writes today, and PKCS#1, which older keys carry.
// A key in neither shape is refused here rather than at the signing call, where
// the failure would read as a cryptographic error instead of a wrong key.
func parseVertexPrivateKey(pemText string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, fmt.Errorf("provider: the service-account private key is not PEM")
	}
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("provider: the service-account private key is not RSA")
		}
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("provider: the service-account private key could not be read")
}

// vertexAssertion signs the JWT the token endpoint reads: the account's own
// address as the issuer, the fixed token endpoint as the audience, and one hour
// of validity, which is the longest lifetime the grant accepts.
func vertexAssertion(account vertexServiceAccount, key *rsa.PrivateKey, now time.Time) (string, error) {
	header, err := json.Marshal(vertexJWTHeader{Alg: vertexAssertionAlg, Typ: vertexAssertionKind})
	if err != nil {
		return "", fmt.Errorf("provider: the service-account assertion header could not be encoded")
	}
	issued := now.UTC().Unix()
	claims, err := json.Marshal(vertexClaims{
		Scope: vertexTokenScope,
		Iss:   account.ClientEmail,
		Aud:   vertexTokenURL,
		Iat:   issued,
		Exp:   issued + int64(vertexAssertionTTL.Seconds()),
	})
	if err != nil {
		return "", fmt.Errorf("provider: the service-account assertion claims could not be encoded")
	}

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("provider: the service-account assertion could not be signed")
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
