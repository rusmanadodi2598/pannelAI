// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin_credential_redact_test.go
// @for       Every formatting surface that renders a credential, checked for the plaintext.
// @uses      bytes, fmt, log/slog, strings, testing.
// @reason    The credential is the one value in this seam whose disclosure is an incident, and the ways to render a struct are more numerous than the ways to read it: a verb, a nested field, a log attribute somebody adds at a call site that already holds the value.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-08
package provider

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const redactTestSecret = "pt-SUPERSECRET-material"

// redactTestCredential carries a plaintext in every field that holds one, so a
// surface that leaks shows it.
func redactTestCredential() Credential {
	return Credential{
		endpointID: "ep_1", keyID: "key_1", family: FamilyStaticKey,
		account: "dev@example.com", projectID: "user-7",
		apiKey: redactTestSecret, accessToken: redactTestSecret,
	}
}

// credentialHolder mirrors how the data plane carries a Credential: as an exported
// field, which is what lets fmt reach the field's own String and GoString. An
// unexported field is a worse case that no production struct has, and fmt cannot
// invoke a method on a read-only reflect.Value at all.
type credentialHolder struct {
	Credential Credential
	attempts   int
}

func TestCredentialStaysRedactedAcrossFormattingSurfaces(t *testing.T) {
	cred := redactTestCredential()
	holder := credentialHolder{Credential: cred, attempts: 2}

	cases := []struct {
		name string
		out  string
	}{
		// %v, %+v and %s all route through String(), so one of them is the case;
		// %#v is the verb that does not, and GoString exists for it.
		{name: "%v on the credential", out: fmt.Sprintf("%v", cred)},
		{name: "%+v on the credential", out: fmt.Sprintf("%+v", cred)},
		{name: "%#v on the credential", out: fmt.Sprintf("%#v", cred)},
		{name: "%v on a struct holding it", out: fmt.Sprintf("%v", holder)},
		{name: "%+v on a struct holding it", out: fmt.Sprintf("%+v", holder)},
		{name: "%#v on a struct holding it", out: fmt.Sprintf("%#v", holder)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.out, redactTestSecret) {
				t.Fatalf("%s wrote the plaintext: %s", tc.name, tc.out)
			}
		})
	}
}

func TestCredentialGoStringKeepsTheIdentityFields(t *testing.T) {
	out := fmt.Sprintf("%#v", redactTestCredential())
	for _, want := range []string{"ep_1", "key_1", "dev@example.com"} {
		if !strings.Contains(out, want) {
			t.Fatalf("GoString dropped %q, which is the field that makes a log line traceable: %s", want, out)
		}
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("GoString carries no redaction marker: %s", out)
	}
}

// TestCredentialLogValueNamesTheAccount is the half that is not about the secret.
// Without a LogValue, slog reflects a Credential, finds only unexported fields, and
// writes an empty object: nothing leaks, and nothing says which account the call was
// for either.
func TestCredentialLogValueNamesTheAccount(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				return slog.Attr{}
			}
			return a
		},
	}))

	logger.Info("dispatch", "credential", redactTestCredential())
	line := buf.String()

	if strings.Contains(line, redactTestSecret) {
		t.Fatalf("the log line carries the plaintext: %s", line)
	}
	if !strings.Contains(line, "ep_1") {
		t.Fatalf("the log line names no account, so it cannot be traced to one: %s", line)
	}
}

func TestRedactedWhenSetKeepsAnAbsentCredentialAbsent(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: "unset"},
		{in: "   ", want: "unset"},
		{in: "pt-real", want: "[redacted]"},
	}
	for _, tc := range cases {
		if got := redactedWhenSet(tc.in); got != tc.want {
			t.Fatalf("redactedWhenSet(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
