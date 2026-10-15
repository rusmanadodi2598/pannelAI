// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/failure_signature_registry_test.go
// @for       The registry as a table an operator edits: what a refusal nobody seeded still answers, the scope that keeps one vendor's wording off another, and the entry that overrides a seeded one.
// @uses      net/http, regexp, testing, internal/domain.
// @reason    Seeding cannot cover every vendor, and a deployment changes its wording without a code release. The fallback and the scope line are what keep an unseeded upstream on the behaviour it had and a registered entry from claiming a refusal it never sent, so both are pinned apart from the seeded fixtures.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestClassify_GenericResetReachesAnUnseededVendor keeps C2 from being only about
// the vendors somebody thought to seed: a stated date is read whoever sent it.
func TestClassify_GenericResetReachesAnUnseededVendor(t *testing.T) {
	got := Classify(Failure{
		Provider: "unseeded", Status: http.StatusTooManyRequests,
		Body: []byte(`weekly limit reached, quota resets at 2026-10-19T10:48:00Z`),
	})
	if got.ResetAt.IsZero() {
		t.Fatal("reset = zero, want the date the body stated")
	}
	if got.ResetSource != domain.ResetSourceBody {
		t.Fatalf("reset source = %q, want %q", got.ResetSource, domain.ResetSourceBody)
	}
	if got.Class != domain.FailureRateLimitTemporary {
		t.Fatalf("class = %s, want the status's own answer: an unseeded body must not change the class", got.Class)
	}
}

// TestClassify_AScopedSignatureDoesNotClaimAnotherUpstream guards the scope: a
// signature scoped to one host or provider must not claim another's refusal, even
// when the wording is identical.
func TestClassify_AScopedSignatureDoesNotClaimAnotherUpstream(t *testing.T) {
	t.Cleanup(ResetSignatures)
	RegisterSignature(Signature{
		ID:       "one-vendor.only",
		Host:     "tokenharbor.ai",
		Provider: "th-harbor",
		Status:   http.StatusBadGateway,
		Pattern:  regexp.MustCompile(`nope`),
		Class:    domain.FailureGateBlocked,
	})
	body := []byte(`nope`)
	if got := Classify(Failure{Provider: "th-harbor", Host: "tokenharbor.ai", Status: http.StatusBadGateway, Body: body}); got.SignatureID != "one-vendor.only" {
		t.Fatalf("signature = %q, want the scoped entry to claim its own vendor", got.SignatureID)
	}
	if got := Classify(Failure{Provider: "th-harbor", Host: "example.com", Status: http.StatusBadGateway, Body: body}); got.SignatureID != "" {
		t.Fatalf("signature = %q, want no match on another host", got.SignatureID)
	}
	if got := Classify(Failure{Provider: "someone", Host: "tokenharbor.ai", Status: http.StatusBadGateway, Body: body}); got.SignatureID != "" {
		t.Fatalf("signature = %q, want no match for another provider", got.SignatureID)
	}
}

// TestClassify_ARegisteredSignatureOverridesASeededOne pins the override path a
// deployment uses when a vendor changes its wording: the newest entry wins.
func TestClassify_ARegisteredSignatureOverridesASeededOne(t *testing.T) {
	t.Cleanup(ResetSignatures)
	RegisterSignature(Signature{
		ID:       "operator.throttle",
		Provider: "th-harbor",
		Status:   http.StatusTooManyRequests,
		Pattern:  regexp.MustCompile(`under heavy load`),
		Class:    domain.FailureServerTransient,
	})
	got := Classify(Failure{
		Provider: "th-harbor", Host: "tokenharbor.ai", Status: http.StatusTooManyRequests,
		Body: []byte(thHeavyLoad),
	})
	if got.SignatureID != "operator.throttle" || got.Class != domain.FailureServerTransient {
		t.Fatalf("Classify() = %s/%s, want the registered override", got.SignatureID, got.Class)
	}
}
