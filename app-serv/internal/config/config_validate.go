// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_validate.go
// @for       The boot-time bounds and required secrets checked before the process serves.
// @uses      fmt, strings, time.
// @reason    A malformed environment must fail before the server listens, and the checks stay together so a new setting cannot be parsed without a decision about its range.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-22
package config

import (
	"fmt"
	"strings"
	"time"
)

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
	if c.RedisPoolSize < 1 {
		problems = append(problems, "REDIS_POOL_SIZE must be at least 1")
	}
	if c.RedisDialTimeout < 100*time.Millisecond || c.RedisDialTimeout > 30*time.Second {
		problems = append(problems, "REDIS_DIAL_TIMEOUT must be between 100ms and 30s")
	}
	if c.RedisReadTimeout < 100*time.Millisecond || c.RedisReadTimeout > 30*time.Second {
		problems = append(problems, "REDIS_READ_TIMEOUT must be between 100ms and 30s")
	}
	if c.RedisWriteTimeout < 100*time.Millisecond || c.RedisWriteTimeout > 30*time.Second {
		problems = append(problems, "REDIS_WRITE_TIMEOUT must be between 100ms and 30s")
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
	if c.RequestLogMaxRows < 1 {
		problems = append(problems, "REQUEST_LOG_MAX_ROWS must be >= 1")
	}
	if c.UsageRetentionDays < 1 {
		problems = append(problems, "USAGE_RETENTION_DAYS must be >= 1")
	}
	if c.QuotaPollBudget < 1 || c.QuotaPollBudget > 500 {
		problems = append(problems, "QUOTA_POLL_BUDGET must be between 1 and 500")
	}
	if c.QuotaPollConcurrency < 1 || c.QuotaPollConcurrency > 16 {
		problems = append(problems, "QUOTA_POLL_CONCURRENCY must be between 1 and 16")
	}
	if c.PublicBaseURL != "" && !isAbsoluteHTTPURL(c.PublicBaseURL) {
		problems = append(problems, "PUBLIC_BASE_URL must be an absolute http(s) URL, e.g. https://gateway.example.com")
	}
	if !isAbsoluteHTTPURL(c.ProxyTestURL) {
		problems = append(problems, "PROXY_TEST_URL must be an absolute http(s) URL")
	}
	if !isValidLogLevel(c.LogLevel) {
		problems = append(problems, "LOG_LEVEL must be one of debug, info, warn, error")
	}

	if len(problems) > 0 {
		return fmt.Errorf("config: invalid: %s", strings.Join(problems, "; "))
	}
	return nil
}
