// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_cosy_test.go
// @for       The composition of one COSY header set: its parts, their order, and
//
//	the refusals before any of it is built.
//
// @uses      bytes, strconv, strings, testing.
//
// @reason    The signature is an MD5 over five parts in a stated order, and the
//
//	order is the thing a port gets wrong quietly: every part is present,
//	joined wrongly, and the vendor rejects the request. So the tests here
//	rebuild the digest from what the headers publish rather than from what
//	the signer kept in memory, which is how the server reads it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestCosyHeadersCarryTheVendorFingerprint(t *testing.T) {
	signer, _ := newCosyTestSigner(t)
	body := []byte(`{"model":"ultimate"}`)

	header, err := signer.headers(body, cosyTestModelListURL, cosyTestIdentity())
	if err != nil {
		t.Fatalf("headers: %v", err)
	}

	want := map[string]string{
		"Cosy-User":              "019f-account",
		"Cosy-Date":              strconv.FormatInt(cosyTestNow.Unix(), 10),
		"Cosy-Version":           qoderIDEVersion,
		"Cosy-Machineid":         "machine-fixed",
		"Cosy-Machinetoken":      "machine-fixed",
		"Cosy-Machinetype":       qoderMachineType,
		"Cosy-Machineos":         qoderMachineOS,
		"Cosy-Clienttype":        qoderClientType,
		"Cosy-Clientip":          qoderClientIP,
		"Cosy-Sigpath":           "/api/v2/model/list",
		"Cosy-Data-Policy":       qoderDataPolicy,
		"Cosy-Organization-Id":   "",
		"Cosy-Organization-Tags": "",
		"Login-Version":          qoderLoginVersion,
	}
	for name, value := range want {
		if got := header.Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}

	if got := header.Get("Cosy-Bodyhash"); got != md5Hex(body) {
		t.Fatalf("Cosy-Bodyhash = %q, want the md5 of the body that goes out", got)
	}
	if got := header.Get("Cosy-Bodylength"); got != strconv.Itoa(len(body)) {
		t.Fatalf("Cosy-Bodylength = %q, want %d", got, len(body))
	}
	credential := header.Get("Authorization")
	if !strings.HasPrefix(credential, "Bearer COSY.") || strings.Count(credential, ".") != 2 {
		t.Fatalf("Authorization = %q, want Bearer COSY.<payload>.<signature>", credential)
	}
}

// TestCosySignatureIsRecomputableFromItsHeaders recomputes the digest the way the
// vendor does, payload, wrapped key, timestamp, body, signing path, in that order,
// from values the request publishes, and requires the signature sent to match. The
// order is spelled out again here rather than called, so a reordered composition is a
// failing test instead of a passing mirror.
func TestCosySignatureIsRecomputableFromItsHeaders(t *testing.T) {
	signer, _ := newCosyTestSigner(t)
	body := []byte("the body that goes out")

	header, err := signer.headers(body, cosyTestChatURL, cosyTestIdentity())
	if err != nil {
		t.Fatalf("headers: %v", err)
	}

	payloadB64, signature := cosyAuthParts(t, header)
	var input bytes.Buffer
	input.WriteString(payloadB64)
	input.WriteByte('\n')
	input.WriteString(header.Get("Cosy-Key"))
	input.WriteByte('\n')
	input.WriteString(header.Get("Cosy-Date"))
	input.WriteByte('\n')
	input.Write(body)
	input.WriteByte('\n')
	input.WriteString(header.Get("Cosy-Sigpath"))

	if got := md5Hex(input.Bytes()); got != signature {
		t.Fatalf("signature = %q, recomputed %q", signature, got)
	}
	if header.Get("Cosy-Sigpath") != "/api/v2/service/pro/sse/agent_chat_generation" {
		t.Fatalf("the chat path signed as %q", header.Get("Cosy-Sigpath"))
	}
}

// TestCosyEmptyBodyIsASignableGET pins a GET's shape: the hash of no bytes and a
// length of zero, which is what the model list sends.
func TestCosyEmptyBodyIsASignableGET(t *testing.T) {
	signer, _ := newCosyTestSigner(t)

	header, err := signer.headers(nil, cosyTestModelListURL, cosyTestIdentity())
	if err != nil {
		t.Fatalf("headers: %v", err)
	}
	if got, want := header.Get("Cosy-Bodyhash"), md5Hex(nil); got != want {
		t.Fatalf("Cosy-Bodyhash = %q, want the md5 of an empty body %q", got, want)
	}
	if header.Get("Cosy-Bodylength") != "0" {
		t.Fatalf("Cosy-Bodylength = %q, want 0", header.Get("Cosy-Bodylength"))
	}
}

// TestCosyMachineIDSurvivesAndFallsBack pins the two machine-id answers: the id the
// login minted is replayed on every request, and an account that never had one draws
// a fresh id rather than sending an empty header.
func TestCosyMachineIDSurvivesAndFallsBack(t *testing.T) {
	signer, _ := newCosyTestSigner(t)

	header, err := signer.headers(nil, cosyTestModelListURL, cosyTestIdentity())
	if err != nil {
		t.Fatalf("headers: %v", err)
	}
	if header.Get("Cosy-Machineid") != "machine-fixed" {
		t.Fatalf("the persisted machine id was not replayed: %q", header.Get("Cosy-Machineid"))
	}

	blank := cosyTestIdentity()
	blank.MachineID = ""
	fallback, err := signer.headers(nil, cosyTestModelListURL, blank)
	if err != nil {
		t.Fatalf("headers without a machine id: %v", err)
	}
	generated := fallback.Get("Cosy-Machineid")
	if len(strings.Split(generated, "-")) != 5 {
		t.Fatalf("generated machine id = %q, want a uuid shape", generated)
	}
	if fallback.Get("Cosy-Machinetoken") != generated {
		t.Fatalf("Cosy-Machinetoken must carry the same id: %q / %q",
			fallback.Get("Cosy-Machinetoken"), generated)
	}
}

// TestCosyRefusesAnAccountThatCannotSign pins the refusals before any crypto runs: no
// user id and no token. A request signed as an unknown account is a rejected request,
// so the refusal belongs to the signer rather than to the call.
func TestCosyRefusesAnAccountThatCannotSign(t *testing.T) {
	cases := []struct {
		name string
		id   cosyIdentity
	}{
		{"no user id", cosyIdentity{AuthToken: "jt-x"}},
		{"a blank user id", cosyIdentity{UserID: "   ", AuthToken: "jt-x"}},
		{"no auth token", cosyIdentity{UserID: "u"}},
		{"an unwired signer", cosyIdentity{UserID: "u", AuthToken: "t"}},
	}
	signer, _ := newCosyTestSigner(t)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.name == "an unwired signer" {
				if _, err := (cosySigner{}).headers(nil, cosyTestModelListURL, cosyTestIdentity()); err == nil {
					t.Fatal("an unwired signer produced headers")
				}
				return
			}
			if _, err := signer.headers(nil, cosyTestModelListURL, testCase.id); err == nil {
				t.Fatal("headers accepted an account that cannot sign")
			}
		})
	}
}

// TestQoderCosySignerIsWired runs the process's own signer rather than the fixed one:
// the real clock, the real id source, and the vendor's key. It is the only place that
// wiring is exercised before the connector arrives, and it proves the pair of things
// a mis-wired signer would get wrong quietly, an unwired signer refuses every
// request, and a corrupted key constant fails to parse.
func TestQoderCosySignerIsWired(t *testing.T) {
	header, err := qoderCosy.headers([]byte("a body"), cosyTestChatURL, cosyTestIdentity())
	if err != nil {
		t.Fatalf("the wired signer refused a well-formed account: %v", err)
	}
	if header.Get("Cosy-Date") == "" || header.Get("X-Request-Id") == "" {
		t.Fatalf("the wired signer drew no clock or no request id: %q / %q",
			header.Get("Cosy-Date"), header.Get("X-Request-Id"))
	}
	if header.Get("Cosy-Machineid") != "machine-fixed" {
		t.Fatalf("the wired signer did not replay the account's machine id: %q",
			header.Get("Cosy-Machineid"))
	}
	payload := decodeCosyPayload(t, header)
	if payload.RequestID == header.Get("X-Request-Id") {
		t.Fatal("the payload request id and the header request id must be separate draws")
	}
	if payload.Info == "" || header.Get("Cosy-Key") == "" {
		t.Fatal("the wired signer produced an empty payload or an empty wrapped key")
	}
}

// TestQoderSigPath pins how a URL becomes the signed path: the "/algo" prefix the CDN
// adds is stripped, a path without it is kept, and a URL with no path signs nothing
// rather than a guess.
func TestQoderSigPath(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{cosyTestModelListURL, "/api/v2/model/list"},
		{cosyTestChatURL + "?Encode=1", "/api/v2/service/pro/sse/agent_chat_generation"},
		{"https://api3.qoder.sh/api/v2/model/list", "/api/v2/model/list"},
		{"https://api3.qoder.sh", ""},
		{":not a url", ""},
	}
	for _, testCase := range cases {
		if got := qoderSigPath(testCase.url); got != testCase.want {
			t.Fatalf("sigPath(%q) = %q, want %q", testCase.url, got, testCase.want)
		}
	}
}
