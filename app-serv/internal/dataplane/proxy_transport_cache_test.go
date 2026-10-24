// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_transport_cache_test.go
// @for       That the per-candidate proxy client cache is bounded, evicts the oldest entry, and reuses a live one.
// @uses      net/http, net/url, sync, testing.
// @reason    A proxy pool can be edited while the gateway runs, and an unbounded cache would keep every removed candidate's transport and idle sockets for the process lifetime. The cap is the property a rotation test can pin without a real proxy.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package dataplane

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
)

// proxyCandidate parses one candidate address the way the planner supplies it.
func proxyCandidate(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing %q: %v", raw, err)
	}
	return parsed
}

// TestProxyDialer_TransportCacheIsBounded pins reuse and FIFO eviction: the
// same candidate returns the same client, and once the cap is crossed the
// oldest transport is gone.
func TestProxyDialer_TransportCacheIsBounded(t *testing.T) {
	dialer := &ProxyDialer{client: &http.Client{Transport: &http.Transport{}}}

	first := proxyCandidate(t, "http://proxy-0.example.com:8080")
	clientA, err := dialer.clientFor(first)
	if err != nil {
		t.Fatalf("first clientFor error = %v", err)
	}
	clientB, err := dialer.clientFor(first)
	if err != nil {
		t.Fatalf("second clientFor error = %v", err)
	}
	if clientA != clientB {
		t.Fatal("clientFor rebuilt a cached candidate instead of reusing it")
	}

	for index := 1; index < maxProxyTransports+8; index++ {
		candidate := proxyCandidate(t, fmt.Sprintf("http://proxy-%d.example.com:8080", index))
		if _, err := dialer.clientFor(candidate); err != nil {
			t.Fatalf("clientFor(%d) error = %v", index, err)
		}
	}

	if len(dialer.transports) != maxProxyTransports {
		t.Fatalf("cached transports = %d, want %d", len(dialer.transports), maxProxyTransports)
	}
	if _, present := dialer.transports[first.String()]; present {
		t.Fatal("the oldest candidate survived the cap")
	}
	newest := fmt.Sprintf("http://proxy-%d.example.com:8080", maxProxyTransports+7)
	if _, present := dialer.transports[newest]; !present {
		t.Fatalf("the newest candidate %q was evicted", newest)
	}
}

// TestProxyDialer_TransportCacheSurvivesConcurrentMisses pins the lock around
// insertion: parallel first uses of distinct candidates stay bounded and no
// duplicate order entry outlives the cap.
func TestProxyDialer_TransportCacheSurvivesConcurrentMisses(t *testing.T) {
	dialer := &ProxyDialer{client: &http.Client{Transport: &http.Transport{}}}
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for index := 0; index < maxProxyTransports+8; index++ {
				candidate := &url.URL{
					Scheme: "http",
					Host:   fmt.Sprintf("worker-%d-proxy-%d.example.com:8080", worker, index),
				}
				if _, err := dialer.clientFor(candidate); err != nil {
					t.Errorf("clientFor(%d/%d) error = %v", worker, index, err)
					return
				}
			}
		}(worker)
	}
	wait.Wait()

	if len(dialer.transports) != maxProxyTransports {
		t.Fatalf("cached transports = %d, want %d", len(dialer.transports), maxProxyTransports)
	}
	if len(dialer.transportOrder) != maxProxyTransports {
		t.Fatalf("order entries = %d, want %d", len(dialer.transportOrder), maxProxyTransports)
	}
}
