// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_cosy.go
// @for       The COSY header set one Qoder request carries, and the signature that
//
//	closes it.
//
// @uses      crypto/md5, crypto/rsa, encoding/base64, encoding/hex,
//
//	encoding/json, fmt, net/http, net/url, strconv, strings, time.
//
// @reason    Qoder does not read a bearer token: it reads a fingerprint of the
//
//	client, the account, the body and the path. The payload is the encrypted
//	user info (see qoder_crypto.go), and the signature is an MD5 over five
//	specific parts in a specific order, which is the only part of this scheme
//	the gateway composes rather than encrypts. Ported verbatim, because none
//	of it is inferable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"crypto/md5"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// cosyIdentity is the account a signed request speaks as. A device login's user id
// and the machine id minted with its round are what the vendor ties the request to;
// name and email ride inside the encrypted payload only.
type cosyIdentity struct {
	UserID    string
	AuthToken string
	Name      string
	Email     string
	MachineID string
}

// cosySigner is the scheme with its randomness and its clock named, so a test can
// pin a whole signature instead of only its shape. publicKey is the vendor's key in
// production and a generated one under test, because the padding it uses is
// randomized and its output can only be proven by unwrapping it.
type cosySigner struct {
	now       func() time.Time
	newID     func() string
	publicKey *rsa.PublicKey
}

// cosyInfoPayload is the encrypted user info. The field order is the reference's:
// it is marshalled to bytes and encrypted, so the order is part of what the vendor
// reads back.
type cosyInfoPayload struct {
	UID           string `json:"uid"`
	SecurityToken string `json:"security_oauth_token"`
	Name          string `json:"name"`
	AID           string `json:"aid"`
	Email         string `json:"email"`
}

// cosyPayload is the signed envelope: the wrapped info, the request id the vendor
// echoes in its own accounting, and the client versions it gates on.
type cosyPayload struct {
	Version     string `json:"version"`
	RequestID   string `json:"requestId"`
	Info        string `json:"info"`
	CosyVersion string `json:"cosyVersion"`
	IDEVersion  string `json:"ideVersion"`
}

// qoderCosy is the signer the connectors use: the clock and the ids are the
// process's own.
var qoderCosy = cosySigner{now: time.Now, newID: newQoderID}

// headers signs one request. The body is the exact byte sequence that will be sent —
// the obfuscated one when the body was encoded — because the vendor hashes what
// arrives, and a nil slice is what a GET passes.
func (s cosySigner) headers(body []byte, requestURL string, id cosyIdentity) (http.Header, error) {
	if strings.TrimSpace(id.UserID) == "" {
		return nil, fmt.Errorf("cosy: the account has no user id")
	}
	if strings.TrimSpace(id.AuthToken) == "" {
		return nil, fmt.Errorf("cosy: the account has no auth token")
	}
	if s.now == nil || s.newID == nil {
		return nil, fmt.Errorf("cosy: the signer is unwired")
	}
	public, err := s.key()
	if err != nil {
		return nil, err
	}

	aesKey := s.aesKey()
	info, err := encryptCosyInfo(aesKey, cosyInfoPayload{
		UID: id.UserID, SecurityToken: id.AuthToken, Name: id.Name, AID: "", Email: id.Email,
	})
	if err != nil {
		return nil, err
	}
	cosyKey, err := wrapAESKey(public, aesKey)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(cosyPayload{
		Version: qoderPayloadVer, RequestID: s.newID(), Info: info,
		CosyVersion: qoderIDEVersion, IDEVersion: "",
	})
	if err != nil {
		return nil, fmt.Errorf("encoding the cosy payload: %w", err)
	}
	payloadB64 := base64.StdEncoding.EncodeToString(payload)

	timestamp := strconv.FormatInt(s.now().Unix(), 10)
	sigPath := qoderSigPath(requestURL)
	sig := qoderSignature(payloadB64, cosyKey, timestamp, body, sigPath)
	machineID := s.machineID(id)

	header := http.Header{}
	header.Set("Authorization", "Bearer COSY."+payloadB64+"."+sig)
	header.Set("Cosy-Key", cosyKey)
	header.Set("Cosy-User", id.UserID)
	header.Set("Cosy-Date", timestamp)
	header.Set("Cosy-Version", qoderIDEVersion)
	header.Set("Cosy-Machineid", machineID)
	header.Set("Cosy-Machinetoken", machineID)
	header.Set("Cosy-Machinetype", qoderMachineType)
	header.Set("Cosy-Machineos", qoderMachineOS)
	header.Set("Cosy-Clienttype", qoderClientType)
	header.Set("Cosy-Clientip", qoderClientIP)
	header.Set("Cosy-Bodyhash", md5Hex(body))
	header.Set("Cosy-Bodylength", strconv.Itoa(len(body)))
	header.Set("Cosy-Sigpath", sigPath)
	header.Set("Cosy-Data-Policy", qoderDataPolicy)
	header.Set("Cosy-Organization-Id", "")
	header.Set("Cosy-Organization-Tags", "")
	header.Set("Login-Version", qoderLoginVersion)
	header.Set("X-Request-Id", s.newID())
	return header, nil
}

// key resolves the vendor's public key once and hands back whatever the signer was
// built with otherwise.
func (s cosySigner) key() (*rsa.PublicKey, error) {
	if s.publicKey != nil {
		return s.publicKey, nil
	}
	return parseQoderRSAPublicKey()
}

// aesKey is the first sixteen characters of a fresh UUID's canonical string, hyphens
// included: the length AES-128 wants and the shape the vendor's client cuts. The IV
// is the same bytes, so a key fresh to every request is what makes the reused IV
// acceptable — the security of this scheme is the vendor's problem to hold, not
// ours to improve (draft 036 slice D).
func (s cosySigner) aesKey() string {
	id := s.newID()
	if len(id) < 16 {
		return id
	}
	return id[:16]
}

// machineID is the id the login minted, replayed on every request from that account
// because the vendor fingerprints it. An account that never had one draws a fresh id
// per request, which is what the reference does for a Personal Access Token.
func (s cosySigner) machineID(id cosyIdentity) string {
	if machine := strings.TrimSpace(id.MachineID); machine != "" {
		return machine
	}
	return s.newID()
}

// qoderSignature is the MD5 the vendor recomputes: payload, wrapped key, timestamp,
// the body bytes, and the signing path, joined by newlines in exactly that order.
func qoderSignature(payloadB64, cosyKey, timestamp string, body []byte, sigPath string) string {
	input := make([]byte, 0, len(payloadB64)+len(cosyKey)+len(timestamp)+len(body)+len(sigPath)+4)
	input = append(input, payloadB64...)
	input = append(input, '\n')
	input = append(input, cosyKey...)
	input = append(input, '\n')
	input = append(input, timestamp...)
	input = append(input, '\n')
	input = append(input, body...)
	input = append(input, '\n')
	input = append(input, sigPath...)
	return md5Hex(input)
}

// qoderSigPath is the request path with the "/algo" prefix stripped, which is how
// the client signs a URL the CDN rewrote. An unparsable URL or a bare host signs an
// empty path rather than a guess, because a wrong path fails the signature outright
// and an empty one is at least a shape the vendor recognises.
func qoderSigPath(requestURL string) string {
	parsed, err := url.Parse(requestURL)
	if err != nil || parsed.Path == "" {
		return ""
	}
	return strings.TrimPrefix(parsed.Path, qoderSigPathPrefix)
}

// md5Hex is the digest the scheme uses. It is named rather than inlined because the
// body hash header and the signature are two different MD5s over two different
// inputs, and a reader has to be able to tell them apart.
func md5Hex(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}
