//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/proxy_integration_test.go
// @for       The proxy repository against a real server: round trip, ordering, the migration's CHECK constraints, and the not-found mapping.
// @uses      testing, context, time, internal/domain.
// @reason    The protocol and port rules exist twice, in the domain and as CHECK constraints, and only a real server can prove the second one holds. A stub cannot fail a bad protocol the way PostgreSQL does, and the earlier P0 schema proved that divergence is real, not theoretical.
//
//	Run with:
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-19
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// newTestProxyRepo returns a proxy repository over a clean table.
func newTestProxyRepo(t *testing.T) *ProxyRepository {
	t.Helper()
	pool := newTestPool(t)
	if _, err := pool.Exec(context.Background(), `TRUNCATE proxies`); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	return NewProxyRepository(pool)
}

// mustProxy builds one candidate or fails the test.
func mustProxy(t *testing.T, label, host string, port int) domain.Proxy {
	t.Helper()
	proxy, err := domain.NewProxy("", label, domain.ProxyProtocolSOCKS5, host, port, "operator", "sealed-value", time.Now().UTC())
	if err != nil {
		t.Fatalf("NewProxy(%q) error = %v", label, err)
	}
	return proxy
}

// TestProxyRepository_RoundTripAndOrder pins that every column survives a
// create/read cycle and that List orders by label.
func TestProxyRepository_RoundTripAndOrder(t *testing.T) {
	repo := newTestProxyRepo(t)
	ctx := context.Background()

	for _, label := range []string{"zulu", "alpha", "mike"} {
		if err := repo.Create(ctx, mustProxy(t, label, "proxy.example.com", 1080)); err != nil {
			t.Fatalf("Create(%s) error = %v", label, err)
		}
	}

	listed, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("List() returned %d rows, want 3", len(listed))
	}
	want := []string{"alpha", "mike", "zulu"}
	for index, proxy := range listed {
		if proxy.Label() != want[index] {
			t.Fatalf("List()[%d] label = %q, want %q", index, proxy.Label(), want[index])
		}
	}

	loaded, err := repo.GetByID(ctx, listed[0].ID())
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if loaded.Host() != "proxy.example.com" || loaded.Port() != 1080 ||
		loaded.Protocol() != domain.ProxyProtocolSOCKS5 || loaded.Username() != "operator" {
		t.Fatalf("GetByID() = %+v, want the stored address and username", loaded)
	}
	if loaded.PasswordEncrypted() != "sealed-value" || !loaded.Enabled() {
		t.Fatalf("GetByID() = %+v, want the sealed password and the enabled flag", loaded)
	}
	if loaded.Status().State != "" || loaded.Status().CheckedAt != nil {
		t.Fatalf("GetByID() status = %+v, want the untested zero", loaded.Status())
	}
}

// TestProxyRepository_UpdatePersistsStatus pins that a recorded test result and
// a repoint are written by the same statement.
func TestProxyRepository_UpdatePersistsStatus(t *testing.T) {
	repo := newTestProxyRepo(t)
	ctx := context.Background()
	proxy := mustProxy(t, "pool", "proxy.example.com", 1080)
	if err := repo.Create(ctx, proxy); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	checked := time.Now().UTC().Truncate(time.Microsecond)
	if err := proxy.Repoint(domain.ProxyProtocolHTTP, "other.example.com", 3128, checked); err != nil {
		t.Fatalf("Repoint() error = %v", err)
	}
	// Repoint clears the status, so the result is recorded after it: one update
	// must persist both the new address and the new status.
	proxy.RecordTest(string(domain.EndpointTestOK), 23, "", checked)
	if err := repo.Update(ctx, proxy); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	loaded, err := repo.GetByID(ctx, proxy.ID())
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if loaded.Host() != "other.example.com" || loaded.Protocol() != domain.ProxyProtocolHTTP || loaded.Port() != 3128 {
		t.Fatalf("GetByID() address = %s/%s/%d, want http/other.example.com/3128",
			loaded.Protocol(), loaded.Host(), loaded.Port())
	}
	status := loaded.Status()
	if status.State != string(domain.EndpointTestOK) || status.LatencyMS != 23 {
		t.Fatalf("GetByID() status = %+v, want ok/23", status)
	}
	if status.CheckedAt == nil || !status.CheckedAt.Equal(checked) {
		t.Fatalf("GetByID() checked_at = %v, want %v", status.CheckedAt, checked)
	}
}

// TestProxyRepository_Constraints pins the migration's CHECK constraints, which
// the domain rules mirror but cannot enforce at rest.
func TestProxyRepository_Constraints(t *testing.T) {
	repo := newTestProxyRepo(t)
	ctx := context.Background()

	cases := []struct {
		name     string
		protocol string
		port     int
	}{
		{name: "a protocol outside the closed set", protocol: "ftp", port: 1080},
		{name: "a port below the range", protocol: "http", port: 0},
		{name: "a port above the range", protocol: "socks5", port: 70000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := domain.RehydrateProxy("prx_raw", "raw", domain.ProxyProtocol(tc.protocol), "proxy.example.com",
				tc.port, "", "", true, domain.ProxyTestStatus{}, time.Now().UTC(), time.Now().UTC())
			if err := repo.Create(ctx, proxy); err == nil {
				t.Fatalf("Create() accepted %s, want the constraint to refuse it", tc.name)
			}
		})
	}
}

// TestProxyRepository_NotFound pins the sentinel mapping on all three routes
// that can miss.
func TestProxyRepository_NotFound(t *testing.T) {
	repo := newTestProxyRepo(t)
	ctx := context.Background()
	missing := mustProxy(t, "pool", "proxy.example.com", 1080)

	if _, err := repo.GetByID(ctx, "prx_missing"); !errors.Is(err, domain.ErrProxyNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrProxyNotFound", err)
	}
	if err := repo.Update(ctx, missing); !errors.Is(err, domain.ErrProxyNotFound) {
		t.Fatalf("Update() error = %v, want ErrProxyNotFound", err)
	}
	if err := repo.Delete(ctx, "prx_missing"); !errors.Is(err, domain.ErrProxyNotFound) {
		t.Fatalf("Delete() error = %v, want ErrProxyNotFound", err)
	}
}

// TestIntegration_ProxyPinsCountsTheEndpointsThatHangOnAPool proves the read a bulk
// proxy erase refuses on: the pins are grouped per pool and provider by the server, and a
// pool nothing references is absent rather than a zero the caller would have to interpret.
func TestIntegration_ProxyPinsCountsTheEndpointsThatHangOnAPool(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	for _, statement := range []string{`TRUNCATE upstream_endpoints CASCADE`, `TRUNCATE proxies`} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("preparing the schema: %v", err)
		}
	}
	endpoints := NewEndpointRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)

	// Two of one provider and one of another pin the same pool; a second pool takes one
	// pin of its own; a third is nobody's egress path.
	for _, seed := range []struct {
		id, provider, poolID string
	}{
		{"ep_pin_a", "openai", "prx_pinned"},
		{"ep_pin_b", "openai", "prx_pinned"},
		{"ep_pin_c", "claude", "prx_pinned"},
		{"ep_pin_d", "gemini", "prx_other"},
		{"ep_pin_e", "deepseek", ""},
	} {
		endpoint, err := domain.NewUpstreamEndpoint(seed.id, seed.provider, seed.id, domain.UpstreamAuthNone, 1, now)
		if err != nil {
			t.Fatalf("building %s: %v", seed.id, err)
		}
		if err := endpoint.SetRouting(0, "", seed.poolID, now); err != nil {
			t.Fatalf("binding %s to %q: %v", seed.id, seed.poolID, err)
		}
		if err := endpoints.Create(ctx, endpoint); err != nil {
			t.Fatalf("storing %s: %v", seed.id, err)
		}
	}

	pins, err := endpoints.ProxyPins(ctx, []string{"prx_pinned", "prx_other", "prx_unused"})
	if err != nil {
		t.Fatalf("ProxyPins(): %v", err)
	}
	if got := len(pins["prx_pinned"]); got != 2 {
		t.Fatalf("prx_pinned rows = %d, want one per provider that pins it", got)
	}
	if got := pins["prx_pinned"][0]; got.ProviderID != "claude" || got.Endpoints != 1 {
		t.Fatalf("first pin = %+v, want claude with its one endpoint", got)
	}
	if got := pins["prx_pinned"][1]; got.ProviderID != "openai" || got.Endpoints != 2 {
		t.Fatalf("second pin = %+v, want openai with its two endpoints", got)
	}
	if got := pins["prx_other"]; len(got) != 1 || got[0].Endpoints != 1 {
		t.Fatalf("prx_other pins = %+v, want gemini's single endpoint", got)
	}
	if _, ok := pins["prx_unused"]; ok {
		t.Fatal("a pool nothing pins was reported as if it were")
	}
}
