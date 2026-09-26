// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings_provider_proxy_test.go
// @for       The write-time rule the per-provider binding adds: a pool id must
//
//	name a stored row before it is stored.
//
// @uses      context, testing, internal/domain.
// @reason    docs/PORT/009-PORT-PROVIDER-PROXY.md D6: a dangling id is the
//
//	failure mode that matters, because the route plan would silently fall
//	through to the rest of the pool and the operator would never learn
//	their binding does nothing. The rule mirrors the endpoint parity
//	check, including its "no finder means refuse" direction.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSettingsService_Update_ChecksTheBoundPoolID pins the existence rule and
// its two carve-outs: an empty id is the global default, and the none sentinel
// is a value rather than a reference.
func TestSettingsService_Update_ChecksTheBoundPoolID(t *testing.T) {
	cases := []struct {
		name    string
		finder  ProxyPoolFinder
		entry   domain.ProviderProxy
		wantErr string
	}{
		{
			name:   "a stored row is accepted",
			finder: stubProxyPoolFinder{known: []string{"prx_a"}},
			entry:  domain.ProviderProxy{PoolID: "prx_a"},
		},
		{
			name:   "the none sentinel needs no row",
			finder: stubProxyPoolFinder{known: []string{"prx_a"}},
			entry:  domain.ProviderProxy{PoolID: domain.ProxyPoolNone},
		},
		{
			name:   "an entry without a pool needs no row",
			finder: stubProxyPoolFinder{known: []string{"prx_a"}},
			entry:  domain.ProviderProxy{Strategy: domain.ProxyStrategyRoundRobin},
		},
		{
			name:    "a dangling id is refused",
			finder:  stubProxyPoolFinder{known: []string{"prx_a"}},
			entry:   domain.ProviderProxy{PoolID: "prx_gone"},
			wantErr: "does not name a stored proxy pool",
		},
		{
			name:    "a deployment with no pool store refuses a non-empty id",
			finder:  nil,
			entry:   domain.ProviderProxy{PoolID: "prx_a"},
			wantErr: "no proxy store is configured",
		},
		{
			name:   "a deployment with no pool store still accepts the sentinel",
			finder: nil,
			entry:  domain.ProviderProxy{PoolID: domain.ProxyPoolNone},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newStubSettingsStore()
			svc, err := NewSettingsService(SettingsServiceDeps{Repo: repo, Proxies: tc.finder})
			if err != nil {
				t.Fatalf("NewSettingsService() error = %v", err)
			}
			_, err = svc.Update(context.Background(), domain.SettingsPatch{Network: &domain.NetworkSettingsPatch{
				ProviderProxies: &map[string]domain.ProviderProxy{"openai": tc.entry},
			}})
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("Update() = nil error, want the binding refused")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err, tc.wantErr)
				}
				if len(repo.written) != 0 {
					t.Fatalf("written = %v, want nothing persisted after a refusal", repo.written)
				}
				return
			}
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			read, err := svc.Settings(context.Background())
			if err != nil {
				t.Fatalf("Settings() error = %v", err)
			}
			if read.Network.ProviderProxies["openai"].PoolID != tc.entry.PoolID {
				t.Fatalf("stored entry = %+v, want %+v", read.Network.ProviderProxies["openai"], tc.entry)
			}
		})
	}
}

// TestSettingsService_Update_IgnoresThePoolStoreWhenThePatchCarriesNoBinding
// keeps a deployment that never bound a provider from paying for (or failing
// on) a check the patch did not ask for.
func TestSettingsService_Update_IgnoresThePoolStoreWhenThePatchCarriesNoBinding(t *testing.T) {
	repo := newStubSettingsStore()
	svc, err := NewSettingsService(SettingsServiceDeps{Repo: repo})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}
	if _, err := svc.Update(context.Background(), domain.SettingsPatch{Network: &domain.NetworkSettingsPatch{
		OutboundProxyStrategy: strPtrBinding(domain.ProxyStrategyRoundRobin),
	}}); err != nil {
		t.Fatalf("Update() error = %v, want the strategy patch accepted without a pool store", err)
	}
}

func strPtrBinding(value string) *string { return &value }
