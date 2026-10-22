// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_redis_test.go
// @for       The Redis pool and timeout knobs, their defaults, and their bounds.
// @uses      testing, time.
// @reason    AGENTS.md §1.7 forbids leaving a pool at the library default, so the values that replace go-redis defaults are pinned at boot rather than discovered under load.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-22
package config

import (
	"testing"
	"time"
)

func TestLoad_RedisDefaults(t *testing.T) {
	setEnv(t, validEnv())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RedisPoolSize != 20 {
		t.Fatalf("RedisPoolSize = %d, want 20", cfg.RedisPoolSize)
	}
	if cfg.RedisDialTimeout != 5*time.Second || cfg.RedisReadTimeout != 3*time.Second ||
		cfg.RedisWriteTimeout != 3*time.Second {
		t.Fatalf("Redis timeouts = %v/%v/%v, want 5s/3s/3s",
			cfg.RedisDialTimeout, cfg.RedisReadTimeout, cfg.RedisWriteTimeout)
	}
}

func TestLoad_RedisOverrides(t *testing.T) {
	env := validEnv()
	env["REDIS_POOL_SIZE"] = "7"
	env["REDIS_DIAL_TIMEOUT"] = "2s"
	env["REDIS_READ_TIMEOUT"] = "1500ms"
	env["REDIS_WRITE_TIMEOUT"] = "4s"
	setEnv(t, env)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RedisPoolSize != 7 || cfg.RedisDialTimeout != 2*time.Second ||
		cfg.RedisReadTimeout != 1500*time.Millisecond || cfg.RedisWriteTimeout != 4*time.Second {
		t.Fatalf("Redis config = %d/%v/%v/%v, want 7/2s/1.5s/4s",
			cfg.RedisPoolSize, cfg.RedisDialTimeout, cfg.RedisReadTimeout, cfg.RedisWriteTimeout)
	}
}

func TestLoad_RejectsRedisBounds(t *testing.T) {
	cases := map[string]string{
		"REDIS_POOL_SIZE":     "0",
		"REDIS_DIAL_TIMEOUT":  "50ms",
		"REDIS_READ_TIMEOUT":  "31s",
		"REDIS_WRITE_TIMEOUT": "0s",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env[name] = value
			setEnv(t, env)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted %s=%s", name, value)
			}
		})
	}
}
