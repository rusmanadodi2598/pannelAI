// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin_test.go
// @for       Table-driven tests for connector registration, lookup, and the
//
//	per-provider fallback.
//
// @uses      testing, net/http, internal/registry.
// @reason    The seam exists so a provider can be patched or added without
//
//	touching shared code, which only holds if the core truly routes
//	through lookup rather than branching on ids. These tests pin that
//	property and the URL/auth rules that made the branch unnecessary.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package provider

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stubPlugin is a connector used to prove lookup behaviour without needing a
// real provider's protocol.
type stubPlugin struct {
	Base
}

func (s *stubPlugin) Endpoint(Request, Credential) (string, error) {
	return "https://stub.test/v1", nil
}
func (s *stubPlugin) ApplyAuth(*http.Request, Credential) error { return nil }

func newStub(id string) *stubPlugin {
	return &stubPlugin{Base: Base{ID: id, Auth: "api_key", Format: "openai"}}
}

func TestNewConnectors_RegistersAndRejectsDuplicates(t *testing.T) {
	cases := []struct {
		name    string
		plugins []Plugin
		wantErr bool
		wantIDs []string
	}{
		{
			name:    "no specialized connectors is valid",
			plugins: nil,
			wantIDs: []string{},
		},
		{
			name:    "several connectors register",
			plugins: []Plugin{newStub("codex"), newStub("antigravity")},
			wantIDs: []string{"antigravity", "codex"},
		},
		{
			name:    "a duplicate provider id is refused",
			plugins: []Plugin{newStub("codex"), newStub("codex")},
			wantErr: true,
		},
		{
			name:    "a connector with no provider id is refused",
			plugins: []Plugin{newStub("")},
			wantErr: true,
		},
		{
			name:    "a nil connector is refused",
			plugins: []Plugin{nil},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			connectors, err := NewConnectors(DefaultFactory, tc.plugins...)
			if tc.wantErr {
				if err == nil {
					t.Fatal("NewConnectors() = nil error, want a refusal")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewConnectors() error = %v", err)
			}
			got := connectors.IDs()
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("IDs() = %v, want %v", got, tc.wantIDs)
			}
			for i, want := range tc.wantIDs {
				if got[i] != want {
					t.Fatalf("IDs()[%d] = %q, want %q", i, got[i], want)
				}
			}
			if connectors.Count() != len(tc.wantIDs) {
				t.Fatalf("Count() = %d, want %d", connectors.Count(), len(tc.wantIDs))
			}
		})
	}

	if _, err := NewConnectors(nil); err == nil {
		t.Fatal("NewConnectors(nil) must refuse a missing fallback factory")
	}
}

// TestConnectors_ForFallsBackPerProvider is the property the whole seam rests
// on: a provider with no connector is still routable, and a registered one is
// never shadowed by the fallback.
func TestConnectors_ForFallsBackPerProvider(t *testing.T) {
	special := newStub("codex")
	connectors, err := NewConnectors(DefaultFactory, special)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}

	cases := []struct {
		name     string
		provider registry.Provider
		wantID   string
		wantKind string
	}{
		{
			name:     "a registered provider resolves to its connector",
			provider: registry.Provider{ID: "codex", Transport: registry.Transport{Format: "openai-responses"}},
			wantID:   "codex",
			wantKind: "*provider.stubPlugin",
		},
		{
			name:     "an unregistered provider gets the fallback",
			provider: registry.Provider{ID: "deepseek", Transport: registry.Transport{Format: "openai"}},
			wantID:   "deepseek",
			wantKind: "*provider.Default",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectors.For(tc.provider)
			if got == nil {
				t.Fatal("For() = nil, want a connector")
			}
			if got.ProviderID() != tc.wantID {
				t.Fatalf("ProviderID() = %q, want %q", got.ProviderID(), tc.wantID)
			}
			if kind := kindOf(got); kind != tc.wantKind {
				t.Fatalf("connector kind = %s, want %s", kind, tc.wantKind)
			}
		})
	}

	if !connectors.Registered("codex") {
		t.Fatal("Registered(codex) = false, want true")
	}
	if connectors.Registered("deepseek") {
		t.Fatal("Registered(deepseek) = true, want false")
	}
}

// kindOf reports the concrete type name of a plugin, so a test can assert which
// implementation a lookup produced without exporting test-only accessors.
func kindOf(p Plugin) string {
	switch p.(type) {
	case *stubPlugin:
		return "*provider.stubPlugin"
	case *Default:
		return "*provider.Default"
	default:
		return "<unknown>"
	}
}
