// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config.go
// @for       Typed environment configuration for app-serv, validated once at boot.
// @uses      os, strconv, time (standard library only).
// @reason    SPEC-API-001 §4 and AGENTS.md §1.4 require env vars to become a
//
//	typed Config with fail-fast validation, so no raw os.Getenv()
//	reaches business logic.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-16
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every runtime setting for app-serv. It is loaded once, validated,
// and then immutable for the process lifetime (AGENTS.md §1.4).
type Config struct {
	AppEnv            string
	HTTPAddr          string
	PostgresDSN       string
	RedisAddr         string
	RedisPassword     string
	DBPoolMax         int
	LogLevel          string
	SessionSecret     string
	SessionTTL        time.Duration
	EncryptionKey     string
	BootstrapPassword string
	LoginMaxFails     int
	LoginLockout      time.Duration
	GatewayKeyPrefix  string
	RateLimitPerMin   int
	// DataPlaneStickyLimit is how many consecutive requests one upstream
	// endpoint serves before round-robin rotation moves on (SPEC-API-001
	// §4 settings.routing.sticky_limit, whose default is 3).
	DataPlaneStickyLimit int
}

// Load reads the environment and returns a validated Config, or an error naming
// the first missing or malformed required value. A boot that cannot satisfy
// these constraints fails rather than serving degraded.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:            getenv("APP_ENV", "development"),
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		PostgresDSN:       getenv("POSTGRES_DSN", ""),
		RedisAddr:         getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getenv("REDIS_PASSWORD", ""),
		LogLevel:          getenv("LOG_LEVEL", "info"),
		SessionSecret:     getenv("SESSION_SECRET", ""),
		EncryptionKey:     getenv("ENCRYPTION_KEY", ""),
		BootstrapPassword: getenv("PANEL_BOOTSTRAP_PASSWORD", ""),
		GatewayKeyPrefix:  getenv("GATEWAY_KEY_PREFIX", "sk-"),
	}

	intFields := []struct {
		name string
		dst  *int
		def  int
		max  int
	}{
		{"DB_POOL_MAX", &cfg.DBPoolMax, 10, 100},
		{"LOGIN_MAX_FAILS", &cfg.LoginMaxFails, 5, 100},
		{"RATE_LIMIT_PER_MIN", &cfg.RateLimitPerMin, 120, 10000},
		{"DATA_PLANE_STICKY_LIMIT", &cfg.DataPlaneStickyLimit, 3, 100},
	}
	for _, f := range intFields {
		v, err := getenvInt(f.name, f.def)
		if err != nil {
			return Config{}, fmt.Errorf("config: %s: %w", f.name, err)
		}
		*f.dst = v
	}

	durFields := []struct {
		name string
		dst  *time.Duration
		def  time.Duration
	}{
		{"SESSION_TTL", &cfg.SessionTTL, 24 * time.Hour},
		{"LOGIN_LOCKOUT", &cfg.LoginLockout, 15 * time.Minute},
	}
	for _, f := range durFields {
		v, err := getenvDuration(f.name, f.def)
		if err != nil {
			return Config{}, fmt.Errorf("config: %s: %w", f.name, err)
		}
		*f.dst = v
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// IsProduction reports whether APP_ENV is a production profile.
func (c Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// validate enforces required secrets and value ranges.
func (c Config) validate() error {
	var problems []string

	if c.HTTPAddr == "" {
		problems = append(problems, "HTTP_ADDR is required")
	}
	if c.PostgresDSN == "" {
		problems = append(problems, "POSTGRES_DSN is required")
	}
	if c.RedisAddr == "" {
		problems = append(problems, "REDIS_ADDR is required")
	}
	if len(c.SessionSecret) < 32 {
		problems = append(problems, "SESSION_SECRET must be at least 32 bytes")
	}
	if len(c.EncryptionKey) != 32 {
		problems = append(problems, "ENCRYPTION_KEY must be exactly 32 bytes (AES-256)")
	}
	if len([]byte(c.BootstrapPassword)) > 72 {
		problems = append(problems, "PANEL_BOOTSTRAP_PASSWORD must be at most 72 bytes")
	}
	if c.DBPoolMax < 1 || c.DBPoolMax > 100 {
		problems = append(problems, "DB_POOL_MAX must be between 1 and 100")
	}
	if c.SessionTTL < time.Minute || c.SessionTTL > 30*24*time.Hour {
		problems = append(problems, "SESSION_TTL must be between 1m and 720h")
	}
	if c.LoginMaxFails < 1 {
		problems = append(problems, "LOGIN_MAX_FAILS must be >= 1")
	}
	if c.LoginLockout < time.Minute {
		problems = append(problems, "LOGIN_LOCKOUT must be at least 1m")
	}
	if c.RateLimitPerMin < 1 {
		problems = append(problems, "RATE_LIMIT_PER_MIN must be >= 1")
	}
	if c.DataPlaneStickyLimit < 1 {
		problems = append(problems, "DATA_PLANE_STICKY_LIMIT must be >= 1")
	}
	if !isValidLogLevel(c.LogLevel) {
		problems = append(problems, "LOG_LEVEL must be one of debug, info, warn, error")
	}

	if len(problems) > 0 {
		return fmt.Errorf("config: invalid: %s", strings.Join(problems, "; "))
	}
	return nil
}

// getenv returns the variable's value when the variable is set, even when that
// value is empty, and the fallback only when it is unset. Collapsing the two
// cases would make an explicitly empty HTTP_ADDR silently become ":8080",
// which is precisely the misconfiguration the boot validation exists to catch.
// isValidLogLevel reports whether the level names one slog supports. Validating
// here means a misspelled LOG_LEVEL is a boot failure rather than a value the
// logger silently ignores.
func isValidLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func getenv(key, fallback string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return v
}

// getenvInt reads an integer, using the fallback only when the variable is
// unset. A variable that is set but empty or malformed is an error: silently
// substituting a default would hide a broken deployment configuration.
func getenvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("not an integer: %q", v)
	}
	return n, nil
}

// getenvDuration reads a Go duration string with the same unset/empty rule.
func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("not a duration: %q", v)
	}
	return d, nil
}
