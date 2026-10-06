// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_crypto.go
// @for       The cryptographic primitives of one COSY signature: the AES payload, the RSA-wrapped request key, and the ids the scheme draws.
// @uses      crypto/aes, crypto/cipher, crypto/rand, crypto/rsa, crypto/x509, encoding/base64, encoding/json, encoding/pem, fmt.
// @reason    These are the parts a reviewer cannot check by reading the header list: a CBC whose IV is the key, a padding scheme applied by hand because the cipher's own is switched off, and a wrapped key whose ciphertext the vendor alone can unwrap. Split from the composition of the headers so each can be read against the reference on its own terms.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-09-27
package provider

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
)

// encryptCosyInfo seals the user info under the request's AES key: CBC with the key
// bytes as the IV, PKCS#7 padded by hand because the cipher's own padding is left
// off, and standard base64 as the vendor expects.
func encryptCosyInfo(aesKey string, info cosyInfoPayload) (string, error) {
	plain, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("encoding the cosy info: %w", err)
	}
	block, err := aes.NewCipher([]byte(aesKey))
	if err != nil {
		return "", fmt.Errorf("opening the cosy cipher: %w", err)
	}
	padded := pkcs7Pad(plain, block.BlockSize())
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(aesKey)).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

// wrapAESKey RSA-encrypts the request's AES key. The vendor's server is the only
// reader, and the padding its client uses is PKCS#1 v1.5: OAEP, which is the shape
// Go now recommends, would arrive as ciphertext it cannot open.
//
// reason: the vendor's COSY scheme, not this gateway's choice of cipher. OAEP,
// which Go now recommends, would arrive as ciphertext their server cannot
// open, so the deprecated call is the only correct one here.
func wrapAESKey(public *rsa.PublicKey, aesKey string) (string, error) {
	//lint:ignore SA1019 the vendor's server unwraps PKCS#1 v1.5 only
	sealed, err := rsa.EncryptPKCS1v15(rand.Reader, public, []byte(aesKey)) //nolint:staticcheck // reason: the vendor's padding, not our preference.
	if err != nil {
		return "", fmt.Errorf("wrapping the cosy aes key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// pkcs7Pad pads to blockSize with the padding byte equal to its count, the same
// scheme the cipher would apply if its padding were left on.
func pkcs7Pad(data []byte, blockSize int) []byte {
	count := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+count)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(count)
	}
	return padded
}

// parseQoderRSAPublicKey reads the vendor's COSY encryption key. It is a fixed
// constant of the scheme, so a PEM that will not parse is a corrupted source file
// rather than a runtime condition, and it is reported as one.
func parseQoderRSAPublicKey() (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(qoderRSAPublicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("decoding the cosy public key pem")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing the cosy public key: %w", err)
	}
	public, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("the cosy public key is not an RSA key")
	}
	return public, nil
}

// newQoderID formats sixteen CSPRNG bytes as the canonical UUID v4 string. Every id
// the scheme draws is this shape, the request ids, the machine id, and the sixteen
// leading characters of that string, which are the AES key, so the version and
// variant bits are part of what the vendor reads, not decoration.
func newQoderID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// reason: crypto/rand does not fail on the platforms this runs on, and a
		// signer that could not draw an id has nothing coherent to send. The
		// panic is recovered at every goroutine boundary (AGENTS.md §1.6), so an
		// impossible failure costs one request rather than the process.
		panic("provider: cannot draw a request id: " + err.Error())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}
