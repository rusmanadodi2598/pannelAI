// Package schema holds the request and response DTOs the wire carries.
//
// @file      internal/schema/settings_routing_failover_test.go
// @for       Tests for the routing PATCH and read-back of the failover walk budget.
// @uses      internal/domain, testing.
// @reason    The tag layer is what refuses an out-of-range budget before a document is stored, and the read-back has to name the budget the router really applies rather than the zeros a pre-migration row holds. Both directions need a pin, or one of them can silently disagree with the other.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-14
package schema

import (
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestValidateStruct_GuardsTheFailoverBounds(t *testing.T) {
	cases := []struct {
		name     string
		patch    PatchSettingsRequest
		wantFail bool
	}{
		{name: "an unset budget is legal", patch: PatchSettingsRequest{}},
		{name: "the smallest legal total passes",
			patch: PatchSettingsRequest{Routing: &RoutingSettingsPatch{
				Failover: &FailoverBudgetPatch{TotalMS: limitPtr(domain.MinFailoverTotalMS)}}}},
		{name: "a total past the ceiling is refused",
			patch: PatchSettingsRequest{Routing: &RoutingSettingsPatch{
				Failover: &FailoverBudgetPatch{TotalMS: limitPtr(domain.MaxFailoverTotalMS + 1)}}},
			wantFail: true},
		{name: "a total below the floor is refused",
			patch: PatchSettingsRequest{Routing: &RoutingSettingsPatch{
				Failover: &FailoverBudgetPatch{TotalMS: limitPtr(domain.MinFailoverTotalMS - 1)}}},
			wantFail: true},
		{name: "an attempt count past the ceiling is refused",
			patch: PatchSettingsRequest{Routing: &RoutingSettingsPatch{
				Failover: &FailoverBudgetPatch{MaxAttempts: limitPtr(domain.MaxFailoverAttempts + 1)}}},
			wantFail: true},
		{name: "zero attempts is refused, because no walk would be allowed",
			patch: PatchSettingsRequest{Routing: &RoutingSettingsPatch{
				Failover: &FailoverBudgetPatch{MaxAttempts: limitPtr(0)}}},
			wantFail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(tc.patch)
			if tc.wantFail && err == nil {
				t.Fatal("ValidateStruct() = nil, want the value refused")
			}
			if !tc.wantFail && err != nil {
				t.Fatalf("ValidateStruct() error = %v, want it accepted", err)
			}
		})
	}
}

// TestSettingsResponseFrom_FailoverIsEffective pins the read-back a panel renders:
// a document that predates the keys names the budget the router applies, so the
// screen cannot show zero attempts for a gateway that walks eight.
func TestSettingsResponseFrom_FailoverIsEffective(t *testing.T) {
	cases := []struct {
		name         string
		stored       domain.FailoverBudget
		wantTotalMS  int
		wantAttempts int
	}{
		{name: "a legacy row answers the documented budget", stored: domain.FailoverBudget{},
			wantTotalMS: domain.DefaultFailoverTotalMS, wantAttempts: domain.DefaultFailoverAttempts},
		{name: "a configured row answers what it stores",
			stored:       domain.FailoverBudget{TotalMS: 45_000, MaxAttempts: 3},
			wantTotalMS:  45_000,
			wantAttempts: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := domain.Settings{Routing: domain.RoutingSettings{Failover: tc.stored}}
			response := SettingsResponseFrom(settings)
			if response.Routing.Failover.TotalMS != tc.wantTotalMS {
				t.Fatalf("total_ms = %d, want %d", response.Routing.Failover.TotalMS, tc.wantTotalMS)
			}
			if response.Routing.Failover.MaxAttempts != tc.wantAttempts {
				t.Fatalf("max_attempts = %d, want %d", response.Routing.Failover.MaxAttempts, tc.wantAttempts)
			}
		})
	}
}

// TestToSettingsPatch_LowersFailover pins that the DTO's two bounds reach the domain
// patch rather than being dropped at the boundary, which is how a stored value and a
// read-back could otherwise disagree.
func TestToSettingsPatch_LowersFailover(t *testing.T) {
	total, attempts := 45_000, 3
	patch := ToSettingsPatch(PatchSettingsRequest{Routing: &RoutingSettingsPatch{
		Failover: &FailoverBudgetPatch{TotalMS: &total, MaxAttempts: &attempts}}})

	if patch.Routing == nil || patch.Routing.Failover == nil {
		t.Fatal("ToSettingsPatch() dropped the failover block")
	}
	if *patch.Routing.Failover.TotalMS != 45_000 || *patch.Routing.Failover.MaxAttempts != 3 {
		t.Fatalf("lowered budget = %v, want the 45000/3 the wire sent", patch.Routing.Failover)
	}
}

// TestFailoverBudgetResponse_Unit pins the unit the wire names: the walk measures
// milliseconds, and a reader that assumed seconds would give every request a
// thirty-millisecond chain.
func TestFailoverBudgetResponse_Unit(t *testing.T) {
	settings := domain.Settings{Routing: domain.RoutingSettings{
		Failover: domain.FailoverBudget{TotalMS: 45_000, MaxAttempts: 3}}}
	response := SettingsResponseFrom(settings)

	if got := time.Duration(response.Routing.Failover.TotalMS) * time.Millisecond; got != 45*time.Second {
		t.Fatalf("total_ms read as milliseconds = %v, want 45s", got)
	}
}
