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
