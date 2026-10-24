// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_failure_classes_test.go
// @for       The per-class failure policies crossing the wire in both directions: a PATCH lowered to the domain, and a stored table rendered back without inventing defaults.
// @uses      encoding/json, strings, testing, internal/domain.
// @reason    A class table is configuration an operator writes once and the router reads on every failure, so a field lowered to the wrong unit or a default rendered as a decision is a bug the panel cannot see. Both directions are pinned here, at the group level and inside a provider entry.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-15
package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestRoutingPtr_LowersFailureClassesBothWays checks that a class entry reaches the
// domain with the seconds it named and nothing else, at both levels: the group's
// table and one provider's.
func TestRoutingPtr_LowersFailureClassesBothWays(t *testing.T) {
	ceiling := 90
	action := "rotate"
	lowered := routingPtr(&RoutingSettingsPatch{
		FailureClasses: &map[string]FailureClassPolicyPatch{
			string(domain.FailureRateLimitTemporary): {CeilingSeconds: &ceiling},
		},
		ProviderStrategies: &map[string]ProviderStrategyPatch{
			"alpha": {FailureClasses: &map[string]FailureClassPolicyPatch{
				string(domain.FailureQuotaExhausted): {Action: &action},
			}},
			"beta": {StickyLimit: limitPtr(3)},
		},
	})

	group := *lowered.FailureClasses
	if group[domain.FailureRateLimitTemporary] != (domain.FailureClassPolicy{CeilingSeconds: 90}) {
		t.Fatalf("group entry = %+v, want ceiling_seconds 90 and no action", group[domain.FailureRateLimitTemporary])
	}
	entries := *lowered.ProviderStrategies
	if got := entries["alpha"].FailureClasses[domain.FailureQuotaExhausted]; got.Action != domain.FailureActionRotate ||
		got.CeilingSeconds != 0 {
		t.Fatalf("alpha entry = %+v, want the action alone", got)
	}
	if !entries["beta"].FailureClasses.IsEmpty() {
		t.Fatalf("beta entry = %+v, want no class table at all", entries["beta"].FailureClasses)
	}
}

// TestFailureClasses_DropsAClassTheDomainDoesNotHave is the boundary's own job: a
// class name the router cannot produce must not reach storage, where it would sit
// as an entry no failure ever matches.
func TestFailureClasses_DropsAClassTheDomainDoesNotHave(t *testing.T) {
	ceiling := 30
	if got := failureClasses(&map[string]FailureClassPolicyPatch{
		"NOT_A_CLASS": {CeilingSeconds: &ceiling},
	}); got != nil {
		t.Fatalf("lowered table = %+v, want the unknown class dropped entirely", got)
	}
}

// TestSettingsResponseFrom_RendersTheStoredClassesIs the panel's contract: the group
// answers what an operator stored rather than a filled-in copy of nine defaults,
// and a document with no table answers nothing.
func TestSettingsResponseFrom_RendersTheStoredClasses(t *testing.T) {
	settings := domain.Settings{}
	settings.Routing = domain.RoutingSettings{
		FallbackStrategy:   domain.RotationFillFirst,
		ProviderStrategies: map[string]domain.ProviderStrategy{},
		FailureClasses: domain.FailureClassPolicies{
			domain.FailureQuotaExhausted: {Action: domain.FailureActionPark, CeilingSeconds: 3600},
		},
	}

	rendered := SettingsResponseFrom(settings)
	entry := rendered.Routing.FailureClasses[string(domain.FailureQuotaExhausted)]
	if entry.Action != string(domain.FailureActionPark) || entry.CeilingSeconds != 3600 {
		t.Fatalf("group entry = %+v, want the stored park and 3600", entry)
	}
	if _, seen := rendered.Routing.FailureClasses[string(domain.FailureAuthInvalid)]; seen {
		t.Fatal("the render invented a class nobody configured")
	}

	settings.Routing.FailureClasses = nil
	if got := SettingsResponseFrom(settings).Routing.FailureClasses; got != nil {
		t.Fatalf("an empty table rendered %+v, want absent", got)
	}
	encoded, err := json.Marshal(SettingsResponseFrom(settings).Routing)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "failure_classes") {
		t.Fatalf("the wire carries failure_classes for a document that stores none: %s", encoded)
	}
}
