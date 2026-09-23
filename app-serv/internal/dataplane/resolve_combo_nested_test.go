// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/resolve_combo_nested_test.go
// @for       Draft 024 F1: a combo member that is itself a combo resolves one
//
//	dereference level deep at runtime, exactly as the write path accepted
//	it, and a cycle between two combos terminates instead of recursing.
//
// @uses      internal/domain, context, testing.
// @reason    §7.7 accepts a combo name as a member and §7.15 resolves a combo
//
//	name as a model string, but resolveMember skipped the combo path, so a
//	nested member saved and then answered MODEL_NOT_FOUND. The write path
//	and the runtime must agree about what a saved member means; these
//	tests pin the runtime half and the cycle guard that makes the
//	agreement safe.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package dataplane

import (
	"context"

	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// nestedLookup is a lookup holding a chain of combos so a test can build the
// nested and the cyclic shapes without a database.
func nestedLookup() fakeLookup {
	return fakeLookup{
		combos: map[string]domain.Combo{
			"inner":       comboRow("inner", "provider-a/fast", "provider-a/backup"),
			"outer":       comboRow("outer", "inner"),
			"outer-last":  comboRow("outer-last", "claude-only/x", "inner"),
			"cycle-a":     comboRow("cycle-a", "cycle-b"),
			"cycle-b":     comboRow("cycle-b", "cycle-a"),
			"deep-a":      comboRow("deep-a", "deep-b"),
			"deep-b":      comboRow("deep-b", "provider-a/fast"),
			"empty-out":   comboRow("empty-out", "empty-inner"),
			"empty-inner": comboRow("empty-inner"),
		},
		aliases: map[string]string{"to-inner": "inner"},
	}
}

// TestResolver_NestedComboMember covers every shape a nested reference can take:
// the leading member, a later member, the member reached through an alias, a
// chain one level long, and the two shapes that must not resolve — an empty
// inner combo and a cycle.
func TestResolver_NestedComboMember(t *testing.T) {
	resolver, err := NewResolver(testIndex(t), nestedLookup())
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}

	cases := []struct {
		name         string
		model        string
		wantProvider string
		wantModel    string
		wantCombo    string
		wantErrCode  string
	}{
		{
			name:  "a nested member resolves to the inner combo's first member",
			model: "outer", wantProvider: "provider-a", wantModel: "fast", wantCombo: "outer",
		},
		{
			name:  "a nested member in a later position is reached when the leader fails",
			model: "outer-last", wantProvider: "claude-only", wantModel: "x", wantCombo: "outer-last",
		},
		{
			name:  "an alias naming an inner combo resolves through it",
			model: "to-inner", wantProvider: "provider-a", wantModel: "fast", wantCombo: "inner",
		},
		{
			name:  "a one-level chain resolves to the model behind it",
			model: "deep-a", wantProvider: "provider-a", wantModel: "fast", wantCombo: "deep-a",
		},
		{
			name:  "an empty inner combo is MODEL_NOT_FOUND, not an empty answer",
			model: "empty-out", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "a cycle between two combos terminates as MODEL_NOT_FOUND",
			model: "cycle-a", wantErrCode: CodeModelNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolver.Resolve(context.Background(), tc.model)
			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatalf("Resolve(%q) err = nil, want %s (got %+v)", tc.model, tc.wantErrCode, got)
				}
				if code := AsError(err).Code; code != tc.wantErrCode {
					t.Fatalf("Resolve(%q) code = %q, want %q", tc.model, code, tc.wantErrCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q) err = %v, want none", tc.model, err)
			}
			if got.Provider.ID != tc.wantProvider {
				t.Fatalf("Resolve(%q) provider = %q, want %q", tc.model, got.Provider.ID, tc.wantProvider)
			}
			if got.ModelID != tc.wantModel {
				t.Fatalf("Resolve(%q) model = %q, want %q", tc.model, got.ModelID, tc.wantModel)
			}
			if got.Combo.Name() != tc.wantCombo {
				t.Fatalf("Resolve(%q) combo = %q, want %q", tc.model, got.Combo.Name(), tc.wantCombo)
			}
		})
	}
}

// TestResolver_NestedComboMemberReportsItsOwnCombo pins which combo the answer
// carries: the one the client addressed, not the inner one. A response
// identity, a rotation key, and a usage row are all keyed by that name, so the
// inner name leaking out would attribute the request to the wrong combo.
func TestResolver_NestedComboMemberReportsItsOwnCombo(t *testing.T) {
	resolver, err := NewResolver(testIndex(t), nestedLookup())
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}
	got, err := resolver.Resolve(context.Background(), "outer")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Combo.Name() != "outer" {
		t.Fatalf("combo = %q, want the addressed %q", got.Combo.Name(), "outer")
	}
	if len(got.Combo.Refs()) != 1 || got.Combo.Refs()[0] != "inner" {
		t.Fatalf("refs = %v, want the stored member list", got.Combo.Refs())
	}
}

// TestResolver_NestedComboCycleDoesNotRecurse is the safety half: a stored cycle
// must produce an error, never an unbounded recursion. The test itself would
// crash the process on a stack overflow, so its passing is the proof.
func TestResolver_NestedComboCycleDoesNotRecurse(t *testing.T) {
	resolver, err := NewResolver(testIndex(t), nestedLookup())
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}
	_, err = resolver.Resolve(context.Background(), "cycle-a")
	if err == nil {
		t.Fatal("Resolve() accepted a cyclic combo chain")
	}
	if code := AsError(err).Code; code != CodeModelNotFound {
		t.Fatalf("cycle code = %q, want %q", code, CodeModelNotFound)
	}
}
