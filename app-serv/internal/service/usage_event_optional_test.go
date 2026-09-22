// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_optional_test.go
// @for       Tests that the usage recorder treats its broker as additive.
//
// @uses      internal/domain, context, testing.
// @reason    The publisher is optional in UsageServiceDeps on purpose: a
//
//	deployment that wired no broker has to keep recording rows rather
//	than refuse to boot. That property is invisible to every other test
//	in this package, because they all wire a publisher, so it is pinned
//	on its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"testing"
)

// TestUsageService_RecordWorksWithoutABroker asserts the recorder is not
// conditional on a broker: a deployment that wired none still records.
func TestUsageService_RecordWorksWithoutABroker(t *testing.T) {
	repo := &recordingUsageRepo{}
	settings, err := NewSettingsService(SettingsServiceDeps{Repo: newStubSettingsStore()})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	svc, err := NewUsageService(UsageServiceDeps{Usage: repo, Settings: settings})
	if err != nil {
		t.Fatalf("usage service: %v", err)
	}
	if _, err := svc.Record(context.Background(), usageRecordInputFixture()); err != nil {
		t.Fatalf("Record() without a broker = %v, want nil", err)
	}
	if repo.count() != 1 {
		t.Fatalf("stored rows = %d, want 1", repo.count())
	}
}
