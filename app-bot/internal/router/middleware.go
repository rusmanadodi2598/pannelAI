// Package router wires the app-bot HTTP surface.
//
// @file      internal/router/middleware.go
// @for       Request id, access log, panic recovery, and loopback gating.
// @uses      crypto/rand, encoding/hex, log/slog, net, net/http, regexp, runtime/debug, strings, time.
// @reason    These four concerns are cross-cutting and security relevant, so they live together and are tested through the router rather than per handler.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-10-21
package router

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

const requestIDHeader = "X-Request-Id"

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if !requestIDPattern.MatchString(id) {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r)
	})
}

func accessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
			"request_id", w.Header().Get(requestIDHeader),
		)
	})
}

func recoverPanics(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("handler panic recovered",
					"panic", recovered,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				if _, err := w.Write([]byte("{\"code\":\"BOT_INTERNAL\",\"message\":\"internal server error\"}\n")); err != nil {
					return
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// loopbackOnly admits a request only when the peer is loopback AND, if a proxy
// forwarded it, the last hop is loopback too. A same-host reverse proxy makes
// every peer loopback, so the peer address alone cannot mean local.
func loopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackPeer(r.RemoteAddr) || !loopbackForwarded(r.Header.Get("X-Forwarded-For")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loopbackPeer reports whether the TCP peer address is a loopback address.
func loopbackPeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// loopbackForwarded reports whether the last hop named in X-Forwarded-For is a
// loopback address. An absent header means the request was not proxied.
func loopbackForwarded(header string) bool {
	if header == "" {
		return true
	}
	hops := strings.Split(header, ",")
	return loopbackPeer(net.JoinHostPort(strings.TrimSpace(hops[len(hops)-1]), "0"))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func newRequestID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(buffer)
}
