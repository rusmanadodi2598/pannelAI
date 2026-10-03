// Package quotafetch mirrors the reference's per-family quota readers.
//
// @file      internal/service/quotafetch/egress_client_test.go
// @for       That every quota read leaves through the egress client the
//
//	composition root installs, and that the shared client refuses a
//	redirect rather than following one.
//
// @uses      context, net/http, net/http/httptest, sync/atomic, testing.
// @reason    R08 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: the package's shared
//
//	client was built bare, so every family's read left without the
//	process egress guard and followed redirects a quota host could use
//	to bounce a credential cross-host. The client is package state,
//	so only a live request through it can prove the install and the
//	redirect refusal.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-03
package quotafetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// recordingTransport delegates to the default transport and counts the requests
// it carried, so a test can tell which client a read actually rode.
type recordingTransport struct {
	rides atomic.Int32
	base  http.RoundTripper
}

func (t *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.rides.Add(1)
	return t.base.RoundTrip(request)
}

// TestUseEgressClient_InstallsTheGuardedTransport pins that a read after the
// install rides the transport the composition root handed over, that the
// installed client keeps the redirect refusal the egress client carries, and
// that a nil or bare client is refused rather than silently dropping the guard.
func TestUseEgressClient_InstallsTheGuardedTransport(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(server.Close)

	rides := &recordingTransport{base: http.DefaultTransport}
	egress := &http.Client{Transport: rides, CheckRedirect: noRedirect}
	t.Cleanup(func() { client = &http.Client{Timeout: requestTimeout, CheckRedirect: noRedirect} })

	if err := UseEgressClient(nil); err == nil {
		t.Fatal("UseEgressClient(nil) = nil error, want a refusal")
	}
	bare := &http.Client{}
	if err := UseEgressClient(bare); err == nil {
		t.Fatal("UseEgressClient with a client carrying no transport = nil error, want a refusal")
	}
	if err := UseEgressClient(egress); err != nil {
		t.Fatalf("UseEgressClient() error = %v", err)
	}
	response, err := requestUsage(context.Background(), http.MethodGet, server.URL, nil, "")
	if err != nil {
		t.Fatalf("requestUsage() error = %v", err)
	}
	if response.status != http.StatusFound {
		t.Fatalf("status = %d, want the redirect surfaced rather than followed", response.status)
	}
	if rides.rides.Load() != 1 {
		t.Fatalf("the installed transport carried %d requests, want exactly the read", rides.rides.Load())
	}
	if hits := targetHits.Load(); hits != 0 {
		t.Fatalf("the redirect target served %d requests, want it never asked", hits)
	}
}

// TestDefaultClient_RefusesRedirects pins that a quota host cannot bounce a
// read cross-host: the 302 comes back to the family's failure policy, and the
// redirect target is never asked.
func TestDefaultClient_RefusesRedirects(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	response, err := requestUsage(context.Background(), http.MethodGet, redirector.URL, nil, "")
	if err != nil {
		t.Fatalf("requestUsage() error = %v", err)
	}
	if response.status != http.StatusFound {
		t.Fatalf("status = %d, want the redirect itself surfaced to the family", response.status)
	}
	if hits := targetHits.Load(); hits != 0 {
		t.Fatalf("the redirect target served %d requests, want it never asked", hits)
	}
}
