// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_account_health_test.go
// @for       Tests for the account's own circuit: which endpoint is its own credential, what a failure parks, and what releases it.
// @uses      time, testing.
// @reason    The boundary is the load-bearing part. Park the wrong unit and a vendor 429 on one key withholds a whole account from rotation (draft 050 T1); park no unit and a dead OAuth token is selected forever. Both failures are silent in traffic, so both need a test that fails loudly.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-14
package domain

import (
	"testing"
	"time"
)

func accountEndpoint(t *testing.T, authType UpstreamAuthType, keyCount int) UpstreamEndpoint {
	t.Helper()
	endpoint, err := NewUpstreamEndpoint("ep_1", "alpha", "account", authType, 1, keyNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	for i := range keyCount {
		endpoint.AttachKey(stampableKey("uky_"+string(rune('a'+i)), 1, nil, nil))
	}
	return endpoint
}

func TestUpstreamEndpoint_ServesItsOwnCredential(t *testing.T) {
	cases := []struct {
		name     string
		authType UpstreamAuthType
		keys     int
		want     bool
	}{
		{name: "an oauth account is its own credential", authType: UpstreamAuthOAuth, want: true},
		{name: "a public lane has no credential to park", authType: UpstreamAuthNone, want: true},
		{name: "a service account keeps its credential in a key row",
			authType: UpstreamAuthServiceAccount, keys: 1, want: false},
		{name: "an api-key account parks the key instead", authType: UpstreamAuthAPIKey, keys: 2, want: false},
		{name: "an api-key account with no key is still not one credential",
			authType: UpstreamAuthAPIKey, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := accountEndpoint(t, tc.authType, tc.keys)
			if got := endpoint.ServesItsOwnCredential(); got != tc.want {
				t.Fatalf("ServesItsOwnCredential() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestUpstreamEndpoint_RecordAccountFailure walks the ladder the account steps on,
// including the strike count that keeps a throttled account's credential window
// where it belongs.
func TestUpstreamEndpoint_RecordAccountFailure(t *testing.T) {
	cases := []struct {
		name   string
		class  KeyFailureClass
		repeat int
		want   time.Duration
	}{
		{name: "first transient is flat", class: KeyFailureTransient, repeat: 1, want: 30 * time.Second},
		{name: "the third rate limit doubles", class: KeyFailureRateLimit, repeat: 3, want: 8 * time.Second},
		{name: "the second auth strike doubles", class: KeyFailureAuth, repeat: 2, want: 4 * time.Minute},
		{name: "the second billing strike doubles", class: KeyFailureBilling, repeat: 2, want: 30 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := accountEndpoint(t, UpstreamAuthOAuth, 0)
			for range tc.repeat {
				if !endpoint.RecordAccountFailure(tc.class, DefaultKeyHealthPolicy(), keyNow) {
					t.Fatalf("strike %d parked nothing, want a credential-shaped failure to park", tc.repeat)
				}
			}
			until := endpoint.RateLimitedUntil()
			if until == nil || !until.Equal(keyNow.Add(tc.want)) {
				t.Fatalf("park = %v, want %v", until, keyNow.Add(tc.want))
			}
			if got := endpoint.AccountConsecutiveErrors(); got != tc.repeat {
				t.Fatalf("account run = %d, want %d", got, tc.repeat)
			}
		})
	}
}

// TestUpstreamEndpoint_AccountFailureParksNothingForItsOwnClasses pins that a
// failure the credential caused leaves the account alone, which is what keeps a
// dead DNS lookup from burying a working account.
func TestUpstreamEndpoint_AccountFailureParksNothingForItsOwnClasses(t *testing.T) {
	for _, class := range []KeyFailureClass{KeyFailureRequest, KeyFailureTarget} {
		endpoint := accountEndpoint(t, UpstreamAuthOAuth, 0)
		if endpoint.RecordAccountFailure(class, DefaultKeyHealthPolicy(), keyNow) {
			t.Fatalf("class %v parked the account, want nothing parked", class)
		}
		if endpoint.RateLimitedUntil() != nil {
			t.Fatalf("class %v set a deadline, want none", class)
		}
		if got := endpoint.AccountConsecutiveErrors(); got != 0 {
			t.Fatalf("class %v run = %d, want 0", class, got)
		}
	}
}

// TestUpstreamEndpoint_AvailableRefusesAParkedAccount is the reason the park matters
// at all: the selector reads Available, so a parked account leaves rotation without
// a sweeper and returns the moment the deadline passes.
func TestUpstreamEndpoint_AvailableRefusesAParkedAccount(t *testing.T) {
	endpoint := accountEndpoint(t, UpstreamAuthOAuth, 0)
	if !endpoint.RecordAccountFailure(KeyFailureAuth, DefaultKeyHealthPolicy(), keyNow) {
		t.Fatal("the auth strike parked nothing")
	}
	if endpoint.Available(keyNow) {
		t.Fatal("Available() = true inside the park window")
	}
	if !endpoint.Available(keyNow.Add(2*time.Minute + time.Second)) {
		t.Fatal("Available() = false after the window expired, want the deadline to be the whole state")
	}

	endpoint.RecordAccountSuccess(keyNow.Add(3 * time.Minute))
	if !endpoint.Available(keyNow.Add(3 * time.Minute)) {
		t.Fatal("a served call must release the park immediately")
	}
	if got := endpoint.AccountConsecutiveErrors(); got != 0 {
		t.Fatalf("account run after success = %d, want 0", got)
	}
}

// TestUpstreamEndpoint_RecordUpstreamSuccessReleasesThePark pins the wiring the
// parity path owns: clearing the recorded error and clearing the circuit are one
// event, so a recovered account cannot keep a countdown nobody can explain.
func TestUpstreamEndpoint_RecordUpstreamSuccessReleasesThePark(t *testing.T) {
	endpoint := accountEndpoint(t, UpstreamAuthOAuth, 0)
	endpoint.RecordAccountFailure(KeyFailureAuth, DefaultKeyHealthPolicy(), keyNow)
	endpoint.RecordUpstreamSuccess(keyNow)

	if endpoint.RateLimitedUntil() != nil {
		t.Fatalf("park = %v, want released by the success", endpoint.RateLimitedUntil())
	}
	if got := endpoint.AccountConsecutiveErrors(); got != 0 {
		t.Fatalf("account run = %d, want 0", got)
	}
}
