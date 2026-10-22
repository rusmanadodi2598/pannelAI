//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_retention_integration_test.go
// @for       The batched usage retention delete.
// @uses      context, fmt, testing, time, internal/domain.
// @reason    The cutoff and the batch loop are the only guarantees that billing history stays bounded, so both are exercised against a real table rather than a fake that can only echo the call.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-22
package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func seedUsageAt(t *testing.T, repo *UsageRepository, requestID string, ts time.Time) {
	t.Helper()
	record, err := domain.NewUsageRecord(domain.UsageRecordInput{
		RequestID: requestID, TS: ts, ProviderID: "provider", Model: "model",
		Status: domain.UsageStatusSuccess, TokensIn: 1, CostUSD: "0.01", LatencyMS: 1,
	}, "", time.Now())
	if err != nil {
		t.Fatalf("building usage %s: %v", requestID, err)
	}
	if err := repo.Record(context.Background(), record); err != nil {
		t.Fatalf("recording usage %s: %v", requestID, err)
	}
}

func TestUsageRepository_DeleteOlderThanKeepsTheWindow(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	previous := usageRetentionBatchSize
	usageRetentionBatchSize = 2
	t.Cleanup(func() { usageRetentionBatchSize = previous })

	for i := 0; i < 5; i++ {
		seedUsageAt(t, repo, fmt.Sprintf("usg_old_%d", i), now.Add(-time.Duration(i+1)*time.Hour))
	}
	seedUsageAt(t, repo, "usg_keep", now)

	deleted, err := repo.DeleteUsageOlderThan(ctx, now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("DeleteUsageOlderThan() error = %v", err)
	}
	if deleted != 5 {
		t.Fatalf("deleted = %d, want the five old rows", deleted)
	}
	if _, err := repo.GetByRequestID(ctx, "usg_keep"); err != nil {
		t.Fatalf("row inside the window was removed: %v", err)
	}
}
