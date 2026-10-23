// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_test.go
// @for       Table-driven contract tests for the environment configuration.
// @uses      testing, os.
// @reason    AGENTS.md §1.4 forbids raw os.Getenv in business logic and requires fail-fast validation, so the loader is tested as a pure function over a getter.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-21
package config_test

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/config"
)

func TestLoad_GivenEnvironmentVariations_ThenConfigIsAcceptedOrRejected(t *testing.T) {
	base := map[string]string{
		"APP_ENV":      "development",
		"HTTP_ADDR":    "127.0.0.1:8090",
		"DATABASE_URL": "postgres://bot:secret@127.0.0.1:5432/pannelai?search_path=bot",
		"DB_POOL_MAX":  "10",
		"REDIS_ADDR":   "127.0.0.1:6379",
		"LOG_LEVEL":    "info",

		"TELEGRAM_BOT_TOKEN":      "123456789:AAH-test-token-value",
		"TELEGRAM_WEBHOOK_SECRET": "test-secret-1234567890",
	}

	cases := []struct {
		name    string
		mutate  func(map[string]string)
		wantErr string
	}{
		{
			name:   "valid environment loads with defaults",
			mutate: func(map[string]string) {},
		},
		{
			name:    "missing database url is refused by name",
			mutate:  func(env map[string]string) { delete(env, "DATABASE_URL") },
			wantErr: "DATABASE_URL is required",
		},
		{
			name:    "unknown app env is refused",
			mutate:  func(env map[string]string) { env["APP_ENV"] = "production-ish" },
			wantErr: "APP_ENV must be one of",
		},
		{
			name:    "pool size above the ceiling is refused",
			mutate:  func(env map[string]string) { env["DB_POOL_MAX"] = "101" },
			wantErr: "DB_POOL_MAX must be between 1 and 100",
		},
		{
			name:    "pool size that is not a number is refused",
			mutate:  func(env map[string]string) { env["DB_POOL_MAX"] = "ten" },
			wantErr: "DB_POOL_MAX must be an integer",
		},
		{
			name:    "unknown log level is refused",
			mutate:  func(env map[string]string) { env["LOG_LEVEL"] = "verbose" },
			wantErr: "LOG_LEVEL must be one of",
		},
		{
			name:    "http address without a port is refused",
			mutate:  func(env map[string]string) { env["HTTP_ADDR"] = "127.0.0.1" },
			wantErr: "HTTP_ADDR must be host:port",
		},
		{
			name:    "missing bot token is refused",
			mutate:  func(env map[string]string) { delete(env, "TELEGRAM_BOT_TOKEN") },
			wantErr: "TELEGRAM_BOT_TOKEN is required",
		},
		{
			name:    "token without a colon is refused",
			mutate:  func(env map[string]string) { env["TELEGRAM_BOT_TOKEN"] = "not-a-bot-token-value" },
			wantErr: "TELEGRAM_BOT_TOKEN is not a bot token",
		},
		{
			name:    "missing webhook secret is refused",
			mutate:  func(env map[string]string) { delete(env, "TELEGRAM_WEBHOOK_SECRET") },
			wantErr: "TELEGRAM_WEBHOOK_SECRET is required",
		},
		{
			name:    "short webhook secret is refused",
			mutate:  func(env map[string]string) { env["TELEGRAM_WEBHOOK_SECRET"] = "short" },
			wantErr: "TELEGRAM_WEBHOOK_SECRET must be 16 to 256 characters",
		},
		{
			name:    "webhook secret with a space is refused",
			mutate:  func(env map[string]string) { env["TELEGRAM_WEBHOOK_SECRET"] = "bad secret value 123" },
			wantErr: "TELEGRAM_WEBHOOK_SECRET must be 16 to 256 characters",
		},
		{
			name:    "plain http api base is refused",
			mutate:  func(env map[string]string) { env["TELEGRAM_API_BASE"] = "http://api.telegram.org" },
			wantErr: "TELEGRAM_API_BASE must be an https URL",
		},
		{
			name:    "api base with a path is refused",
			mutate:  func(env map[string]string) { env["TELEGRAM_API_BASE"] = "https://api.telegram.org/bot" },
			wantErr: "TELEGRAM_API_BASE must not carry a path",
		},
		{
			name:    "api base with credentials is refused",
			mutate:  func(env map[string]string) { env["TELEGRAM_API_BASE"] = "https://user:pass@api.telegram.org" },
			wantErr: "TELEGRAM_API_BASE must be an https URL",
		},
		{
			name:    "webhook secret with a tiny alphabet is refused",
			mutate:  func(env map[string]string) { env["TELEGRAM_WEBHOOK_SECRET"] = "aaaaaaaaaaaaaaaaaaaa" },
			wantErr: "must use at least 8 distinct characters",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := make(map[string]string, len(base))
			for key, value := range base {
				env[key] = value
			}
			tc.mutate(env)

			cfg, err := config.Load(func(key string) string { return env[key] })

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, want error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Load() error = %q, want it to contain %q", err.Error(), tc.wantErr)
				}
				if strings.Contains(err.Error(), "secret") {
					t.Fatalf("Load() error leaked a credential: %q", err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if cfg.DBPoolMax != 10 {
				t.Fatalf("DBPoolMax = %d, want 10", cfg.DBPoolMax)
			}
			if cfg.ReadinessTimeout.String() != "3s" {
				t.Fatalf("ReadinessTimeout = %s, want 3s", cfg.ReadinessTimeout)
			}
			if cfg.ShutdownTimeout.String() != "10s" {
				t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
			}
			if cfg.RedisAddr != "127.0.0.1:6379" {
				t.Fatalf("RedisAddr = %q, want 127.0.0.1:6379", cfg.RedisAddr)
			}
			if cfg.TelegramAPIBase != "https://api.telegram.org" {
				t.Fatalf("TelegramAPIBase = %q, want the default", cfg.TelegramAPIBase)
			}
		})
	}
}

func TestLoad_GivenOptionalOverrides_ThenValuesAreApplied(t *testing.T) {
	env := map[string]string{
		"APP_ENV":                 "staging",
		"HTTP_ADDR":               "127.0.0.1:9090",
		"DATABASE_URL":            "postgres://bot@127.0.0.1:5432/pannelai",
		"DB_POOL_MAX":             "4",
		"REDIS_ADDR":              "127.0.0.1:6380",
		"REDIS_PASSWORD":          "redis-secret",
		"LOG_LEVEL":               "debug",
		"READINESS_TIMEOUT":       "5s",
		"SHUTDOWN_TIMEOUT":        "20s",
		"TELEGRAM_BOT_TOKEN":      "123456789:AAF-test-token-0000000000000000000",
		"TELEGRAM_WEBHOOK_SECRET": "test-secret-1234567890",
	}

	cfg, err := config.Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.AppEnv != "staging" || cfg.HTTPAddr != "127.0.0.1:9090" || cfg.DBPoolMax != 4 {
		t.Fatalf("Load() = %+v, want the overridden values", cfg)
	}
	if cfg.ReadinessTimeout.String() != "5s" || cfg.ShutdownTimeout.String() != "20s" {
		t.Fatalf("timeouts = %s/%s, want 5s/20s", cfg.ReadinessTimeout, cfg.ShutdownTimeout)
	}
}
