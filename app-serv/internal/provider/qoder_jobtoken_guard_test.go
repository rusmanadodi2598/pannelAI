// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_jobtoken_guard_test.go
// @for       The boot-time refusals of the job-token exchanger: no base URL and
//
//	no guarded egress client both end the boot rather than the request.
//
// @uses      testing.
// @reason    The exchange is the one Qoder call that carries a customer's
//
//	Personal Access Token, so both halves of the guard the composition root
//	is trusted to wire are pinned here together (draft 042 R07,
//	docs/RULLES/SSRF.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-10-04
package provider

import (
	"testing"
)

// TestNewQoderJobTokenClientRefusesNoBase pins the boot-time guard: an entry without
// an openapi base cannot produce an exchange URL, and defaulting one would send a
// customer's Personal Access Token to an address the registry never declared.
func TestNewQoderJobTokenClientRefusesNoBase(t *testing.T) {
	for _, base := range []string{"", "   "} {
		if _, err := NewQoderJobTokenClient(base, nil); err == nil {
			t.Fatalf("base %q was accepted", base)
		}
	}
}

// TestNewQoderJobTokenClientRefusesNilClient pins the other half of the same
// boot-time guard: a client built here would carry no egress guard and no
// redirect rule, so a 30x from the exchange host could re-send the customer's
// Personal Access Token to a host the registry never named
// (docs/RULLES/SSRF.md §2.1).
func TestNewQoderJobTokenClientRefusesNilClient(t *testing.T) {
	if _, err := NewQoderJobTokenClient("https://openapi.qoder.sh", nil); err == nil {
		t.Fatal("NewQoderJobTokenClient() accepted a nil client, want a refusal")
	}
}
