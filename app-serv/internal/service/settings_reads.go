// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings_reads.go
// @for       The named settings reads the data plane's seams ask for, so no seam decodes the whole document.
// @uses      internal/domain, context.
// @reason    The selector needs one provider's credential rotation policy and its parking windows, and the reasoning injection needs one provider's thinking mode; all three are questions about the same stored document, and answering them here is what keeps §7.14's shape out of the data plane. They live beside the read they wrap, in their own file so settings.go stays inside the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-26
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// RotationPolicy resolves the credential rotation policy for one provider
// (§7.5, §7.14): the provider's own override over the global default. It is the
// one settings read the data plane's selector needs, so it is a named method
// rather than making the selector decode the whole document.
func (s *SettingsService) RotationPolicy(ctx context.Context, providerID string) (domain.RotationPolicy, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return domain.RotationPolicy{}, err
	}
	return settings.Routing.RotationFor(providerID), nil
}

// FailoverBudget resolves the walk budget one client request gets, the routing group's global
// bound rather than a per-provider one because the chain it cuts belongs to the request. A read
// that fails answers the documented budget: an outage must not grant an unbounded walk.
func (s *SettingsService) FailoverBudget(ctx context.Context) (domain.FailoverBudget, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return domain.FailoverBudget{}.Normalized(), err
	}
	return settings.Routing.Failover.Normalized(), nil
}

// KeyHealthPolicy resolves the parking windows configured for one provider
// (§7.5, §7.14): the provider's own override over the group default. Both readers
// of the health model ask it, so the Test button and a live request park the same
// credential for the same length.
func (s *SettingsService) KeyHealthPolicy(ctx context.Context, providerID string) (domain.KeyHealthPolicy, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return domain.KeyHealthPolicy{}, err
	}
	return settings.Routing.HealthFor(providerID), nil
}

// ThinkingMode resolves one provider's stored reasoning mode (§7.14): the
// second result is false when the provider has no entry, which is the "auto"
// state the injection leaves alone. It is the one settings read the data
// plane's injection needs, so it is a named method rather than making the
// engine decode the whole document.
func (s *SettingsService) ThinkingMode(ctx context.Context, providerID string) (domain.ThinkingMode, bool, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return "", false, err
	}
	mode, found := settings.Reasoning.ThinkingFor(providerID)
	return mode, found, nil
}
