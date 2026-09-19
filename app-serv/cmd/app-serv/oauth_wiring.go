// Command app-serv wires the §7.4 OAuth flow into the management graph.
//
// @file      cmd/app-serv/oauth_wiring.go
// @for       Builds the OAuth flow service, its handler, and its refresh worker
//
//	from the graph's shared seams.
//
// @uses      internal/config, internal/domain, internal/handler,
//
//	internal/repository/redis, internal/service, redis.
//
// @reason    AGENTS.md §1.5 makes this file wiring only, and §1.1 caps
//
//	management_wiring.go, which the OAuth block pushed over. It is a
//	separate function rather than inline construction because the
//	handler and the worker must share one flow service: two instances
//	would mean two views of the same tokens.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// buildOAuth assembles the §7.4 flow. The state store is Redis-backed because
// the replay guard must be single-use across processes, and the token client
// speaks both grant encodings from the one shared HTTP pool.
func buildOAuth(
	cfg config.Config,
	index service.ProviderIndex,
	store service.OAuthAccountStore,
	client redis.UniversalClient,
	sealer *domain.Sealer,
) (*handler.OAuthHandler, *service.OAuthRefreshWorker, error) {
	flow, err := service.NewOAuthFlowService(service.OAuthFlowDeps{
		Index: index, Store: store,
		States: redisrepo.NewOAuthStateStore(client),
		Tokens: service.NewOAuthHTTPClient(nil), Sealer: sealer,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("management wiring: oauth flow: %w", err)
	}
	return handler.NewOAuthHandler(flow, cfg.PublicBaseURL),
		service.NewOAuthRefreshWorker(flow, index), nil
}
