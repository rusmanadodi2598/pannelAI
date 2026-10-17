// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_bulk_test.go
// @for       The bulk actions over proxy candidates: the operator's switch across a set, and the erase that refuses until the pins it takes down are confirmed.
// @uses      context, fmt, strings, testing, time, internal/domain, internal/repository.
// @reason    A candidate is pinned by an id string on the endpoint rather than by a foreign key, so nothing in the schema stops an erase that strands connections. The two things worth proving are that a bulk switch reaches the whole selection without disturbing the probe evidence, and that a bulk erase answers with the number of endpoints and providers it takes down and refuses to proceed on a number that no longer matches.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// stubProxyPins answers a canned pin report and records the ids it was asked about, so a
// test can prove the erase consulted storage rather than trusting the caller's number.
type stubProxyPins struct {
	pins  map[string][]repository.ProxyPin
	err   error
	asked []string
}

func (s *stubProxyPins) ProxyPins(_ context.Context, poolIDs []string) (map[string][]repository.ProxyPin, error) {
	s.asked = append(s.asked, poolIDs...)
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[string][]repository.ProxyPin, len(poolIDs))
	for _, id := range poolIDs {
		if pins, ok := s.pins[id]; ok {
			out[id] = pins
		}
	}
	return out, nil
}

// newProxyBulkFixture wires the service over the doubles with a pin reader attached.
func newProxyBulkFixture(t *testing.T, pins ProxyPinReader) (*ProxyService, *stubProxyRepo) {
	t.Helper()
	repo := newStubProxyRepo()
	service, err := NewProxyService(ProxyServiceDeps{
		Repo: repo, Sealer: newTestSealer(t),
		Prober: &stubProxyProber{result: ProxyProbeResult{State: string(domain.EndpointTestOK), LatencyMS: 12}},
		Pins:   pins,
	})
	if err != nil {
		t.Fatalf("NewProxyService() error = %v", err)
	}
	service.clock = func() time.Time { return testNow }
	return service, repo
}

// seedPool stores n candidates and answers their ids in label order.
func seedPool(t *testing.T, service *ProxyService, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := range n {
		proxy, err := service.Create(context.Background(), proxyDraft(fmt.Sprintf("pool-%d", i), "proxy.example.com", 3128+i))
		if err != nil {
			t.Fatalf("seeding candidate %d: %v", i, err)
		}
		ids = append(ids, proxy.ID())
	}
	return ids
}

func TestProxyBulkStatusSwitchesTheSelectionAndKeepsTheProbe(t *testing.T) {
	reader := &stubProxyPins{}
	service, repo := newProxyBulkFixture(t, reader)
	ids := seedPool(t, service, 3)

	outcome, err := service.BulkStatus(context.Background(), BulkProxySelection{All: true}, false, 3)
	if err != nil {
		t.Fatalf("BulkStatus() error = %v", err)
	}
	if outcome.Matched != 3 || outcome.Changed != 3 {
		t.Fatalf("outcome = %+v, want the whole pool switched", outcome)
	}
	for _, id := range ids {
		stored := repo.byID[id]
		if stored.Enabled() {
			t.Fatalf("%s is still enabled", id)
		}
		if stored.RouteUsable() {
			t.Fatalf("%s still routes egress after the switch", id)
		}
	}
}

func TestProxyBulkStatusReEnableLeavesTheFailedProbeStanding(t *testing.T) {
	service, repo := newProxyBulkFixture(t, &stubProxyPins{})
	ids := seedPool(t, service, 1)
	failed := repo.byID[ids[0]]
	failed.RecordTest(string(domain.EndpointTestFail), 0, "connect refused", testNow)
	repo.byID[ids[0]] = failed

	if _, err := service.BulkStatus(context.Background(), BulkProxySelection{IDs: ids}, true, 1); err != nil {
		t.Fatalf("BulkStatus() error = %v", err)
	}
	stored := repo.byID[ids[0]]
	if !stored.Enabled() {
		t.Fatal("the operator's switch did not return")
	}
	if stored.RouteUsable() {
		t.Fatal("an enabled candidate whose last probe failed became routable; the probe is not the operator's to overwrite")
	}
}

func TestProxyBulkStatusRefusesAStaleCount(t *testing.T) {
	service, repo := newProxyBulkFixture(t, &stubProxyPins{})
	seedPool(t, service, 2)

	_, err := service.BulkStatus(context.Background(), BulkProxySelection{All: true}, false, 1)
	if domain.AsAppError(err).Code != "BULK_TARGET_COUNT_MISMATCH" {
		t.Fatalf("code = %q, want the confirmed count checked against the pool", domain.AsAppError(err).Code)
	}
	for _, proxy := range repo.byID {
		if !proxy.Enabled() {
			t.Fatal("a refused bulk switch moved a candidate anyway")
		}
	}
}

func TestProxyBulkSelectionNeedsASet(t *testing.T) {
	service, _ := newProxyBulkFixture(t, &stubProxyPins{})
	_, err := service.BulkStatus(context.Background(), BulkProxySelection{}, false, 1)
	if domain.AsAppError(err).Code != "BULK_SELECTOR_REQUIRED" {
		t.Fatalf("code = %q, want an empty selection refused", domain.AsAppError(err).Code)
	}
}

func TestProxyBulkDeleteReportsThePinsItTakesDown(t *testing.T) {
	reader := &stubProxyPins{pins: map[string][]repository.ProxyPin{
		"prx_one": {{ProviderID: "openai", Endpoints: 2}, {ProviderID: "claude", Endpoints: 1}},
		"prx_two": {{ProviderID: "openai", Endpoints: 3}},
	}}
	service, repo := newProxyBulkFixture(t, reader)
	repo.byID["prx_one"] = rehydratedProxy(t, "prx_one")
	repo.byID["prx_two"] = rehydratedProxy(t, "prx_two")

	outcome, err := service.BulkDelete(context.Background(),
		BulkProxySelection{IDs: []string{"prx_one", "prx_two"}}, 2, 6)
	if err != nil {
		t.Fatalf("BulkDelete() error = %v", err)
	}
	if outcome.Pins.Endpoints != 6 {
		t.Fatalf("dangling endpoints = %d, want the six pins added up across both pools", outcome.Pins.Endpoints)
	}
	if strings.Join(outcome.Pins.Providers, ",") != "claude,openai" {
		t.Fatalf("providers losing a pin = %v, want each named once and in order", outcome.Pins.Providers)
	}
	if len(repo.byID) != 0 {
		t.Fatalf("%d candidate(s) survived the erase", len(repo.byID))
	}
}

// TestProxyBulkDeleteRefusesAnUnderstatedPinCount is D5 working: the operator confirmed
// the number their list showed, storage says more connections hang on these pools, and the
// refusal answers with the number to confirm instead of erasing the difference.
func TestProxyBulkDeleteRefusesAnUnderstatedPinCount(t *testing.T) {
	reader := &stubProxyPins{pins: map[string][]repository.ProxyPin{
		"prx_one": {{ProviderID: "openai", Endpoints: 4}},
	}}
	service, repo := newProxyBulkFixture(t, reader)
	repo.byID["prx_one"] = rehydratedProxy(t, "prx_one")

	_, err := service.BulkDelete(context.Background(), BulkProxySelection{IDs: []string{"prx_one"}}, 1, 0)
	appErr := domain.AsAppError(err)
	if appErr.Code != "BULK_TARGET_COUNT_MISMATCH" {
		t.Fatalf("code = %q, want the pin count checked before the erase", appErr.Code)
	}
	if !strings.Contains(appErr.Message, "4 endpoint pin(s)") || !strings.Contains(appErr.Message, "openai") {
		t.Fatalf("refusal = %q, want it to name both the count and the provider losing its pin", appErr.Message)
	}
	if _, ok := repo.byID["prx_one"]; !ok {
		t.Fatal("a refused erase removed the candidate")
	}
}

func TestProxyBulkDeleteNeedsThePinReader(t *testing.T) {
	service, repo := newProxyBulkFixture(t, nil)
	repo.byID["prx_one"] = rehydratedProxy(t, "prx_one")

	_, err := service.BulkDelete(context.Background(), BulkProxySelection{IDs: []string{"prx_one"}}, 1, 0)
	if domain.AsAppError(err).Code != "INTERNAL_ERROR" {
		t.Fatalf("code = %q, want an erase that cannot report its pins refused", domain.AsAppError(err).Code)
	}
	if _, ok := repo.byID["prx_one"]; !ok {
		t.Fatal("the candidate went with an erase that could not describe it")
	}
}

// rehydratedProxy builds one stored candidate without routing through Create, so a test can
// stage a pool row whose id is known.
func rehydratedProxy(t *testing.T, id string) domain.Proxy {
	t.Helper()
	proxy, err := domain.NewProxy(id, "pool", domain.ProxyProtocolHTTP, "proxy.example.com", 3128,
		"operator", "sealed", testNow)
	if err != nil {
		t.Fatalf("building the candidate: %v", err)
	}
	return proxy
}
