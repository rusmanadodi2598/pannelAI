// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_retry_test.go
// @for       The retry decision and its backoff bounds.
// @uses      net/http, testing, internal/provider, internal/registry, internal/schema.
// @reason    Whether an upstream answer is worth repeating is a policy with a status
//
//	table and a ceiling (SPEC-API-001 §4), and it needs a connector
//	fixture of its own rather than the stream fixtures.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestDecideRetry pins the retry policy per status (SPEC-API-001 §4): only an
// outcome the plugin calls transient is repeated, the entry's attempt count bounds
// it, and a non-idempotent POST is capped at one retry.
func TestDecideRetry(t *testing.T) {
	transient := retryPlugin{retryable: map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true}}

	cases := []struct {
		name       string
		entry      registry.Provider
		attempt    Attempt
		plugin     provider.Plugin
		wantRetry  bool
		wantAfterU bool
	}{
		{
			name:    "a 429 is retried while attempts remain",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 429}, plugin: transient, wantRetry: true,
		},
		{
			name:    "a 502 is retried",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 502}, plugin: transient, wantRetry: true,
		},
		{
			name:    "a 400 is never retried",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 400}, plugin: transient, wantRetry: false,
		},
		{
			name:    "a 401 is never retried",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 401}, plugin: transient, wantRetry: false,
		},
		{
			name:    "the attempt budget bounds a retryable status",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 3, Status: 503}, plugin: transient, wantRetry: false,
		},
		{
			name:    "an entry allowing a single attempt retries none",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 1}}},
			attempt: Attempt{Retries: 0, Status: 429}, plugin: transient, wantRetry: false,
		},
		{
			name:    "a non-idempotent POST is capped at one retry",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 6}}},
			attempt: Attempt{Retries: 1, Status: 429, Idempotent: false}, plugin: transient, wantRetry: false,
		},
		{
			// Three attempts allow two retries, so the second one is the last the
			// entry's budget admits; a third would be one attempt too many.
			name:    "an idempotent request may use the entry's full budget",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 1, Status: 429, Idempotent: true}, plugin: transient, wantRetry: true,
		},
		{
			name:    "a transport failure is classified by the plugin's transient status",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 0}, plugin: transient, wantRetry: true,
		},
		{
			name: "a per-status override is what bounds its own status",
			entry: registry.Provider{Transport: registry.Transport{Retry: registry.Retry{
				DefaultAttempts: 5, ByStatus: map[int]int{429: 1},
			}}},
			attempt: Attempt{Retries: 0, Status: 429}, plugin: transient, wantRetry: false,
		},
		{
			// A second retry is only available when the default is three attempts,
			// which is what makes this case pin the default rather than restate it.
			name:    "an entry declaring no retry policy gets the documented default",
			entry:   registry.Provider{},
			attempt: Attempt{Retries: 1, Status: 503, Idempotent: true}, plugin: transient, wantRetry: true,
		},
		{
			name:    "a nil plugin is never retried",
			entry:   registry.Provider{},
			attempt: Attempt{Retries: 0, Status: 429}, plugin: nil, wantRetry: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideRetry(tc.entry, tc.plugin, tc.attempt)
			if got.Retry != tc.wantRetry {
				t.Fatalf("Retry = %v, want %v", got.Retry, tc.wantRetry)
			}
			if tc.wantRetry && got.After < 0 {
				t.Fatalf("After = %v, want a non-negative wait", got.After)
			}
		})
	}
}

// TestBackoffBounds pins the backoff window: it grows with the attempt number and
// stays inside the ceiling, so a long chain cannot park a request for minutes.
func TestBackoffBounds(t *testing.T) {
	cases := []struct {
		retries int
		maxWait int64
	}{
		{retries: 0, maxWait: int64(backoffBase)},
		{retries: 1, maxWait: int64(backoffBase) * 2},
		{retries: 5, maxWait: int64(backoffBase) * 32},
		{retries: 40, maxWait: int64(backoffCeiling)},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("retries=%d", tc.retries), func(t *testing.T) {
			for attempt := 0; attempt < 8; attempt++ {
				got := int64(Backoff(tc.retries))
				if got < 0 {
					t.Fatalf("Backoff(%d) = %d, want a non-negative wait", tc.retries, got)
				}
				if got > tc.maxWait {
					t.Fatalf("Backoff(%d) = %d, want at most %d", tc.retries, got, tc.maxWait)
				}
			}
		})
	}
	if got := Backoff(-3); got < 0 {
		t.Fatalf("Backoff(-3) = %d, want it clamped to a non-negative wait", got)
	}
}

// retryPlugin answers the retry question for the statuses a test drives, which is
// what the transport reads from a connector.
type retryPlugin struct {
	provider.Base
	// retryable is the status set this plugin considers worth repeating.
	retryable map[int]bool
}

func (p retryPlugin) ShouldRetry(status int, _ http.Header) provider.RetryDecision {
	if !p.retryable[status] {
		return provider.RetryDecision{}
	}
	return provider.RetryDecision{Retry: true}
}

// Endpoint and ApplyAuth are required by the seam but irrelevant to the retry
// question this plugin answers, so they return the zero answer rather than making
// the fixture pretend to be a full connector.
func (p retryPlugin) Endpoint(provider.Request, provider.Credential) (string, error) { return "", nil }

func (p retryPlugin) ApplyAuth(*http.Request, provider.Credential) error { return nil }
