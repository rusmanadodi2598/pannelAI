// Package handler serves the console and operations HTTP endpoints.
//
// @file      internal/handler/endpoint_key_provenance_test.go
// @for       The parked key's own story on the wire: which failure it was, how long its rest was, and which statement warranted that wait.
// @uses      encoding/json, testing, time, internal/domain, internal/schema.
// @reason    The two diagnostics exist so an operator can tell a park the vendor promised from one the gateway guessed. Read only by the health write and dropped by the render, they would answer nothing on the endpoint page that motivated migration 000021.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-15
package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestToKeyResponse_CarriesTheParksProvenance is the operator's question answered
// from a row: what was the failure, and did the vendor say the wait or did we.
func TestToKeyResponse_CarriesTheParksProvenance(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	key, err := domain.NewUpstreamKey("primary", "ep-1", "uky-1", "sealed-ciphertext", "sk-…last", 1, now)
	if err != nil {
		t.Fatalf("building the key: %v", err)
	}
	reset := now.Add(48 * time.Hour)
	policy := domain.DefaultKeyHealthPolicy().WithRest(domain.RestDirective{
		Class:       domain.FailureQuotaExhausted,
		ResetAt:     reset,
		Source:      domain.ResetSourceBody,
		SignatureID: "tokenharbor.free-allowance",
	})
	key.RecordFailure("allowance spent", domain.KeyFailureQuota, policy, now)

	resp := toKeyResponse(key, now)
	if resp.LastErrorClass != string(domain.FailureQuotaExhausted) {
		t.Fatalf("last_error_class = %q, want the recognised class", resp.LastErrorClass)
	}
	if resp.ResetSource != string(domain.ResetSourceBody) {
		t.Fatalf("reset_source = %q, want body", resp.ResetSource)
	}
	if resp.SignatureID != "tokenharbor.free-allowance" {
		t.Fatalf("signature_id = %q, want the signature that recognised it", resp.SignatureID)
	}
	if resp.RateLimitedUntil == nil {
		t.Fatal("the park itself is absent, so the provenance describes nothing")
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"reset_source":"body"`) {
		t.Fatalf("the wire dropped the provenance: %s", encoded)
	}
}

// TestToKeyResponse_AKeyThatNeverFailedNamesNothing is the absent half: a healthy
// key carries no class, no source and no signature, rather than an empty string an
// operator would have to interpret as "classified as nothing".
func TestToKeyResponse_AKeyThatNeverFailedNamesNothing(t *testing.T) {
	now := time.Now().UTC()
	key, err := domain.NewUpstreamKey("primary", "ep-1", "uky-1", "sealed-ciphertext", "sk-…last", 1, now)
	if err != nil {
		t.Fatalf("building the key: %v", err)
	}
	encoded, err := json.Marshal(toKeyResponse(key, now))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, absent := range []string{"last_error_class", "reset_source", "signature_id"} {
		if strings.Contains(string(encoded), absent) {
			t.Fatalf("a key that never failed carries %q: %s", absent, encoded)
		}
	}

	key.RecordFailure("throttled", domain.KeyFailureRateLimit, domain.DefaultKeyHealthPolicy(), now)
	if resp := toKeyResponse(key, now); resp.ResetSource != string(domain.ResetSourceDefault) {
		t.Fatalf("reset_source = %q, want a wait the gateway chose named as default", resp.ResetSource)
	}

	key.RecordSuccess(now)
	if resp := toKeyResponse(key, now); resp.LastErrorClass != "" || resp.ResetSource != "" || resp.SignatureID != "" {
		t.Fatalf("a served call left %+v behind, want the whole classification cleared", resp)
	}
}
