//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_key_rest_integration_test.go
// @for       The park's provenance surviving the round trip: what the health write stored in the two diagnostic columns, and what a reload hands back.
// @uses      context, testing, time, internal/domain.
// @reason    Both columns are written by one UPDATE and read by a projection two files away, and the projection is a positional scan list. A fake repository cannot catch a column added to one and not the other, which is the exact way this pair breaks: the code compiles, the row scans wrong, and every key after it is unreadable.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-15
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestIntegration_RestProvenanceRoundTripsThroughTheStore parks a key on a reset
// the vendor stated, reloads it through the read path the selection uses, and then
// serves a call that must clear the whole classification.
func TestIntegration_RestProvenanceRoundTripsThroughTheStore(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	seedHealthKey(t, repo, 0, now)

	reset := now.Add(48 * time.Hour)
	rest := domain.RestDirective{
		Class:       domain.FailureQuotaExhausted,
		ResetAt:     reset,
		Source:      domain.ResetSourceBody,
		SignatureID: "tokenharbor.free-allowance",
	}
	loaded, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	parked, err := loaded.RecordKeyFailure(healthKeyID, "allowance spent",
		domain.KeyFailureQuota, domain.DefaultKeyHealthPolicy().WithRest(rest), now)
	if err != nil {
		t.Fatalf("recording a spent allowance: %v", err)
	}
	if err := repo.RecordKeyHealth(ctx, parked); err != nil {
		t.Fatalf("RecordKeyHealth() error = %v", err)
	}

	stored := readHealthKey(t, repo, ctx).Key(healthKeyID)
	if got := stored.LastErrorClass(); got != string(domain.FailureQuotaExhausted) {
		t.Fatalf("last_error_class = %q, want the recognised class", got)
	}
	if got := stored.ResetSource(); got != string(domain.ResetSourceBody) {
		t.Fatalf("reset_source = %q, want body: the vendor stated the date in its own text", got)
	}
	if got := stored.SignatureID(); got != "tokenharbor.free-allowance" {
		t.Fatalf("signature_id = %q, want the recogniser named", got)
	}
	until := stored.RateLimitedUntil()
	if until == nil || !until.Equal(reset) {
		t.Fatalf("parked until = %v, want the stated reset %v", until, reset)
	}

	after := readHealthKey(t, repo, ctx)
	served, err := after.RecordKeySuccess(healthKeyID, now)
	if err != nil {
		t.Fatalf("recording a served call: %v", err)
	}
	if err := repo.RecordKeyHealth(ctx, served); err != nil {
		t.Fatalf("persisting the served call: %v", err)
	}
	cleared := readHealthKey(t, repo, ctx).Key(healthKeyID)
	if cleared.ResetSource() != "" || cleared.SignatureID() != "" || cleared.LastErrorClass() != "" {
		t.Fatalf("a served call left %q/%q/%q behind, want all three NULL",
			cleared.LastErrorClass(), cleared.ResetSource(), cleared.SignatureID())
	}
	if cleared.RateLimitedUntil() != nil {
		t.Fatalf("park = %v, want the key back in rotation", cleared.RateLimitedUntil())
	}
}

// TestIntegration_RestProvenanceOfAFreshKeyIsNULL is the older row: a key that has
// never been parked, or one parked before the classifier existed, reads back as the
// empty string rather than failing the positional scan on a NULL.
func TestIntegration_RestProvenanceOfAFreshKeyIsNULL(t *testing.T) {
	repo := newEndpointRepo(t)
	now := time.Now().UTC().Truncate(time.Millisecond)

	neverFailed := seedHealthKey(t, repo, 0, now).Key(healthKeyID)
	if neverFailed.ResetSource() != "" || neverFailed.SignatureID() != "" {
		t.Fatalf("a fresh key read %q/%q, want both NULL", neverFailed.ResetSource(), neverFailed.SignatureID())
	}
}

// TestIntegration_RestProvenanceOfAnUnclassifiedPark is the half an operator reads
// most: a park the status alone produced carries the gateway's own answer, not a
// blank that would look like "unknown".
func TestIntegration_RestProvenanceOfAnUnclassifiedPark(t *testing.T) {
	repo := newEndpointRepo(t)
	now := time.Now().UTC().Truncate(time.Millisecond)

	parked := seedHealthKey(t, repo, 1, now).Key(healthKeyID)
	if got := parked.ResetSource(); got != string(domain.ResetSourceDefault) {
		t.Fatalf("reset_source = %q, want default: the ladder chose the wait, the vendor did not", got)
	}
	if got := parked.SignatureID(); got != "" {
		t.Fatalf("signature_id = %q, want no recogniser named for a park that fell back to the status", got)
	}
	if got := parked.LastErrorClass(); got != domain.KeyFailureAuth.String() {
		t.Fatalf("last_error_class = %q, want the parking class's own token %q", got, domain.KeyFailureAuth.String())
	}
}
