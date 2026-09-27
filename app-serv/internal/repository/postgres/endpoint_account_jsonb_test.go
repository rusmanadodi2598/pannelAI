// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_account_jsonb_test.go
// @for       The stored shape of an endpoint's account identity, proven by
//
//	writing it and reading it back.
//
// @uses      domain, strings, testing.
// @reason    The account column is JSONB, so a field the payload struct does not
//
//	name is silently dropped rather than rejected: the write succeeds, the
//	read returns empty, and the loss surfaces much later as a provider
//	that authenticates as an anonymous device. A signed provider replays
//	one of those fields — the machine id a login minted — on every
//	request, which is what makes an in-memory round trip worth a test.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-27
package postgres

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestMarshalAccountStoresEveryIdentityField pins the stored keys, not just the
// Go struct: the JSONB column is read by a lookup that matches on the account's
// email and workspace, so a renamed key breaks matching without failing a build.
func TestMarshalAccountStoresEveryIdentityField(t *testing.T) {
	raw, err := marshalAccount(domain.EndpointAccount{
		Name: "Dodi", Email: "dev@example.com", MachineID: "machine-fixed", WorkspaceID: "user-7",
	})
	if err != nil {
		t.Fatalf("marshalAccount() error = %v", err)
	}
	for _, key := range []string{`"name"`, `"email"`, `"machine_id"`, `"workspace_id"`} {
		if !strings.Contains(raw, key) {
			t.Fatalf("stored account %q is missing %s", raw, key)
		}
	}
}

// TestAccountRoundTripKeepsTheMachineID is the field a signed provider reads back.
func TestAccountRoundTripKeepsTheMachineID(t *testing.T) {
	stored := domain.EndpointAccount{
		Name: "Dodi", Email: "dev@example.com", MachineID: "machine-fixed", WorkspaceID: "user-7",
	}
	raw, err := marshalAccount(stored)
	if err != nil {
		t.Fatalf("marshalAccount() error = %v", err)
	}

	read, err := unmarshalAccount(raw)
	if err != nil {
		t.Fatalf("unmarshalAccount() error = %v", err)
	}
	if read != stored {
		t.Fatalf("account = %+v after the round trip, want %+v", read, stored)
	}
}

// TestUnmarshalAccountAcceptsAnEmptyColumn pins the three shapes the table can hold
// for an account: SQL NULL, the JSON null literal, and an empty string. None of
// them is an error, because an endpoint created without an identity is legal and
// the read must not invent one.
func TestUnmarshalAccountAcceptsAnEmptyColumn(t *testing.T) {
	for _, raw := range []string{"", "null", "{}"} {
		account, err := unmarshalAccount(raw)
		if err != nil {
			t.Fatalf("unmarshalAccount(%q) error = %v", raw, err)
		}
		if account != (domain.EndpointAccount{}) {
			t.Fatalf("unmarshalAccount(%q) = %+v, want an empty identity", raw, account)
		}
	}
}

// TestUnmarshalAccountRefusesGarbage keeps a corrupted column a repository error
// rather than an empty account, because an empty account on a signed provider is
// read as "no machine id" and every later request is signed as a new device.
func TestUnmarshalAccountRefusesGarbage(t *testing.T) {
	if _, err := unmarshalAccount(`{"email":`); err == nil {
		t.Fatal("a truncated account json must not decode quietly")
	}
}
