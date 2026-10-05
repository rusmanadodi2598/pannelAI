// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config.go
// @for       Typed environment configuration for app-serv, validated once at boot.
// @uses      fmt, strings, time (the env-reading helpers are in env.go).
// @reason    SPEC-API-001 §4 and AGENTS.md §1.4 require env vars to become a
//
//	typed Config with fail-fast validation, so no raw os.Getenv()
//	reaches business logic.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-16
package config

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/clientip"
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
	// QuotaPollBudget and QuotaPollConcurrency are the operator's two knobs on the
	// published-quota sweep: how many accounts one tick may ask, and how many of those
	// calls may be in flight. They are configuration because the right number depends on
	// how many accounts a deployment holds and on how tolerant each provider is of being
	// asked, which is something the operator knows and the binary does not.
	QuotaPollBudget      int
	QuotaPollConcurrency int
	// PublicBaseURL is the absolute URL this gateway answers on, when it is
	// deployed behind a proxy or on a hostname the request's Host header does
	// not name. SPEC-API-001 §7.4 needs it to build the OAuth callback URL and
	// to send a browser back to the panel after an authorization; leaving it
	// empty makes the callback refuse to redirect rather than trust a header.
	PublicBaseURL string
	// EgressAllowedTargets is the operator's allowlist of outbound
	// destinations, as CIDR prefixes or single addresses. The egress guard
	// (internal/netguard) refuses loopback and private ranges unless they
	// appear here, so a self-hosted proxy on the operator's own network is a
	// deliberate opt-in rather than a default (OWASP A01).
	EgressAllowedTargets []string
	// ProxyTestURL is the URL the proxy connectivity test fetches through the
	// candidate. It is server configuration rather than a request field on
	// purpose: a client-supplied test URL would be an SSRF seam, and the value
	// only has to be something a working proxy can reach.
	ProxyTestURL string
	// TrustedProxies is the parsed TRUSTED_PROXY_CIDRS list: the peers whose
	// X-Forwarded-For chains the rate limiters may read to bucket a request by
	// its real client (draft 042 R20). Empty, the default, means no proxy is
	// trusted and every request is bucketed by its direct peer.
	TrustedProxies []*net.IPNet
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
		PublicBaseURL:     getenv("PUBLIC_BASE_URL", ""),
		ProxyTestURL:      getenv("PROXY_TEST_URL", "https://www.google.com/"),
	}
	cfg.EgressAllowedTargets = splitList(getenv("EGRESS_ALLOWED_TARGETS", ""))

	trusted, err := clientip.ParseTrusted(splitList(getenv("TRUSTED_PROXY_CIDRS", "")))
	if err != nil {
		return Config{}, fmt.Errorf("config: TRUSTED_PROXY_CIDRS: %w", err)
	}
	cfg.TrustedProxies = trusted

	intFields := []struct {
		name string
		dst  *int
		def  int
		max  int
	}{
		{"DB_POOL_MAX", &cfg.DBPoolMax, 10, 100},
		{"LOGIN_MAX_FAILS", &cfg.LoginMaxFails, 5, 100},
		{"RATE_LIMIT_PER_MIN", &cfg.RateLimitPerMin, 120, 10000},
		{"QUOTA_POLL_BUDGET", &cfg.QuotaPollBudget, 40, 500},
		{"QUOTA_POLL_CONCURRENCY", &cfg.QuotaPollConcurrency, 4, 16},
	}
	for _, f := range intFields {
		v, err := getenvInt(f.name, f.def)
		if err != nil {
			return Config{}, fmt.Errorf("config: %s: %w", f.name, err)
		}
		// The ceiling is part of the declaration, so it is enforced here rather
		// than left to validate(): a bound nothing reads is worse than no bound,
		// because the table reads as if RATE_LIMIT_PER_MIN could not be set to a
		// value that disables the limiter.
		if v > f.max {
			return Config{}, fmt.Errorf("config: %s: %d exceeds the ceiling of %d", f.name, v, f.max)
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
