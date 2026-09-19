// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_test.go
// @for       The proxy pool's create, update, and delete rules (SPEC-API-001 §7.11).
// @uses      testing, context, errors, strings, internal/domain.
// @reason    §7.11 makes the password write-only and the status a proof that a
//
//	candidate was tested; the tests pin that a save which changes nothing
//	does not erase the proof, and that an empty password does not erase the
//	secret — the two silent losses an operator would only notice later.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestProxyService_Create pins the create rules, including that the stored
// password is sealed and never the plaintext.
func TestProxyService_Create(t *testing.T) {
	ctx := context.Background()
	disabled := false

	cases := []struct {
		name        string
		draft       ProxyDraft
		wantErr     string
		wantEnabled bool
		wantSealed  bool
	}{
		{name: "a candidate with a password", draft: proxyDraft("pool", "proxy.example.com", 3128), wantEnabled: true, wantSealed: true},
		{name: "a candidate without a password", draft: ProxyDraft{
			Label: "pool", Protocol: domain.ProxyProtocolSOCKS5, Host: "proxy.example.com", Port: 1080,
		}, wantEnabled: true},
		{name: "a disabled candidate", draft: ProxyDraft{
			Label: "pool", Protocol: domain.ProxyProtocolHTTP, Host: "proxy.example.com", Port: 3128, Enabled: &disabled,
		}},
		{name: "a URL in the host field", draft: proxyDraft("pool", "http://proxy.example.com", 3128), wantErr: "bare hostname"},
		{name: "a port outside the range", draft: proxyDraft("pool", "proxy.example.com", 70000), wantErr: "between 1 and 65535"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, repo := newProxyFixture(t, nil)
			proxy, err := service.Create(ctx, tc.draft)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Create() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Create() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			stored, err := repo.GetByID(ctx, proxy.ID())
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			if stored.Enabled() != tc.wantEnabled {
				t.Fatalf("stored enabled = %v, want %v", stored.Enabled(), tc.wantEnabled)
			}
			if stored.HasPassword() != tc.wantSealed {
				t.Fatalf("stored has_password = %v, want %v", stored.HasPassword(), tc.wantSealed)
			}
			if tc.wantSealed {
				if !isSealed(stored.PasswordEncrypted()) || stored.PasswordEncrypted() == tc.draft.Password {
					t.Fatalf("stored password = %q, want a sealed value that is not the plaintext", stored.PasswordEncrypted())
				}
			}
		})
	}
}

// TestProxyService_Update pins the patch rules: a save that changes nothing
// keeps the last test result, a repoint clears it, and an empty password keeps
// the stored secret.
func TestProxyService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("renaming keeps the last test result", func(t *testing.T) {
		service, repo := newProxyFixture(t, nil)
		created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if _, err := service.Test(ctx, created.ID()); err != nil {
			t.Fatalf("Test() error = %v", err)
		}

		draft := proxyDraft("renamed", "proxy.example.com", 3128)
		updated, err := service.Update(ctx, created.ID(), draft)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.Label() != "renamed" {
			t.Fatalf("Update() label = %q, want renamed", updated.Label())
		}
		if updated.Status().State != domain.EndpointTestOK {
			t.Fatalf("Update() cleared a status it did not invalidate: %+v", updated.Status())
		}
		if _, err := repo.GetByID(ctx, created.ID()); err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
	})

	t.Run("repointing clears the last test result", func(t *testing.T) {
		service, _ := newProxyFixture(t, nil)
		created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if _, err := service.Test(ctx, created.ID()); err != nil {
			t.Fatalf("Test() error = %v", err)
		}

		updated, err := service.Update(ctx, created.ID(), proxyDraft("pool", "other.example.com", 3128))
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.Status().State != "" || updated.Status().CheckedAt != nil {
			t.Fatalf("Update() kept a status measured against the old address: %+v", updated.Status())
		}
	})

	t.Run("an empty password keeps the stored secret", func(t *testing.T) {
		service, _ := newProxyFixture(t, nil)
		created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		draft := proxyDraft("pool", "proxy.example.com", 3128)
		draft.Password = ""
		updated, err := service.Update(ctx, created.ID(), draft)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.PasswordEncrypted() != created.PasswordEncrypted() {
			t.Fatal("Update() replaced a secret the caller did not send")
		}
	})

	t.Run("a new password replaces the secret", func(t *testing.T) {
		service, _ := newProxyFixture(t, nil)
		created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		draft := proxyDraft("pool", "proxy.example.com", 3128)
		draft.Password = "rotated-value"
		updated, err := service.Update(ctx, created.ID(), draft)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		opened, err := newTestSealer(t).Open(updated.PasswordEncrypted())
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if opened != "rotated-value" {
			t.Fatalf("stored password opens to %q, want the rotated value", opened)
		}
	})

	t.Run("disabling keeps the address", func(t *testing.T) {
		service, _ := newProxyFixture(t, nil)
		created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		disabled := false
		draft := proxyDraft("pool", "proxy.example.com", 3128)
		draft.Enabled = &disabled
		updated, err := service.Update(ctx, created.ID(), draft)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.Enabled() || updated.Host() != "proxy.example.com" || !updated.HasPassword() {
			t.Fatalf("Update() = %+v, want disabled with the address and secret kept", updated)
		}
	})

	t.Run("an unknown id", func(t *testing.T) {
		service, _ := newProxyFixture(t, nil)
		if _, err := service.Update(ctx, "prx_missing", proxyDraft("pool", "proxy.example.com", 3128)); !errors.Is(err, domain.ErrProxyNotFound) {
			t.Fatalf("Update() error = %v, want ErrProxyNotFound", err)
		}
	})
}

// TestProxyService_Delete pins removal and the not-found mapping.
func TestProxyService_Delete(t *testing.T) {
	ctx := context.Background()
	service, repo := newProxyFixture(t, nil)
	created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := service.Delete(ctx, created.ID()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.GetByID(ctx, created.ID()); !errors.Is(err, domain.ErrProxyNotFound) {
		t.Fatalf("GetByID() after Delete() = %v, want ErrProxyNotFound", err)
	}
	if err := service.Delete(ctx, created.ID()); !errors.Is(err, domain.ErrProxyNotFound) {
		t.Fatalf("Delete() again = %v, want ErrProxyNotFound", err)
	}
}
