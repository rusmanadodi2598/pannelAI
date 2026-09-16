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

// validEnv is the smallest environment Load accepts. Each case overrides one
// variable so a failure points at exactly one rule.
func validEnv() map[string]string {
	return map[string]string{
		"POSTGRES_DSN":   "postgres://user:pass@localhost:5432/db",
		"SESSION_SECRET": strings.Repeat("a", 32),
		"ENCRYPTION_KEY": strings.Repeat("b", 32),
	}
}

// setEnv applies a map to the process environment for the duration of a test.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

// TestLoad_Defaults asserts the documented defaults are applied when a variable
// is absent, so a minimal environment still boots.
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
		{"AppEnv", cfg.AppEnv, "development"},
		{"HTTPAddr", cfg.HTTPAddr, ":8080"},
		{"RedisAddr", cfg.RedisAddr, "localhost:6379"},
		{"LogLevel", cfg.LogLevel, "info"},
		{"GatewayKeyPrefix", cfg.GatewayKeyPrefix, "sk-"},
		{"DBPoolMax", cfg.DBPoolMax, 10},
		{"SessionTTL", cfg.SessionTTL, 24 * time.Hour},
		{"LoginMaxFails", cfg.LoginMaxFails, 5},
		{"LoginLockout", cfg.LoginLockout, 15 * time.Minute},
		{"RateLimitPerMin", cfg.RateLimitPerMin, 120},
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

// TestLoad_Validation walks every rejection rule: a boot must fail rather than
// serve with a value it cannot honour.
func TestLoad_Validation(t *testing.T) {
	cases := []struct {
		name     string
		override map[string]string
		// unset removes a variable, exercising the "absent" path. An empty
		// string in override exercises the distinct "set but empty" path, which
		// must not be silently replaced by a default.
		unset   []string
		wantErr bool
	}{
		{"valid baseline", nil, nil, false},
		{"production profile", map[string]string{"APP_ENV": "production"}, nil, false},
		{"DSN absent", nil, []string{"POSTGRES_DSN"}, true},
		{"DSN set but empty", map[string]string{"POSTGRES_DSN": ""}, nil, true},
		{"redis addr set but empty", map[string]string{"REDIS_ADDR": ""}, nil, true},
		{"redis addr absent falls back", nil, []string{"REDIS_ADDR"}, false},
		{"http addr set but empty", map[string]string{"HTTP_ADDR": ""}, nil, true},
		{"http addr absent falls back", nil, []string{"HTTP_ADDR"}, false},
		{"session secret one byte short", map[string]string{"SESSION_SECRET": strings.Repeat("a", 31)}, nil, true},
		{"session secret exactly 32", map[string]string{"SESSION_SECRET": strings.Repeat("a", 32)}, nil, false},
		{"encryption key one byte short", map[string]string{"ENCRYPTION_KEY": strings.Repeat("b", 31)}, nil, true},
		{"encryption key one byte long", map[string]string{"ENCRYPTION_KEY": strings.Repeat("b", 33)}, nil, true},
		{"pool below range", map[string]string{"DB_POOL_MAX": "0"}, nil, true},
		{"pool at minimum", map[string]string{"DB_POOL_MAX": "1"}, nil, false},
		{"pool at maximum", map[string]string{"DB_POOL_MAX": "100"}, nil, false},
		{"pool above range", map[string]string{"DB_POOL_MAX": "101"}, nil, true},
		{"pool not an integer", map[string]string{"DB_POOL_MAX": "many"}, nil, true},
		{"pool set but empty", map[string]string{"DB_POOL_MAX": ""}, nil, true},
		{"session ttl set but empty", map[string]string{"SESSION_TTL": ""}, nil, true},
		{"session ttl below range", map[string]string{"SESSION_TTL": "30s"}, nil, true},
		{"session ttl at minimum", map[string]string{"SESSION_TTL": "1m"}, nil, false},
		{"session ttl at maximum", map[string]string{"SESSION_TTL": "720h"}, nil, false},
		{"session ttl above range", map[string]string{"SESSION_TTL": "800h"}, nil, true},
		{"session ttl not a duration", map[string]string{"SESSION_TTL": "one day"}, nil, true},
		{"login max fails zero", map[string]string{"LOGIN_MAX_FAILS": "0"}, nil, true},
		{"login lockout below range", map[string]string{"LOGIN_LOCKOUT": "30s"}, nil, true},
		{"rate limit zero", map[string]string{"RATE_LIMIT_PER_MIN": "0"}, nil, true},
		{"rate limit negative", map[string]string{"RATE_LIMIT_PER_MIN": "-5"}, nil, true},
		{"log level debug", map[string]string{"LOG_LEVEL": "debug"}, nil, false},
		{"log level warn", map[string]string{"LOG_LEVEL": "warn"}, nil, false},
		{"log level misspelled", map[string]string{"LOG_LEVEL": "verbose"}, nil, true},
		{"log level uppercase", map[string]string{"LOG_LEVEL": "INFO"}, nil, true},
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
				t.Fatal("Load() = nil error, want a rejection")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
		})
	}
}

// TestLoad_IsProduction checks the production profile is reported, since it
// changes cookie and error-detail behavior.
func TestLoad_IsProduction(t *testing.T) {
	cases := []struct {
		appEnv string
		want   bool
	}{
		{"production", true},
		{"development", false},
		{"staging", false},
		{"", false},
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
