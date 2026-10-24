// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_probe_policy_test.go
// @for       Which connectivity test failures park a key, and the window each applies, resolved from the routing group's configuration for that provider.
// @uses      context, errors, testing, time, internal/domain.
// @reason    Draft 050 T8 moved the §7.5 windows from constants to settings, and the probe is the second writer of a parked key. If it read the windows from anywhere else, the Test button and a live request would park the same credential for different lengths, which is the disagreement §7.5 exists to prevent.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-13
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubKeyHealthPolicy answers a configured budget per provider and records that it
// was asked, so a probe that never consulted the settings is visible as a failure.
type stubKeyHealthPolicy struct {
	policies map[string]domain.KeyHealthPolicy
	err      error
	asked    []string
}

func (s *stubKeyHealthPolicy) KeyHealthPolicy(_ context.Context, providerID string) (domain.KeyHealthPolicy, error) {
	s.asked = append(s.asked, providerID)
	if s.err != nil {
		return domain.KeyHealthPolicy{}, s.err
	}
	return s.policies[providerID], nil
}

// probedKey parks one key with a refused credential and returns what the store was
// handed, so the window is read from the write the router will later see.
func probedKey(t *testing.T, svc *EndpointService, providerID string) domain.UpstreamKey {
	t.Helper()
	endpoint := keyedEndpointWith(t, svc, providerID, "acct", "primary")
	svc.prober = &fakeProber{outcome: ProbeOutcome{
		State: string(domain.EndpointTestFail), Status: 401, Message: "credential rejected",
	}}
	if _, _, err := svc.Test(context.Background(), endpoint.ID(), ""); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	reloaded, err := svc.Get(context.Background(), endpoint.ID())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	return reloaded.Keys()[0]
}

// TestEndpointService_TestParksWithTheConfiguredWindow is the finding answered on
// the panel's own path: the provider's window, not the compiled-in one.
func TestEndpointService_TestParksWithTheConfiguredWindow(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	health := &stubKeyHealthPolicy{policies: map[string]domain.KeyHealthPolicy{
		"deepseek": {AuthWindow: 45 * time.Second},
	}}
	svc.health = health

	key := probedKey(t, svc, "deepseek")
	if key.RateLimitedUntil() == nil {
		t.Fatal("a refused credential was not parked")
	}
	if got := key.RateLimitedUntil().Sub(testNow); got != 45*time.Second {
		t.Fatalf("park window = %v, want the configured 45s", got)
	}
	if len(health.asked) == 0 || health.asked[0] != "deepseek" {
		t.Fatalf("policy reads = %v, want one for deepseek", health.asked)
	}
}

// TestEndpointService_TestWithoutAnUpstreamAnswerParksNothing covers the probe paths
// that send nothing: no validation surface declared, a host the egress guard refused,
// a call that never completed. They carry no status, and a call the credential never
// saw says nothing about it, which is what routing already answers for the same fact.
func TestEndpointService_TestWithoutAnUpstreamAnswerParksNothing(t *testing.T) {
	for _, message := range []string{
		"this provider does not declare a validation endpoint",
		"the upstream could not be reached",
		"the account identity could not be read",
	} {
		t.Run(message, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			endpoint := keyedEndpointWith(t, svc, "deepseek", "acct", "primary")
			svc.prober = &fakeProber{outcome: ProbeOutcome{
				State: string(domain.EndpointTestFail), Message: message,
			}}
			if _, _, err := svc.Test(context.Background(), endpoint.ID(), ""); err != nil {
				t.Fatalf("Test() error = %v", err)
			}

			reloaded, err := svc.Get(context.Background(), endpoint.ID())
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			key := reloaded.Keys()[0]
			if key.RateLimitedUntil() != nil {
				t.Fatalf("park = %v, want nothing parked for a request that was never sent",
					key.RateLimitedUntil())
			}
			if !key.Available(testNow) {
				t.Fatal("the key left rotation on a probe that sent no request")
			}
			if strikes := key.ConsecutiveErrors(); strikes != 0 {
				t.Fatalf("consecutive errors = %d, want no strike recorded", strikes)
			}
			if reloaded.TestStatus().State() != domain.EndpointTestFail {
				t.Fatal("the test answer itself was not recorded on the endpoint")
			}
		})
	}
}

// TestEndpointService_TestParksAtTheDocumentedWindowWhenTheReadFails keeps the
// probe useful with an unreadable settings row: the strike is recorded at the length
// the spec documents rather than not at all.
func TestEndpointService_TestParksAtTheDocumentedWindowWhenTheReadFails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reader *stubKeyHealthPolicy
	}{
		{name: "no reader wired"},
		{name: "the reader fails", reader: &stubKeyHealthPolicy{err: errors.New("settings row unreadable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			if tc.reader != nil {
				svc.health = tc.reader
			}

			key := probedKey(t, svc, "deepseek")
			if key.RateLimitedUntil() == nil {
				t.Fatal("a refused credential was not parked")
			}
			if got := key.RateLimitedUntil().Sub(testNow); got != 2*time.Minute {
				t.Fatalf("park window = %v, want the documented 2m", got)
			}
		})
	}
}
