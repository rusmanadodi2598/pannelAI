// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_ports.go
// @for       The narrow questions the data plane asks about stored media
//
//	configuration.
//
// @uses      internal/domain, context.
// @reason    §7.10 lets an operator point a provider's media kind at their own
//
//	host, and the data plane must honour that at call time — a value
//	read once at boot would keep dialing the old host after a save.
//	The interface is declared here rather than reusing the media
//	provider service so the embeddings use case depends on the one
//	question it asks, not on the whole management surface.
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
)

// MediaOverrideReader reads the stored §7.10 override for one provider and
// kind. MediaProviderService implements it.
type MediaOverrideReader interface {
	// MediaBaseURL returns the stored base URL for one provider and kind, or
	// an empty string when no override is stored. An empty answer is not an
	// error: the registry's own base URL applies then.
	MediaBaseURL(ctx context.Context, providerID string, kind domain.MediaKind) (string, error)
}

// MediaRouter picks the endpoint a media call uses and applies the call's
// outcome to that endpoint's health, so a media route feeds the same circuit
// state the chat plane reads.
//
// The methods are the engine's, gathered into one port because a media call
// asks all three of the same collaborator: taking the engine itself would drag
// the resolver and the wire translators into every media test.
type MediaRouter interface {
	Select(ctx context.Context, providerID string) (dataplane.Selection, error)
	RecordSuccess(ctx context.Context, selection dataplane.Selection) error
	RecordFailure(ctx context.Context, selection dataplane.Selection, reason string, class domain.KeyFailureClass) error
}

// ModelResolver turns a client model string into a routable provider. It is the
// one question the embeddings use case asks of the resolver, kept as its own
// port for the same reason as MediaRouter: the use case must not carry the
// engine's wire translators to answer it.
type ModelResolver interface {
	Resolve(ctx context.Context, model string) (dataplane.Resolution, error)
}

// KindModelResolver is the same question asked for one plane, which is what the
// decision route needs: it accepts only models declaring the systemone kind, so
// asking the chat question would either serve a chat model with a decision
// payload or refuse the model it wants. It is a separate port rather than a
// second method on ModelResolver because the planes differ in the question they
// ask, not in who answers it: the chat and media use cases have no reason to
// carry a method they never call.
type KindModelResolver interface {
	ResolveForSystemOne(ctx context.Context, model string) (dataplane.Resolution, error)
}

// GatewayAuthenticator applies the §4 gateway-key rule to one data-plane
// request. ChatService implements it.
//
// The media and embeddings handlers take this rather than the chat service
// itself for the same reason the router seam exists: a handler test should not
// have to build the engine the chat service needs in order to prove that a
// route refuses a bad key.
type GatewayAuthenticator interface {
	Authenticate(ctx context.Context, presented string) (domain.GatewayKey, error)
}
