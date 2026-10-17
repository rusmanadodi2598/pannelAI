// Command app-serv wires the §7.4 OAuth flow into the management graph.
//
// @file      cmd/app-serv/oauth_wiring.go
// @for       Builds the OAuth flow service, its handler, and its refresh worker from the graph's shared seams.
// @uses      context, net/http, redis, internal/config, internal/domain, internal/handler, internal/repository/redis, internal/service, internal/service/quotafetch.
// @reason    AGENTS.md §1.5 makes this file wiring only, and §1.1 caps management_wiring.go, which the OAuth block pushed over. It is a separate function rather than inline construction because the handler and the worker must share one flow service: two instances would mean two views of the same tokens.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-19
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// buildOAuth assembles the §7.4 flow. The state store is Redis-backed because
// the replay guard must be single-use across processes, and the token client
// speaks both grant encodings from the process's guarded HTTP pool.
func buildOAuth(
	cfg config.Config,
	index service.ProviderIndex,
	store service.OAuthAccountStore,
	client redis.UniversalClient,
	sealer *domain.Sealer,
	upstream *http.Client,
) (*handler.OAuthHandler, *service.OAuthRefreshWorker, error) {
	tokens, err := oauthhttp.NewOAuthHTTPClient(upstream)
	if err != nil {
		return nil, nil, fmt.Errorf("management wiring: oauth token client: %w", err)
	}
	flow, err := service.NewOAuthFlowService(service.OAuthFlowDeps{
		Index: index, Store: store,
		States:          redisrepo.NewOAuthStateStore(client),
		Tokens:          tokens,
		Sealer:          sealer,
		ProviderClients: oauthClientMaterial(cfg),
		Projects:        projectLookup{index: index},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("management wiring: oauth flow: %w", err)
	}
	return handler.NewOAuthHandler(flow, cfg.PublicBaseURL),
		service.NewOAuthRefreshWorker(flow, index), nil
}

// oauthClientMaterial moves the deployment's OAuth client material across the
// boundary the flow reads. The registry document is generated and committed, so
// a secret it does not carry arrives here instead.
func oauthClientMaterial(cfg config.Config) map[string]service.OAuthClient {
	if len(cfg.OAuthClients) == 0 {
		return nil
	}
	clients := make(map[string]service.OAuthClient, len(cfg.OAuthClients))
	for providerID, client := range cfg.OAuthClients {
		clients[providerID] = service.OAuthClient{ClientID: client.ClientID, ClientSecret: client.ClientSecret}
	}
	return clients
}

// projectLookup lands the Cloud project a login's token holds, asking the
// package that already speaks that protocol rather than a second reading of it.
// The lookup rides quotafetch's guarded client, the same one the quota pollers
// were installed onto at boot, so no egress path opens here that was not open.
type projectLookup struct {
	index service.ProviderIndex
}

func (p projectLookup) ResolveProject(ctx context.Context, providerID, accessToken string) (string, bool) {
	entry, ok := p.index.Provider(providerID)
	if !ok {
		return "", false
	}
	endpoint, userAgent := entry.ProjectBootstrap()
	if endpoint == "" {
		return "", false
	}
	return quotafetch.LoadProject(ctx, quotafetch.Credentials{
		Endpoints: quotafetch.UsageEndpoints{LoadCodeAssistURL: endpoint},
	}, accessToken, userAgent)
}
