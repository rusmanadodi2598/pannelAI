// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_crypto_test.go
// @for       The primitives under a COSY signature, proven by running the vendor's read of them backwards.
// @uses      crypto/aes, crypto/cipher, crypto/rsa, encoding/base64, encoding/json, strings, testing.
// @reason    Two of the three secrets a COSY request carries are randomized, the AES key is fresh per request and PKCS#1 v1.5 padding is random, so no fixed expected string can prove them. What proves them is the decode path the vendor runs: unwrap the key, decrypt the info, and require both to yield what was put in.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestQoderRSAPublicKeyParses pins the vendor's constant: the PEM decodes, the key
// parses as RSA, and the modulus is the size the IDE's key is. A corrupted constant
// would otherwise first show up as a rejected request.
func TestQoderRSAPublicKeyParses(t *testing.T) {
	public, err := parseQoderRSAPublicKey()
	if err != nil {
		t.Fatalf("parsing the vendor key: %v", err)
	}
	if bits := public.N.BitLen(); bits != 1024 {
		t.Fatalf("modulus is %d bits, want the vendor's 1024", bits)
	}
	if public.E != 65537 {
		t.Fatalf("public exponent = %d, want 65537", public.E)
	}
}

// TestCosySecretsRoundTrip walks the vendor's decode path: the wrapped AES key
// unwraps to the sixteen leading characters of the drawn id, and the info blob
// decrypts under that key to the account the request speaks as.
func TestCosySecretsRoundTrip(t *testing.T) {
	signer, private := newCosyTestSigner(t)

	header, err := signer.headers([]byte("x"), cosyTestModelListURL, cosyTestIdentity())
	if err != nil {
		t.Fatalf("headers: %v", err)
	}

	aesKey := cosyTestIDs[0][:16]
	sealed, err := base64.StdEncoding.DecodeString(header.Get("Cosy-Key"))
	if err != nil {
		t.Fatalf("decoding Cosy-Key: %v", err)
	}
	// Unwrapping with the test's own key pair is the only way to prove the padding
	// this scheme requires; the vendor's server runs the same read.
	//
	//lint:ignore SA1019 proving the vendor's padding means reading it the vendor's way
	unwrapped, err := rsa.DecryptPKCS1v15(nil, private, sealed) //nolint:staticcheck // reason: the vendor's padding, read back by the test.
	if err != nil {
		t.Fatalf("unwrapping the cosy key: %v", err)
	}
	if string(unwrapped) != aesKey {
		t.Fatalf("wrapped key = %q, want the aes key %q", unwrapped, aesKey)
	}

	payload := decodeCosyPayload(t, header)
	if payload.Version != qoderPayloadVer || payload.CosyVersion != qoderIDEVersion || payload.IDEVersion != "" {
		t.Fatalf("payload identity = %+v", payload)
	}
	if payload.RequestID == "" || payload.RequestID == cosyTestIDs[0] {
		t.Fatalf("payload requestId = %q, want its own drawn id", payload.RequestID)
	}

	var info cosyInfoPayload
	if err := json.Unmarshal(decryptCosyInfo(t, aesKey, payload.Info), &info); err != nil {
		t.Fatalf("decoding the info: %v", err)
	}
	if info.UID != "019f-account" || info.SecurityToken != "jt-jobtoken" {
		t.Fatalf("info account = %+v, want the id and token the request signs as", info)
	}
	if info.Name != "Dodi" || info.Email != "dodi@example.com" || info.AID != "" {
		t.Fatalf("info identity = %+v", info)
	}
}

// TestEncryptCosyInfoIsBlockSized pins the shape the vendor reads back: the cipher
// text is a whole number of 16-byte blocks, so its own padding removal cannot fail,
// and it is longer than the plaintext it sealed.
func TestEncryptCosyInfoIsBlockSized(t *testing.T) {
	aesKey := cosyTestIDs[0][:16]
	sealed, err := encryptCosyInfo(aesKey, cosyInfoPayload{UID: "u", SecurityToken: "t"})
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(raw)%aes.BlockSize != 0 {
		t.Fatalf("cipher text is %d bytes, want a multiple of %d", len(raw), aes.BlockSize)
	}
	plain, err := json.Marshal(cosyInfoPayload{UID: "u", SecurityToken: "t"})
	if err != nil {
		t.Fatalf("encoding the plain: %v", err)
	}
	if len(raw) <= len(plain) {
		t.Fatalf("cipher text %d bytes against plain %d, want the padding included", len(raw), len(plain))
	}
}

// TestPkcs7PadAlwaysFillsABlock pins the padding rule the vendor's unpad depends on:
// an input that already fills a block still gains a full block of padding, and the
// last byte always counts them.
func TestPkcs7PadAlwaysFillsABlock(t *testing.T) {
	cases := []struct {
		size  int
		want  int
		count byte
	}{
		{size: 0, want: 16, count: 16},
		{size: 1, want: 16, count: 15},
		{size: 15, want: 16, count: 1},
		{size: 16, want: 32, count: 16},
		{size: 17, want: 32, count: 15},
	}
	for _, testCase := range cases {
		padded := pkcs7Pad(make([]byte, testCase.size), 16)
		if len(padded) != testCase.want {
			t.Fatalf("size %d padded to %d, want %d", testCase.size, len(padded), testCase.want)
		}
		if padded[len(padded)-1] != testCase.count {
			t.Fatalf("size %d padded with %d, want the count %d",
				testCase.size, padded[len(padded)-1], testCase.count)
		}
	}
}

// TestNewQoderIDIsUUIDShape pins the id every caller draws: a canonical UUID with the
// version and variant bits set, because the AES key is cut from its leading sixteen
// characters and the vendor's client sends the same shape.
func TestNewQoderIDIsUUIDShape(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id := newQoderID()
		if len(id) != 36 || strings.Count(id, "-") != 4 {
			t.Fatalf("id = %q, want the canonical uuid shape", id)
		}
		if id[14] != '4' {
			t.Fatalf("id = %q, want version 4 at index 14", id)
		}
		if variant := id[19]; variant != '8' && variant != '9' && variant != 'a' && variant != 'b' {
			t.Fatalf("id = %q, want a variant-1 digit at index 19, got %q", id, string(variant))
		}
		seen[id] = true
	}
	if len(seen) != 200 {
		t.Fatalf("200 draws collapsed to %d, so the source is not random", len(seen))
	}
}

// decryptCosyInfo runs the vendor's read of the encrypted user info: AES-128-CBC with
// the key as the IV, then its padding taken back off.
func decryptCosyInfo(t *testing.T, aesKey, infoB64 string) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(infoB64)
	if err != nil {
		t.Fatalf("decoding the info: %v", err)
	}
	block, err := aes.NewCipher([]byte(aesKey))
	if err != nil {
		t.Fatalf("opening the cipher: %v", err)
	}
	if len(raw)%block.BlockSize() != 0 {
		t.Fatalf("the info is not a whole number of cipher blocks: %d bytes", len(raw))
	}
	plain := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, []byte(aesKey)).CryptBlocks(plain, raw)

	count := int(plain[len(plain)-1])
	if count < 1 || count > block.BlockSize() {
		t.Fatalf("the info carries a padding count of %d", count)
	}
	return plain[:len(plain)-count]
}
