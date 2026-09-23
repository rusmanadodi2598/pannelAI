// Command app-serv adapts the stateless credential check to a registry provider.
//
// @file      cmd/app-serv/provider_validate_registry.go
// @for       The provider-side entry point: resolve the entry, derive its plan,
//
//	and report which request proved the answer.
//
// @uses      internal/domain, internal/provider, internal/registry,
//
//	internal/service, context, net/http, strings.
//
// @reason    Draft 017 §4.2's second consequence is per-format, and this is where
// //
//
//	the plan meets the wire: a declared URL, a derived models path, or the
//	Anthropic wire's own messages endpoint. Keeping it apart from the node
//	path keeps each file about one destination kind.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// validateProvider implements service.CredentialValidator for a registry provider.
func (p *httpEndpointProber) ValidateProvider(ctx context.Context, check service.CredentialCheck) (service.ProbeOutcome, error) {
	entry, connector, err := p.resolve(check.ProviderID)
	if err != nil {
		return service.ProbeOutcome{}, err
	}
	cred := provider.StaticKey("", "", check.Credential)
	if check.Credential == "" {
		cred = provider.NoCredential(entry.ID)
	}
	plan := service.PlanFor(entry)
	if plan.URL == "" {
		return failure("this provider declares no validation endpoint and its base URL names no models path"), nil
	}
	if plan.Method == "POST" {
		// A claude-format base that IS the messages endpoint: post to it exactly
		// (reference providers/validate/route.js:303-329).
		return p.validate(ctx, validateRequest{
			base:      plan.URL,
			entry:     entry,
			connector: connector,
			cred:      cred,
			anthropic: true,
			direct:    true,
			modelID:   check.ModelID,
		})
	}
	outcome, err := p.probe(ctx, plan.URL, entry, connector, cred)
	if err != nil {
		return outcome, err
	}
	outcome.Method = service.ProbeMethodModels
	if plan.AnthropicRule {
		return service.ValidateOutcomeForAnthropic(outcome.Status), nil
	}
	return outcome, nil
}
