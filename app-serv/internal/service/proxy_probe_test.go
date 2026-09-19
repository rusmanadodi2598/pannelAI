// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_probe_test.go
// @for       The two proxy test routes' use case (SPEC-API-001 §7.11).
// @uses      testing, context, errors, internal/domain.
// @reason    §7.11's test is a diagnostic, so the tests pin three properties an
//
//	operator depends on: the stored status matches the answer, a refused
//	destination is reported as a failure rather than a pass (OWASP A10,
//	fail closed), and the probe receives the unsealed password while the
//	stored row keeps only the sealed form.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestProxyService_Test pins the stored route: the answer and the stored status
// agree, and the probe is handed the unsealed password.
func TestProxyService_Test(t *testing.T) {
	ctx := context.Background()
	prober := &stubProxyProber{result: ProxyProbeResult{State: domain.EndpointTestOK, LatencyMS: 18}}
	service, repo := newProxyFixture(t, prober)
	created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	status, err := service.Test(ctx, created.ID())
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if status.State != domain.EndpointTestOK || status.LatencyMS != 18 || status.CheckedAt == nil {
		t.Fatalf("Test() status = %+v, want ok/18 with a timestamp", status)
	}
	if len(prober.targets) != 1 {
		t.Fatalf("prober calls = %d, want 1", len(prober.targets))
	}
	target := prober.targets[0]
	if target.Host != "proxy.example.com" || target.Port != 3128 || target.Protocol != domain.ProxyProtocolHTTP {
		t.Fatalf("prober target = %+v, want the stored address", target)
	}
	if target.Password != "s3cret-value" || target.Username != "operator" {
		t.Fatalf("prober target credentials = %q/%q, want the opened pair", target.Username, target.Password)
	}

	stored, err := repo.GetByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status().State != domain.EndpointTestOK || stored.Status().LatencyMS != 18 {
		t.Fatalf("stored status = %+v, want the probed result", stored.Status())
	}
	if !isSealed(stored.PasswordEncrypted()) {
		t.Fatalf("stored password = %q, want it to stay sealed", stored.PasswordEncrypted())
	}
}

// TestProxyService_Test_RefusalIsAResult pins that a refused destination is
// reported as a failure with its reason, not as an error and never as a pass.
func TestProxyService_Test_RefusalIsAResult(t *testing.T) {
	ctx := context.Background()
	prober := &stubProxyProber{result: ProxyProbeResult{
		State: domain.EndpointTestFail, LatencyMS: 0,
		Message: "the proxy address was refused: a private address",
	}}
	service, repo := newProxyFixture(t, prober)
	created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	status, err := service.Test(ctx, created.ID())
	if err != nil {
		t.Fatalf("Test() error = %v, want the refusal reported as a result", err)
	}
	if status.State != domain.EndpointTestFail {
		t.Fatalf("Test() state = %q, want fail", status.State)
	}
	if status.Message == "" {
		t.Fatal("Test() dropped the reason a refusal carries")
	}
	stored, err := repo.GetByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status().State != domain.EndpointTestFail || stored.Status().Message == "" {
		t.Fatalf("stored status = %+v, want the failure and its reason", stored.Status())
	}
}

// TestProxyService_Test_UnknownStateFailsClosed pins that a state this layer
// does not recognize is recorded as a failure, never as a pass.
func TestProxyService_Test_UnknownStateFailsClosed(t *testing.T) {
	ctx := context.Background()
	for _, state := range []string{"", "unknown", "OK"} {
		t.Run("state="+state, func(t *testing.T) {
			service, _ := newProxyFixture(t, &stubProxyProber{result: ProxyProbeResult{State: state}})
			created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			status, err := service.Test(ctx, created.ID())
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			if status.State != domain.EndpointTestFail {
				t.Fatalf("Test() state = %q, want fail for an unrecognized state", status.State)
			}
		})
	}
}

// TestProxyService_Test_NoPassword pins that a candidate without a secret probes
// anonymously rather than with an empty credential it never had.
func TestProxyService_Test_NoPassword(t *testing.T) {
	ctx := context.Background()
	prober := &stubProxyProber{result: ProxyProbeResult{State: domain.EndpointTestOK}}
	service, _ := newProxyFixture(t, prober)
	created, err := service.Create(ctx, ProxyDraft{
		Label: "pool", Protocol: domain.ProxyProtocolSOCKS5, Host: "proxy.example.com", Port: 1080,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.Test(ctx, created.ID()); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if prober.targets[0].Password != "" || prober.targets[0].Username != "" {
		t.Fatalf("prober target = %+v, want no credentials", prober.targets[0])
	}
}

// TestProxyService_Test_UnknownID pins that an unreadable candidate is an error.
func TestProxyService_Test_UnknownID(t *testing.T) {
	service, _ := newProxyFixture(t, nil)
	if _, err := service.Test(context.Background(), "prx_missing"); !errors.Is(err, domain.ErrProxyNotFound) {
		t.Fatalf("Test() error = %v, want ErrProxyNotFound", err)
	}
}

// TestProxyService_Test_ProberFailure pins that an adapter fault is an error
// rather than a fabricated result.
func TestProxyService_Test_ProberFailure(t *testing.T) {
	ctx := context.Background()
	service, _ := newProxyFixture(t, &stubProxyProber{err: errors.New("the dialer is misconfigured")})
	created, err := service.Create(ctx, proxyDraft("pool", "proxy.example.com", 3128))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.Test(ctx, created.ID()); err == nil {
		t.Fatal("Test() = nil error, want the adapter fault reported")
	}
}

// TestProxyService_TestCandidate pins the unsaved route: it probes and answers
// without writing anything.
func TestProxyService_TestCandidate(t *testing.T) {
	ctx := context.Background()
	prober := &stubProxyProber{result: ProxyProbeResult{State: domain.EndpointTestOK, LatencyMS: 7}}
	service, repo := newProxyFixture(t, prober)

	status, err := service.TestCandidate(ctx, ProxyTarget{
		Protocol: domain.ProxyProtocolHTTPS, Host: "proxy.example.com", Port: 8443,
		Username: "operator", Password: "typed-once",
	})
	if err != nil {
		t.Fatalf("TestCandidate() error = %v", err)
	}
	if status.State != domain.EndpointTestOK || status.LatencyMS != 7 || status.CheckedAt == nil {
		t.Fatalf("TestCandidate() status = %+v, want ok/7 with a timestamp", status)
	}
	if len(prober.targets) != 1 || prober.targets[0].Password != "typed-once" {
		t.Fatalf("prober targets = %+v, want the typed candidate", prober.targets)
	}
	if repo.updates != 0 || len(repo.byID) != 0 {
		t.Fatalf("TestCandidate() wrote to the store: %d updates, %d rows", repo.updates, len(repo.byID))
	}
}
