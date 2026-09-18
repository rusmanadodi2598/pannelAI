// Command app-serv adapts the registry's capability knowledge to the service
//
//	layer's predicate shape.
//
// @file      cmd/app-serv/vision_capability_test.go
// @for       Test for the vision predicate the adapter service is wired with.
// @uses      testing, internal/domain.
// @reason    The adapter is three lines, and the failure it guards against is
//
//	subtle: a wiring that supplies no predicate leaves the service on its
//	reject-everything default, which looks like a working API that refuses
//	every model (§7.8). Pinning the two answers keeps that from returning
//	unnoticed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package main

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestVisionCapabilityCheck_AnswersForACatalogRef pins both arms, so the
// predicate cannot silently become the reject-all default.
func TestVisionCapabilityCheck_AnswersForACatalogRef(t *testing.T) {
	cases := []struct {
		provider string
		model    string
		want     bool
	}{
		{provider: "openai", model: "gpt-4o", want: true},
		{provider: "anthropic", model: "claude-sonnet-4-6", want: true},
		{provider: "deepseek", model: "deepseek-chat", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.provider+"/"+tc.model, func(t *testing.T) {
			ref, err := domain.NewModelRef(tc.provider, tc.model)
			if err != nil {
				t.Fatalf("NewModelRef() error = %v", err)
			}
			if got := visionCapabilityCheck(ref); got != tc.want {
				t.Fatalf("visionCapabilityCheck(%s) = %v, want %v", ref, got, tc.want)
			}
		})
	}
}
