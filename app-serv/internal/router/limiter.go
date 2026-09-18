// Package router maps HTTP routes and cross-cutting middleware.
//
// @file      internal/router/limiter.go
// @for       Enforces the configured Redis-backed gateway request budget.
// @uses      internal/domain, internal/repository, internal/schema, net/http.
// @reason    SPEC-API-001 §4 requires public traffic to be rate-limited by
//
//	configuration rather than leaving the validated value unused.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-09-17
package router

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// requestRateLimit applies one configured fixed window per client address.
func requestRateLimit(next http.Handler, limiter repository.RateLimiter, limit int) http.Handler {
	if limiter == nil || limit < 1 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == APIVersion+"/health" || r.URL.Path == APIVersion+"/version" {
			next.ServeHTTP(w, r)
			return
		}
		remaining, err := limiter.Allow(r.Context(), clientAddress(r), limit, time.Minute)
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

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
