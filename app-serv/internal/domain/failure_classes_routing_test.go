// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/failure_classes_routing_test.go
// @for       The per-class failure policy table: how it layers group over default and provider over group, what a write refuses, and what one PATCH leaves alone.
// @uses      testing, time.
// @reason    The table is read on every failure the router records, and it is written by an operator who cannot see the read. Each layering rule and each rejection is therefore pinned where it is decided, rather than inferred from a router test that happens to pass.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-15
package domain

import (
	"strings"
	"testing"
	"time"
)

func TestRoutingSettings_HealthForLayersTheClassTables(t *testing.T) {
	settings := RoutingSettings{
		FailureClasses: FailureClassPolicies{
			FailureRateLimitTemporary: {CeilingSeconds: 60},
			FailureQuotaExhausted:     {Action: FailureActionRotate},
		},
		ProviderStrategies: map[string]ProviderStrategy{
			"alpha": {FailureClasses: FailureClassPolicies{
				FailureRateLimitTemporary: {CeilingSeconds: 300, Action: FailureActionPark},
			}},
			"beta": {},
		},
	}

	alpha := settings.HealthFor("alpha")
	if got := alpha.PolicyFor(FailureRateLimitTemporary); got.Ceiling(0) != 5*time.Minute ||
		got.Action != FailureActionPark {
		t.Fatalf("alpha rate-limit policy = %+v, want the provider's 300 and park", got)
	}
	if got := alpha.PolicyFor(FailureQuotaExhausted).ActionOrDefault(defaultActionFor(KeyFailureQuota)); got != FailureActionRotate {
		t.Fatalf("alpha quota action = %s, want the group's rotate inherited", got)
	}

	beta := settings.HealthFor("beta")
	if got := beta.PolicyFor(FailureRateLimitTemporary).Ceiling(0); got != time.Minute {
		t.Fatalf("beta rate-limit ceiling = %v, want the group's 60s", got)
	}
	if got := DefaultKeyHealthPolicy().PolicyFor(FailureRateLimitTemporary).Ceiling(0); got != 5*time.Minute {
		t.Fatalf("unconfigured ceiling = %v, want the documented 5m", got)
	}
}

func TestValidateRouting_RefusesClassesTheRouterCannotTune(t *testing.T) {
	cases := []struct {
		name    string
		classes FailureClassPolicies
		want    string
	}{
		{
			name:    "a class the gateway does not produce",
			classes: FailureClassPolicies{"MADE_UP": {CeilingSeconds: 60}},
			want:    "is not a failure class",
		},
		{
			name:    "an action the router cannot execute",
			classes: FailureClassPolicies{FailureAuthInvalid: {Action: FailureAction("explode")}},
			want:    "must be one of park, rotate, stop",
		},
		{
			name:    "a ceiling past the stored bound",
			classes: FailureClassPolicies{FailureAuthInvalid: {CeilingSeconds: MaxFailureClassSeconds + 1}},
			want:    "ceiling_seconds",
		},
		{
			name:    "a class that parks nothing",
			classes: FailureClassPolicies{FailureTargetUnreachable: {Action: FailureActionPark}},
			want:    "implicates no credential",
		},
		{
			name:    "a gate the router cannot withhold a key for",
			classes: FailureClassPolicies{FailureGateBlocked: {CooldownSeconds: 30}},
			want:    "implicates no credential",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := defaultRoutingSettings()
			settings.FailureClasses = tc.classes
			err := validateRouting(settings)
			if err == nil {
				t.Fatalf("validateRouting(%+v) = nil, want it refused", tc.classes)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to say %q", err, tc.want)
			}
		})
	}
}

// TestApplyRouting_MergesOneNamedClassLeavesTheRest is the reason the table merges
// rather than replaces: a PATCH that retunes the throttle must not silently put
// every other class back to its default.
func TestApplyRouting_MergesOneNamedClassLeavesTheRest(t *testing.T) {
	stored := RoutingSettings{FailureClasses: FailureClassPolicies{
		FailureQuotaExhausted: {CeilingSeconds: 3600},
	}}
	patch := RoutingSettingsPatch{FailureClasses: &FailureClassPolicies{
		FailureRateLimitTemporary: {Action: FailureActionStop},
	}}

	applyRouting(&stored, &patch)
	if got := stored.FailureClasses[FailureQuotaExhausted].CeilingSeconds; got != 3600 {
		t.Fatalf("quota ceiling = %d, want the stored hour left where it was", got)
	}
	if got := stored.FailureClasses[FailureRateLimitTemporary].Action; got != FailureActionStop {
		t.Fatalf("rate-limit action = %s, want the patched stop", got)
	}
}

// TestValidateRouting_AClassEntryAloneMakesAProviderEntryLive is the other half of
// the merge rule: an entry that names only a class policy is a decision, and an
// entry that names nothing at all is dead configuration the write refuses.
func TestValidateRouting_AClassEntryAloneMakesAProviderEntryLive(t *testing.T) {
	live := defaultRoutingSettings()
	live.ProviderStrategies = map[string]ProviderStrategy{
		"alpha": {FailureClasses: FailureClassPolicies{FailureAuthInvalid: {CeilingSeconds: 90}}},
	}
	if err := validateRouting(live); err != nil {
		t.Fatalf("validateRouting() error = %v, want the entry accepted", err)
	}

	dead := defaultRoutingSettings()
	dead.ProviderStrategies = map[string]ProviderStrategy{"alpha": {}}
	err := validateRouting(dead)
	if err == nil {
		t.Fatal("validateRouting() = nil, want an empty entry refused")
	}
	if !strings.Contains(err.Error(), "failure_classes") {
		t.Fatalf("error = %q, want the refusal to name the new field", err)
	}
}
