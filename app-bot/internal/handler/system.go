// Package handler holds the HTTP surface of app-bot.
//
// @file      internal/handler/system.go
// @for       Liveness, readiness, and metrics responses.
// @uses      context, encoding/json, net/http, strconv, time.
// @reason    The process needs one readiness answer that fails closed when either dependency is down, and one metrics body that does not require a metrics library yet.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-21
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Pinger is satisfied by pgxpool.Pool and redis.Client.
type Pinger interface {
	Ping(ctx context.Context) error
}

// System serves the operational routes.
type System struct {
	postgres Pinger
	redis    Pinger
	timeout  time.Duration
	started  time.Time
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// NewSystem wires the two dependency probes with one shared timeout.
func NewSystem(postgres, redis Pinger, timeout time.Duration) *System {
	return &System{postgres: postgres, redis: redis, timeout: timeout, started: time.Now()}
}

// Health reports that the process is alive, without touching a dependency.
func (s *System) Health(w http.ResponseWriter, _ *http.Request) {
	writeBody(w, http.StatusOK, []byte("{\"status\":\"ok\"}\n"))
}

// Ready reports 200 only when both dependencies answer inside the timeout.
func (s *System) Ready(w http.ResponseWriter, _ *http.Request) {
	checks := map[string]string{"postgres": "ok", "redis": "ok"}
	status := "ready"
	code := http.StatusOK

	if err := s.ping(s.postgres); err != nil {
		checks["postgres"] = "down"
		status = "unavailable"
		code = http.StatusServiceUnavailable
	}
	if err := s.ping(s.redis); err != nil {
		checks["redis"] = "down"
		status = "unavailable"
		code = http.StatusServiceUnavailable
	}

	body, err := json.Marshal(readyResponse{Status: status, Checks: checks})
	if err != nil {
		writeBody(w, http.StatusInternalServerError, []byte("{\"status\":\"error\"}\n"))
		return
	}
	writeBody(w, code, append(body, '\n'))
}

// Metrics answers Prometheus text from process state alone.
func (s *System) Metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	uptime := int64(time.Since(s.started).Seconds())
	body := "app_bot_up 1\napp_bot_uptime_seconds " + strconv.FormatInt(uptime, 10) + "\n"
	writeBody(w, http.StatusOK, []byte(body))
}

func (s *System) ping(pinger Pinger) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	return pinger.Ping(ctx)
}

func writeBody(w http.ResponseWriter, status int, body []byte) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		return
	}
}
