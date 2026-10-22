// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings_proxy_mask_test.go
// @for       The PATCH merge that keeps a stored proxy password when the panel sends the response mask back.
// @uses      context, testing, internal/domain.
// @reason    The settings response masks the password, so an unchanged form would otherwise store the mask as the credential; the merge keeps the stored secret while still allowing host and username edits.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestSettingsUpdateKeepsTheStoredProxyPasswordWhenMasked(t *testing.T) {
	repo := newStubSettingsStore()
	if err := repo.Save(context.Background(), domain.SettingsKeyNetwork,
		`{"outbound_proxy_enabled":true,"outbound_proxy_url":"http://user:secret@proxy.internal:8080"}`); err != nil {
		t.Fatalf("seeding settings: %v", err)
	}
	svc := newSettingsUpdateFixture(t, repo)

	next, err := svc.Update(context.Background(), domain.SettingsPatch{
		Network: &domain.NetworkSettingsPatch{
			OutboundProxyURL: strPtr("http://user:" + domain.ProxyPasswordMask + "@proxy.internal:8080"),
		},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got := next.Network.OutboundProxyURL; got != "http://user:secret@proxy.internal:8080" {
		t.Fatalf("URL = %q, want the stored password preserved", got)
	}
}

func TestSettingsUpdateStoresANewProxyPassword(t *testing.T) {
	repo := newStubSettingsStore()
	if err := repo.Save(context.Background(), domain.SettingsKeyNetwork,
		`{"outbound_proxy_enabled":true,"outbound_proxy_url":"http://user:secret@proxy.internal:8080"}`); err != nil {
		t.Fatalf("seeding settings: %v", err)
	}
	svc := newSettingsUpdateFixture(t, repo)

	next, err := svc.Update(context.Background(), domain.SettingsPatch{
		Network: &domain.NetworkSettingsPatch{OutboundProxyURL: strPtr("http://user:new-secret@proxy.internal:8080")},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got := next.Network.OutboundProxyURL; got != "http://user:new-secret@proxy.internal:8080" {
		t.Fatalf("URL = %q, want the new password stored", got)
	}
}
