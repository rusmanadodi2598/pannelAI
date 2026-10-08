// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/credential_redaction_test.go
// @for       The credential as one field of the structs this plane logs and dumps.
// @uses      fmt, internal/provider, strings, testing.
// @reason    Redaction on a credential only counts if it survives being one field deep, and Call and Selection are the two shapes that reach a log line or a test failure while holding one.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-08
package dataplane

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

const redactionTestSecret = "pt-SUPERSECRET-material"

// TestCarriedCredentialStaysRedacted covers both holders and the verbs that reach
// them. `%#v` is the one fmt does not route through a field's String method, so it is
// the case that stayed open after provider.Credential grew one, and the reason
// GoString exists.
func TestCarriedCredentialStaysRedacted(t *testing.T) {
	cred, err := provider.NewCredential(provider.CredentialInput{
		EndpointID: "ep_1", KeyID: "key_1", APIKey: redactionTestSecret,
		Family: provider.FamilyStaticKey, Account: "dev@example.com",
	})
	if err != nil {
		t.Fatalf("NewCredential() error = %v", err)
	}

	cases := []struct {
		name string
		out  string
	}{
		{name: "%v on a Call", out: fmt.Sprintf("%v", Call{Credential: cred})},
		{name: "%+v on a Call", out: fmt.Sprintf("%+v", Call{Credential: cred})},
		{name: "%#v on a Call", out: fmt.Sprintf("%#v", Call{Credential: cred})},
		{name: "%v on a Selection", out: fmt.Sprintf("%v", Selection{Credential: cred})},
		{name: "%+v on a Selection", out: fmt.Sprintf("%+v", Selection{Credential: cred})},
		{name: "%#v on a Selection", out: fmt.Sprintf("%#v", Selection{Credential: cred})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.out, redactionTestSecret) {
				t.Fatalf("%s wrote the plaintext: %s", tc.name, tc.out)
			}
		})
	}
}
