// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_parity_test.go
// @for       The connection-parity fields end to end through the service: a PATCH
//
//	that sets them, a read that returns them, and the two refusals.
//
// @uses      internal/domain, context, errors, testing, time.
// @reason    Draft 017 §4.1b adds five fields, and the failure mode that matters is
//
//	a field that is accepted and then lost between the write and the read.
//	The stub store carries them explicitly (endpoint_stub_test.go), so this
//	test proves the whole path rather than the aggregate alone.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubProxyPoolFinder answers a fixed set of stored pool ids.
type stubProxyPoolFinder struct {
	known []string
	err   error
}

func (f stubProxyPoolFinder) ProxyPoolExists(_ context.Context, id string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	for _, known := range f.known {
		if known == id {
			return true, nil
		}
	}
	return false, nil
}

// TestEndpointService_UpdateCarriesTheParityFields is the round trip: set each
// field through a patch and read it back from the store.
func TestEndpointService_UpdateCarriesTheParityFields(t *testing.T) {
	cases := []struct {
		name  string
		patch UpdatePatch
		want  func(t *testing.T, got domain.UpstreamEndpoint)
	}{
		{
			name:  "a default model round-trips",
			patch: UpdatePatch{DefaultModel: strPtr("gpt-4o")},
			want: func(t *testing.T, got domain.UpstreamEndpoint) {
				if got.DefaultModel() != "gpt-4o" {
					t.Fatalf("DefaultModel() = %q, want gpt-4o", got.DefaultModel())
				}
			},
		},
		{
			name:  "a global priority round-trips",
			patch: UpdatePatch{GlobalPriority: intPtr(3)},
			want: func(t *testing.T, got domain.UpstreamEndpoint) {
				if got.GlobalPriority() != 3 {
					t.Fatalf("GlobalPriority() = %d, want 3", got.GlobalPriority())
				}
			},
		},
		{
			name:  "a proxy binding round-trips when the pool exists",
			patch: UpdatePatch{ProxyPoolID: strPtr("proxy-01ABC")},
			want: func(t *testing.T, got domain.UpstreamEndpoint) {
				if got.ProxyPoolID() != "proxy-01ABC" {
					t.Fatalf("ProxyPoolID() = %q, want proxy-01ABC", got.ProxyPoolID())
				}
			},
		},
		{
			name:  "a cleared default model round-trips as empty",
			patch: UpdatePatch{DefaultModel: strPtr("")},
			want: func(t *testing.T, got domain.UpstreamEndpoint) {
				if got.DefaultModel() != "" {
					t.Fatalf("DefaultModel() = %q, want empty", got.DefaultModel())
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newParityEndpointService(t)
			seeded := seedEndpoint(t, svc, "openai", "Primary", domain.UpstreamAuthAPIKey, KeyInput{Value: "sk-live-abcdef"})

			updated, err := svc.Update(context.Background(), seeded.ID(), tc.patch)
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			tc.want(t, updated)

			// Read it back from the store: the value the response carries and the
			// value a later read returns must be the same one.
			reloaded, err := store.GetByID(context.Background(), seeded.ID())
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			tc.want(t, reloaded)
		})
	}
}

// TestEndpointService_UpdateRefusesABadRoutingValue covers the two refusals, and
// asserts neither of them left a partial write behind.
func TestEndpointService_UpdateRefusesABadRoutingValue(t *testing.T) {
	cases := []struct {
		name  string
		patch UpdatePatch
		want  string
	}{
		{
			name:  "a negative global priority is refused",
			patch: UpdatePatch{GlobalPriority: intPtr(-1)},
			want:  "global_priority",
		},
		{
			name:  "a proxy binding to an unknown pool is refused",
			patch: UpdatePatch{ProxyPoolID: strPtr("no-such-pool")},
			want:  "proxy_pool_id",
		},
		{
			name:  "an over-long default model is refused",
			patch: UpdatePatch{DefaultModel: strPtr(strings.Repeat("x", 201))},
			want:  "default_model",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newParityEndpointService(t)
			seeded := seedEndpoint(t, svc, "openai", "Primary", domain.UpstreamAuthAPIKey, KeyInput{Value: "sk-live-abcdef"})

			_, err := svc.Update(context.Background(), seeded.ID(), tc.patch)
			if err == nil {
				t.Fatal("Update() returned no error for an invalid value")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to name %q", err, tc.want)
			}
			reloaded, err := store.GetByID(context.Background(), seeded.ID())
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			if reloaded.GlobalPriority() != 0 || reloaded.DefaultModel() != "" || reloaded.ProxyPoolID() != "" {
				t.Fatalf("a refused patch left a partial write: priority=%d model=%q proxy=%q",
					reloaded.GlobalPriority(), reloaded.DefaultModel(), reloaded.ProxyPoolID())
			}
		})
	}
}

// TestEndpointService_UpdateWithoutAProxyStoreRefusesTheBinding pins the
// fail-closed rule: a deployment that cannot check must not store a name nothing
// can resolve.
func TestEndpointService_UpdateWithoutAProxyStoreRefusesTheBinding(t *testing.T) {
	store := newMemEndpointStore()
	svc, err := NewEndpointService(EndpointServiceDeps{
		Store: store, Index: newFakeIndex("openai"),
		Sealer: newTestSealer(t), Prober: &fakeProber{},
	})
	if err != nil {
		t.Fatalf("NewEndpointService() error = %v", err)
	}
	svc.clock = func() time.Time { return testNow }
	seeded := seedEndpoint(t, svc, "openai", "Primary", domain.UpstreamAuthAPIKey, KeyInput{Value: "sk-live-abcdef"})

	if _, err := svc.Update(context.Background(), seeded.ID(), UpdatePatch{ProxyPoolID: strPtr("proxy-01ABC")}); err == nil {
		t.Fatal("Update() stored a proxy binding with no proxy store wired")
	}
}

// newParityEndpointService wires an endpoint service with a proxy store holding
// one pool, so the binding case has something to resolve.
func newParityEndpointService(t *testing.T) (*EndpointService, *memEndpointStore) {
	t.Helper()
	store := newMemEndpointStore()
	svc, err := NewEndpointService(EndpointServiceDeps{
		Store: store, Index: newFakeIndex("openai"),
		Sealer: newTestSealer(t), Prober: &fakeProber{},
		Proxies: stubProxyPoolFinder{known: []string{"proxy-01ABC"}},
	})
	if err != nil {
		t.Fatalf("NewEndpointService() error = %v", err)
	}
	svc.clock = func() time.Time { return testNow }
	return svc, store
}
