// Package router maps HTTP routes and cross-cutting middleware.
//
// @file      internal/router/limiter.go
// @for       Enforces the configured Redis-backed gateway request budget.
// @uses      internal/clientip, internal/domain, internal/repository,
//
//	internal/schema, net, net/http.
//
// @reason    SPEC-API-001 §4 requires public traffic to be rate-limited by
//
//	configuration rather than leaving the validated value unused, and
//	draft 042 R20 requires the bucket to name the real client when the
//	gateway sits behind a proxy the operator has named as trusted.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-09-17
package router

import (
	"net"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/clientip"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// requestRateLimit applies one configured fixed window per client address. The
// trusted-proxy set decides which address that is: with none configured the
// direct peer is the whole rule, and a proxy the operator named hands the
// bucket to the forwarded client (draft 042 R20).
func requestRateLimit(next http.Handler, limiter repository.RateLimiter, limit int, trusted []*net.IPNet) http.Handler {
	if limiter == nil || limit < 1 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == APIVersion+"/health" || r.URL.Path == APIVersion+"/version" {
			next.ServeHTTP(w, r)
			return
		}
		remaining, err := limiter.Allow(r.Context(),
			clientip.Address(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), trusted), limit, time.Minute)
		if err != nil {
			schema.WriteError(w, err)
			return
		}
		if remaining > 0 {
			schema.WriteError(w, domain.NewRateLimitedAfter("request rate limit exceeded", remaining))
			return
		}
		next.ServeHTTP(w, r)
	})
}
