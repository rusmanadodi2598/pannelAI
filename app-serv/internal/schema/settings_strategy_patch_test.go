// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_strategy_patch_test.go
// @for       The outbound_proxy_strategy PATCH field: the closed set at the
//
//	boundary, the tag-versus-domain drift pin, the mapping into the
//	domain patch, and the effective value the read answers.
//
// @uses      testing, reflect, strings, internal/domain.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D2 makes the strategy a closed
//
//	set whose empty stored value reads as the default. The tag is what
//	refuses a bad value at the boundary and the domain's parse is what
//	the plan runs on, so a member added to one without the other would
//	either poison every read (an out-of-set write that lands) or strand
//	a valid one (a member the boundary refuses). The tests pin both
//	directions, plus the read that answers fallback for a blank.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-26
package schema

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// proxyStrategySet is the domain's closed set, written out for the drift pin.
var proxyStrategySet = []string{domain.ProxyStrategyFallback, domain.ProxyStrategyRoundRobin}

// TestValidateStruct_ProxyStrategyPatch drives the field through the same
// validate call the handler makes: both members and both blank states pass,
// and the out-of-set spellings the reference allowed are refused here.
func TestValidateStruct_ProxyStrategyPatch(t *testing.T) {
	cases := []struct {
		name    string
		strage  *string
		wantErr bool
	}{
		{name: "an omitted strategy", strage: nil},
		{name: "fallback", strage: strPtr("fallback")},
		{name: "round_robin", strage: strPtr("round_robin")},
		// The boundary refuses a non-nil empty string, unlike the URL fields:
		// an empty URL is a real state (no URL), but the strategy has no empty
		// member: the reset lever is sending fallback, the default's own name.
		// A nil field is the only "leave unchanged" (D2's blank reading is about
		// stored documents, not the wire).
		{name: "an emptied strategy is refused", strage: strPtr(""), wantErr: true},
		{name: "the reference's random is refused", strage: strPtr("random"), wantErr: true},
		{name: "an uppercase member is refused", strage: strPtr("Fallback"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			patch := PatchSettingsRequest{Network: &NetworkSettingsPatch{OutboundProxyStrategy: tc.strage}}
			err := ValidateStruct(patch)
			if tc.wantErr && err == nil {
				t.Fatal("ValidateStruct() = nil, want the out-of-set strategy refused")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateStruct() = %v, want the strategy accepted", err)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "OutboundProxyStrategy") {
				t.Fatalf("error = %v, want it to name the field", err)
			}
		})
	}
}

// TestProxyStrategyTags_MatchTheDomainSet pins the oneof tag against the
// domain's set in both directions: a member the tag carries must parse, and a
// member the domain runs must be at the boundary.
func TestProxyStrategyTags_MatchTheDomainSet(t *testing.T) {
	field, ok := reflect.TypeOf(NetworkSettingsPatch{}).FieldByName("OutboundProxyStrategy")
	if !ok {
		t.Fatal("NetworkSettingsPatch carries no OutboundProxyStrategy field")
	}
	tagged := oneofValues(field.Tag.Get("validate"))
	if !reflect.DeepEqual(tagged, proxyStrategySet) {
		t.Fatalf("oneof list = %v, want the domain set %v", tagged, proxyStrategySet)
	}
	for _, member := range tagged {
		if _, err := domain.ParseProxyStrategy(member); err != nil {
			t.Fatalf("ParseProxyStrategy(%q) = %v, want every tagged member to parse", member, err)
		}
	}
}

// TestToSettingsPatch_CarriesTheStrategy pins the lowering: a strategy the
// boundary accepted cannot be lost between the DTO and the domain patch.
func TestToSettingsPatch_CarriesTheStrategy(t *testing.T) {
	patch := ToSettingsPatch(PatchSettingsRequest{
		Network: &NetworkSettingsPatch{OutboundProxyStrategy: strPtr(domain.ProxyStrategyRoundRobin)},
	})
	if patch.Network == nil || patch.Network.OutboundProxyStrategy == nil {
		t.Fatal("the lowered patch carries no strategy")
	}
	if got := *patch.Network.OutboundProxyStrategy; got != domain.ProxyStrategyRoundRobin {
		t.Fatalf("lowered strategy = %q, want %q", got, domain.ProxyStrategyRoundRobin)
	}
}

// TestSettingsResponseFrom_AnswersTheEffectiveStrategy pins the read side: a
// blank stored value is answered as the default the plan will run, so the
// panel renders what the engine does rather than a blank.
func TestSettingsResponseFrom_AnswersTheEffectiveStrategy(t *testing.T) {
	blank := domain.Settings{}
	blank.Network = domain.NetworkSettings{OutboundProxyStrategy: ""}
	if got := SettingsResponseFrom(blank).Network.OutboundProxyStrategy; got != domain.DefaultProxyStrategy {
		t.Fatalf("blank strategy read = %q, want %q", got, domain.DefaultProxyStrategy)
	}
	stored := domain.Settings{}
	stored.Network = domain.NetworkSettings{OutboundProxyStrategy: domain.ProxyStrategyRoundRobin}
	if got := SettingsResponseFrom(stored).Network.OutboundProxyStrategy; got != domain.ProxyStrategyRoundRobin {
		t.Fatalf("stored strategy read = %q, want %q", got, domain.ProxyStrategyRoundRobin)
	}
}
