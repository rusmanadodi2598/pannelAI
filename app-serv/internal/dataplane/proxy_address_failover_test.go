// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_address_failover_test.go
// @for       The walk's reading of an upstream refusal that belongs to the address rather than to the request.
// @uses      internal/domain, context, io, net/http, net/http/httptest, strings, testing, time.
// @reason    Measured against the live OpenCode free lane on 2026-10-15: bursting one egress address produced 429 while a second address, same session, same model, unanswered in that burst, served three requests in a row, and the first was still refusing eleven seconds later. The quota bucket is therefore the address, so a refusal worth another candidate must be decided by the walk rather than handed to the client while capacity sits unused. These pins hold that decision, the cooldown it writes, and the refusals that must NOT move on.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-15
package dataplane

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// refusedThenServed builds the two-candidate walk the measurement produced: the
// first address answers the upstream's own status, the second answers properly.
func refusedThenServed(t *testing.T, refuse int, header map[string]string) (*ProxyDialer, *fakePlanner, *int) {
	t.Helper()
	var servedHits int
	served := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		servedHits++
		raw, _ := io.ReadAll(r.Body)
		if len(raw) == 0 {
			t.Errorf("the served attempt received an empty body; the walk must replay it")
		}
		_, _ = io.WriteString(w, "served")
	}))
	t.Cleanup(served.Close)

	refused := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for name, value := range header {
			w.Header().Set(name, value)
		}
		w.WriteHeader(refuse)
		_, _ = io.WriteString(w, "refused-by-this-address")
	}))
	t.Cleanup(refused.Close)

	planner := &fakePlanner{plans: [][]domain.ProxyRouteAttempt{
		{attemptFor("prx_refused", refused.URL), attemptFor("prx_served", served.URL)},
	}}
	return &ProxyDialer{client: served.Client(), routes: planner}, planner, &servedHits
}

func postToDialer(t *testing.T, dialer *ProxyDialer) (*http.Response, error) {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://destination.example/x", strings.NewReader("the-body"))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	return dialer.Do(context.Background(), request, "opencode")
}

func TestProxyDialer_RateLimitedAddressIsNotTheAnswer(t *testing.T) {
	dialer, planner, servedHits := refusedThenServed(t, http.StatusTooManyRequests, nil)

	response, err := postToDialer(t, dialer)
	if err != nil {
		t.Fatalf("Do() error = %v, want the next address to serve", err)
	}
	raw, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()

	if response.StatusCode != http.StatusOK || string(raw) != "served" {
		t.Fatalf("walk returned %d %q, want the second address's answer", response.StatusCode, raw)
	}
	if *servedHits != 1 {
		t.Fatalf("the serving address was dialed %d times, want once", *servedHits)
	}
	if len(planner.parks) != 1 || planner.parks[0].id != "prx_refused" {
		t.Fatalf("parked candidates = %v, want only the refusing address", planner.parks)
	}
}

func TestProxyDialer_RefusedAddressSitsOutTheVendorsOwnWait(t *testing.T) {
	dialer, planner, _ := refusedThenServed(t, http.StatusTooManyRequests,
		map[string]string{"Retry-After": "30"})

	response, err := postToDialer(t, dialer)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = response.Body.Close()

	if len(planner.parks) != 1 {
		t.Fatalf("parked candidates = %v, want the refusing address once", planner.parks)
	}
	if planner.parks[0].ttl != 30*time.Second {
		t.Fatalf("parked for %v, want the vendor's own 30s", planner.parks[0].ttl)
	}
}

func TestProxyDialer_VendorsWaitIsClampedToTheProxyCooldown(t *testing.T) {
	dialer, planner, _ := refusedThenServed(t, http.StatusTooManyRequests,
		map[string]string{"Retry-After": "900"})

	response, err := postToDialer(t, dialer)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = response.Body.Close()

	if len(planner.parks) != 1 || planner.parks[0].ttl != domain.ProxyFailureCooldown {
		t.Fatalf("parked %v, want the cooldown the walk stops at rather than the vendor's 900s", planner.parks)
	}
}

func TestProxyDialer_ARefusalThatIsNotAboutTheAddressIsNotRepeated(t *testing.T) {
	dialer, planner, servedHits := refusedThenServed(t, http.StatusInternalServerError, nil)

	response, err := postToDialer(t, dialer)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	raw, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()

	if response.StatusCode != http.StatusInternalServerError || string(raw) != "refused-by-this-address" {
		t.Fatalf("walk returned %d %q, want the upstream's answer handed back unchanged", response.StatusCode, raw)
	}
	if *servedHits != 0 {
		t.Fatalf("the second address was dialed for a failure that is not about an address")
	}
	if len(planner.parks) != 0 {
		t.Fatalf("parked candidates = %v, want nothing: this address told the truth about the request", planner.parks)
	}
}

func TestProxyDialer_WhenEveryAddressRefusesTheClientGetsTheRefusal(t *testing.T) {
	var secondHits int
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, "first-refused")
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondHits++
		w.Header().Set("Retry-After", "12")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, "second-refused")
	}))
	defer second.Close()

	planner := &fakePlanner{plans: [][]domain.ProxyRouteAttempt{
		{attemptFor("prx_1", first.URL), attemptFor("prx_2", second.URL)},
	}}
	dialer := &ProxyDialer{client: second.Client(), routes: planner}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://destination.example/x", strings.NewReader("the-body"))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}

	response, err := dialer.Do(context.Background(), request, "opencode")
	if err != nil {
		t.Fatalf("Do() error = %v, want the upstream's refusal rather than a gateway invention", err)
	}
	raw, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()

	if response.StatusCode != http.StatusTooManyRequests || string(raw) != "second-refused" {
		t.Fatalf("returned %d %q, want the last address's refusal intact", response.StatusCode, raw)
	}
	if secondHits != 1 {
		t.Fatalf("second address dialed %d times, want once", secondHits)
	}
	if len(planner.parks) != 2 {
		t.Fatalf("parked candidates = %v, want both refusing addresses", planner.parks)
	}
	if planner.parks[1].ttl != 12*time.Second {
		t.Fatalf("the last address parked for %v, want its own Retry-After", planner.parks[1].ttl)
	}
}

func TestProxyDialer_ABodyThatCannotBeReplayedIsNotResent(t *testing.T) {
	dialer, planner, servedHits := refusedThenServed(t, http.StatusTooManyRequests, nil)

	// A pipe has no GetBody, so a second attempt could not rebuild the payload.
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://destination.example/x", nonReplayable{strings.NewReader("the-body")})
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	response, err := dialer.Do(context.Background(), request, "opencode")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	raw, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()

	if *servedHits != 0 {
		t.Fatalf("the second address was dialed although the body cannot be rebuilt")
	}
	if string(raw) != "refused-by-this-address" {
		t.Fatalf("returned %q, want the refusal the client can still be told about", raw)
	}
	if len(planner.parks) != 1 {
		t.Fatalf("parked candidates = %v, want the refusing address skipped by later requests", planner.parks)
	}
}

// nonReplayable carries a payload the request cannot rebuild, which is what a
// streamed client body looks like to net/http: no GetBody, so a second attempt
// over another address is impossible rather than merely unwanted.
type nonReplayable struct {
	io.Reader
}
