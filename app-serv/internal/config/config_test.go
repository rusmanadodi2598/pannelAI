// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_test.go
// @for       Table-driven tests for environment parsing and range validation.
// @uses      testing, standard library only.
// @reason    AGENTS.md §2.1 requires tests alongside new config logic, and
//
//	§1.4 makes boot-time validation the only thing standing between a
//	malformed environment and a server that starts degraded. These are
//	exactly the boundaries a single smoke run cannot cover
//	(docs/RULLES/TDD.md §2.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-16
package config

import (
	"strings"
	"testing"
	"time"
)

func validEnv() map[string]string {
	return map[string]string{
		"POSTGRES_DSN":   "postgres://user:pass@localhost:5432/db",
		"SESSION_SECRET": strings.Repeat("a", 32),
		"ENCRYPTION_KEY": strings.Repeat("b", 32),
	}
}

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, validEnv())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cases := []struct {
		name string
		got  any
		want any
	}{
		{"AppEnv", cfg.AppEnv, "development"}, {"HTTPAddr", cfg.HTTPAddr, ":8080"},
		{"RedisAddr", cfg.RedisAddr, "localhost:6379"}, {"LogLevel", cfg.LogLevel, "info"},
		{"GatewayKeyPrefix", cfg.GatewayKeyPrefix, "sk-"}, {"DBPoolMax", cfg.DBPoolMax, 10},
		{"SessionTTL", cfg.SessionTTL, 24 * time.Hour}, {"LoginMaxFails", cfg.LoginMaxFails, 5},
		{"LoginLockout", cfg.LoginLockout, 15 * time.Minute}, {"RateLimitPerMin", cfg.RateLimitPerMin, 120},
		{"BootstrapPassword", cfg.BootstrapPassword, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s = %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}
	if cfg.IsProduction() {
		t.Fatal("IsProduction() = true for the development default")
	}
}

func TestLoad_Validation(t *testing.T) {
	cases := []struct {
		name     string
		override map[string]string
		unset    []string
		wantErr  bool
	}{
		{"valid baseline", nil, nil, false}, {"production profile", map[string]string{"APP_ENV": "production"}, nil, false},
		{"DSN absent", nil, []string{"POSTGRES_DSN"}, true}, {"DSN empty", map[string]string{"POSTGRES_DSN": ""}, nil, true},
		{"redis empty", map[string]string{"REDIS_ADDR": ""}, nil, true}, {"redis absent", nil, []string{"REDIS_ADDR"}, false},
		{"http empty", map[string]string{"HTTP_ADDR": ""}, nil, true}, {"http absent", nil, []string{"HTTP_ADDR"}, false},
		{"session short", map[string]string{"SESSION_SECRET": strings.Repeat("a", 31)}, nil, true},
		{"session minimum", map[string]string{"SESSION_SECRET": strings.Repeat("a", 32)}, nil, false},
		{"encryption short", map[string]string{"ENCRYPTION_KEY": strings.Repeat("b", 31)}, nil, true},
		{"encryption long", map[string]string{"ENCRYPTION_KEY": strings.Repeat("b", 33)}, nil, true},
		{"pool zero", map[string]string{"DB_POOL_MAX": "0"}, nil, true}, {"pool minimum", map[string]string{"DB_POOL_MAX": "1"}, nil, false},
		{"pool maximum", map[string]string{"DB_POOL_MAX": "100"}, nil, false}, {"pool above", map[string]string{"DB_POOL_MAX": "101"}, nil, true},
		{"pool malformed", map[string]string{"DB_POOL_MAX": "many"}, nil, true}, {"pool empty", map[string]string{"DB_POOL_MAX": ""}, nil, true},
		{"ttl empty", map[string]string{"SESSION_TTL": ""}, nil, true}, {"ttl below", map[string]string{"SESSION_TTL": "30s"}, nil, true},
		{"ttl minimum", map[string]string{"SESSION_TTL": "1m"}, nil, false}, {"ttl maximum", map[string]string{"SESSION_TTL": "720h"}, nil, false},
		{"ttl above", map[string]string{"SESSION_TTL": "800h"}, nil, true}, {"ttl malformed", map[string]string{"SESSION_TTL": "one day"}, nil, true},
		{"login fails zero", map[string]string{"LOGIN_MAX_FAILS": "0"}, nil, true}, {"lockout below", map[string]string{"LOGIN_LOCKOUT": "30s"}, nil, true},
		{"rate zero", map[string]string{"RATE_LIMIT_PER_MIN": "0"}, nil, true}, {"rate negative", map[string]string{"RATE_LIMIT_PER_MIN": "-5"}, nil, true},
		{"log debug", map[string]string{"LOG_LEVEL": "debug"}, nil, false}, {"log warn", map[string]string{"LOG_LEVEL": "warn"}, nil, false},
		{"log misspelled", map[string]string{"LOG_LEVEL": "verbose"}, nil, true}, {"log uppercase", map[string]string{"LOG_LEVEL": "INFO"}, nil, true},
		{"bootstrap maximum", map[string]string{"PANEL_BOOTSTRAP_PASSWORD": strings.Repeat("p", 72)}, nil, false},
		{"bootstrap too long", map[string]string{"PANEL_BOOTSTRAP_PASSWORD": strings.Repeat("p", 73)}, nil, true},
		{"public base URL absent", nil, []string{"PUBLIC_BASE_URL"}, false},
		{"public base URL empty", map[string]string{"PUBLIC_BASE_URL": ""}, nil, false},
		{"public base URL https", map[string]string{"PUBLIC_BASE_URL": "https://gateway.example.com"}, nil, false},
		{"public base URL http on a port", map[string]string{"PUBLIC_BASE_URL": "http://localhost:8080"}, nil, false},
		{"public base URL without a scheme", map[string]string{"PUBLIC_BASE_URL": "gateway.example.com"}, nil, true},
		{"public base URL with a bad scheme", map[string]string{"PUBLIC_BASE_URL": "ftp://gateway.example.com"}, nil, true},
		{"public base URL relative", map[string]string{"PUBLIC_BASE_URL": "/api/v1"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			for _, k := range tc.unset {
				delete(env, k)
			}
			for k, v := range tc.override {
				env[k] = v
			}
			setEnv(t, env)
			_, err := Load()
			if tc.wantErr && err == nil {
				t.Fatal("Load() = nil error, want rejection")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
		})
	}
}

func TestLoad_IsProduction(t *testing.T) {
	cases := []struct {
		appEnv string
		want   bool
	}{
		{"production", true}, {"development", false}, {"staging", false}, {"", false},
	}
	for _, tc := range cases {
		t.Run(tc.appEnv, func(t *testing.T) {
			env := validEnv()
			if tc.appEnv != "" {
				env["APP_ENV"] = tc.appEnv
			}
			setEnv(t, env)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.IsProduction() != tc.want {
				t.Fatalf("IsProduction() = %v, want %v", cfg.IsProduction(), tc.want)
			}
		})
	}
}
