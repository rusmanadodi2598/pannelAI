// Command app-serv wires the production authentication dependencies.
//
// @file      cmd/app-serv/auth_wiring.go
// @for       Constructs and bootstraps dashboard authentication dependencies.
// @uses      internal/config, internal/handler, internal/repository, internal/service, pgxpool, redis.
// @reason    Keeping auth wiring separate preserves the composition root's line
//
//	limit while making PostgreSQL/Redis dependencies explicit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-17
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

func buildAuth(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, client redis.UniversalClient) (*handler.AuthHandler, *redisrepo.Limiter, error) {
	sessions := redisrepo.NewSessionStore(client)
	limiter := redisrepo.NewLimiter(client)
	authSvc, err := service.NewAuthService(service.AuthServiceDeps{
		Repo: postgres.NewAuthRepository(pool), Sessions: sessions, Limiter: limiter,
		Hasher: service.BcryptHasher{}, Secret: []byte(cfg.SessionSecret), SessionTTL: cfg.SessionTTL,
		LoginMaxFails: cfg.LoginMaxFails, LoginLockout: cfg.LoginLockout,
		BootstrapPassword: cfg.BootstrapPassword,
	})
	if err != nil {
		return nil, nil, err
	}
	bootstrapCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := authSvc.Bootstrap(bootstrapCtx); err != nil {
		return nil, nil, fmt.Errorf("auth bootstrap: %w", err)
	}
	return handler.NewAuthHandler(authSvc, handler.SessionCookieOptions{
		Secure: cfg.IsProduction(), TTL: cfg.SessionTTL,
	}), limiter, nil
}
