// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings_update_test.go
// @for       The §7.14 partial PATCH: per-group persistence, the corrupted-row
//
//	fallback, and the round-trip the panel depends on.
//
// @uses      internal/domain, reflect, testing.
//
// @reason    SPEC-API-001 §7.14 makes the PATCH the one door every gateway
//
//	setting changes through, and §2.1 requires service logic to be
//	proven beside its implementation: only the groups a patch touched
//	may be written (concurrent PATCHes must not clobber each other),
//	the deprecated caveman row must survive a token_saver write
//	untouched (§7.9), and a stored row that predates a key or does not
//	decode must fall back to the documented default rather than lock
//	the panel out of its own settings.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSettingsService_UpdateWritesOnlyTouchedGroups pins the per-row write
// rule: a patch naming one group persists exactly that group's row, so two
// concurrent PATCHes of different groups cannot overwrite each other.
func TestSettingsService_UpdateWritesOnlyTouchedGroups(t *testing.T) {
	repo := newStubSettingsStore()
	svc := newSettingsUpdateFixture(t, repo)

	next, err := svc.Update(context.Background(), domain.SettingsPatch{
		Logging: &domain.LoggingSettingsPatch{CaptureBodyMaxBytes: intPtr(4096)},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if len(repo.written) != 1 || repo.written[0] != domain.SettingsKeyLogging {
		t.Fatalf("written rows = %v, want only %q", repo.written, domain.SettingsKeyLogging)
	}
	if next.Logging.CaptureBodyMaxBytes != 4096 {
		t.Fatalf("capture_body_max_bytes = %d, want the patched 4096", next.Logging.CaptureBodyMaxBytes)
	}
	if next.Logging.RetentionDays != domain.DefaultSettings().Logging.RetentionDays {
		t.Fatalf("retention_days = %d, want the default left intact", next.Logging.RetentionDays)
	}
}

// TestSettingsService_UpdateLeavesTheCavemanRowUntouched pins §7.9's freeze
// rule across the §7.14 door: a token_saver patch rewrites the token_saver row
// and never the deprecated caveman row beside it.
func TestSettingsService_UpdateLeavesTheCavemanRowUntouched(t *testing.T) {
	repo := newStubSettingsStore()
	if err := repo.Save(context.Background(), domain.SettingsKeyCaveman, `{"enabled":false,"level":"full"}`); err != nil {
		t.Fatalf("seeding the caveman row: %v", err)
	}
	repo.written = nil // the seeding write is not the patch's doing
	svc := newSettingsUpdateFixture(t, repo)

	if _, err := svc.Update(context.Background(), domain.SettingsPatch{
		TokenSaver: &domain.TokenSaverSettingsPatch{RTK: &domain.TokenSaverRTKPatch{Enabled: boolPtr(true)}},
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	for _, key := range repo.written {
		if key == domain.SettingsKeyCaveman {
			t.Fatal("the deprecated caveman row was rewritten, want it frozen")
		}
	}
}

// TestSettingsService_UpdateRoundTripsThroughTheStore covers the read-back the
// panel depends on: the document Update returns is the document a following
// read answers with, field for field.
func TestSettingsService_UpdateRoundTripsThroughTheStore(t *testing.T) {
	svc := newSettingsUpdateFixture(t, newStubSettingsStore())
	patch := domain.SettingsPatch{
		Security: &domain.SecuritySettingsPatch{RequireLogin: boolPtr(false)},
		Network:  &domain.NetworkSettingsPatch{OutboundProxyURL: strPtr("http://proxy.internal:8080")},
	}

	written, err := svc.Update(context.Background(), patch)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	read, err := svc.Settings(context.Background())
	if err != nil {
		t.Fatalf("Settings() error = %v", err)
	}
	if !reflect.DeepEqual(written, read) {
		t.Fatalf("read-back = %+v, want the written document %+v", read, written)
	}
	if read.Security.RequireLogin {
		t.Fatal("require_login survived as true, want the patched false")
	}
	if read.Network.OutboundProxyURL != "http://proxy.internal:8080" {
		t.Fatalf("outbound_proxy_url = %q, want the patched value", read.Network.OutboundProxyURL)
	}
}

// TestSettingsService_CorruptStoredRowFallsBackToDefault pins the merge rule:
// a stored row that does not decode (a hand edit, a newer version's shape) is
// skipped for the documented default, so one bad row cannot lock the panel out
// of its own settings, and a patch of another group still succeeds.
func TestSettingsService_CorruptStoredRowFallsBackToDefault(t *testing.T) {
	repo := newStubSettingsStore()
	if err := repo.Save(context.Background(), domain.SettingsKeySecurity, "{not json"); err != nil {
		t.Fatalf("seeding a corrupt row: %v", err)
	}
	svc := newSettingsUpdateFixture(t, repo)

	got, err := svc.Settings(context.Background())
	if err != nil {
		t.Fatalf("Settings() error = %v", err)
	}
	if !reflect.DeepEqual(got.Security, domain.DefaultSettings().Security) {
		t.Fatalf("security = %+v, want the documented defaults", got.Security)
	}

	next, err := svc.Update(context.Background(), domain.SettingsPatch{
		Logging: &domain.LoggingSettingsPatch{RetentionDays: intPtr(30)},
	})
	if err != nil {
		t.Fatalf("Update() beside a corrupt row error = %v", err)
	}
	if next.Logging.RetentionDays != 30 {
		t.Fatalf("retention_days = %d, want the patched 30", next.Logging.RetentionDays)
	}
}

// TestSettingsService_InvalidPatchPersistsNothing pins the write ordering: the
// merged document is validated before anything is stored, so a rejected patch
// leaves the store exactly as it was.
func TestSettingsService_InvalidPatchPersistsNothing(t *testing.T) {
	cases := []struct {
		name  string
		patch domain.SettingsPatch
	}{
		{"zero sticky limit", domain.SettingsPatch{Routing: &domain.RoutingSettingsPatch{StickyLimit: intPtr(0)}}},
		{"headroom on without url", domain.SettingsPatch{TokenSaver: &domain.TokenSaverSettingsPatch{
			Headroom: &domain.TokenSaverHeadroomPatch{Enabled: boolPtr(true)},
		}}},
		{"unknown filter name", domain.SettingsPatch{TokenSaver: &domain.TokenSaverSettingsPatch{
			RTK: &domain.TokenSaverRTKPatch{Filters: &[]string{"not-a-filter"}},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newStubSettingsStore()
			svc := newSettingsUpdateFixture(t, repo)

			if _, err := svc.Update(context.Background(), tc.patch); err == nil {
				t.Fatal("Update() error = nil, want a validation failure")
			} else if !strings.Contains(domain.AsAppError(err).Code, "VALIDATION") {
				t.Fatalf("code = %q, want VALIDATION_ERROR", domain.AsAppError(err).Code)
			}
			if len(repo.written) != 0 {
				t.Fatalf("written rows = %v, want none", repo.written)
			}
		})
	}
}

// newSettingsUpdateFixture builds the settings service over the shared stub
// store, which records which keys a patch wrote.
func newSettingsUpdateFixture(t *testing.T, repo *stubSettingsStore) *SettingsService {
	t.Helper()
	svc, err := NewSettingsService(SettingsServiceDeps{Repo: repo})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}
	return svc
}

func boolPtr(v bool) *bool { return &v }
