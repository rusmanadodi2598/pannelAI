// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_refusal.go
// @for       The log row a §7.10 media call leaves when it is refused before
//
//	any upstream attempt, and the identity such a row can still name.
//
// @uses      internal/dataplane, internal/domain, internal/schema, context,
//
//	strings.
//
// @reason    Register G20: the chat plane records every call, including one
//
//	refused before the pipeline ran, while the media routes wrote rows
//	only after Perform — so a refusal the client saw never appeared in
//	the Logs screen. The identity rule lives here because a refusal has
//	no resolved call to read it from, only the model string the client
//	sent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// prepareForCall runs Prepare and records the one log row a refusal leaves
// (register G20), so a media call the client saw fail is visible in the Logs
// screen even though no upstream was dialed. The usage row stays unwritten:
// nothing reached an attempt.
func (s *MediaCallService) prepareForCall(ctx context.Context, model string, kind domain.MediaKind, query map[string]string, keyID string) (MediaCall, error) {
	call, err := s.Prepare(ctx, model, kind, query)
	if err != nil {
		s.recorder.refuse(ctx, refusalOutcome(model), keyID, err)
	}
	return call, err
}

// refusalOutcome names a refused call by the model string the client sent, as
// far as it parses: `provider/model` fills both columns, and a string with no
// provider is named by nothing rather than guessed at — the same honesty the
// chat plane keeps for a request refused before resolution.
func refusalOutcome(model string) dataplane.Outcome {
	providerID, upstreamModel, err := splitMediaModel(model)
	if err != nil {
		return dataplane.Outcome{}
	}
	return mediaRefusalOutcome(providerID, upstreamModel)
}

// mediaRefusalOutcome names a refusal by a provider the route already resolved.
// A kind that names no model (search) is named by its provider, which is what a
// served search call's row carries too.
func mediaRefusalOutcome(providerID, upstreamModel string) dataplane.Outcome {
	if upstreamModel == "" {
		upstreamModel = providerID
	}
	return dataplane.Outcome{
		Format: schema.FormatOpenAI, ProviderID: providerID, Model: upstreamModel,
	}
}
