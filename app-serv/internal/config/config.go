// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config.go
// @for       Typed environment configuration for app-serv, validated once at boot.
// @uses      fmt, net, time (the env-reading helpers are in env.go, the range checks in config_validate.go).
// @reason    SPEC-API-001 §4 and AGENTS.md §1.4 require env vars to become a typed Config with fail-fast validation, so no raw os.Getenv() reaches business logic.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-16
package config

import (
	"fmt"
	"net"
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
	RedisPoolSize     int
	RedisDialTimeout  time.Duration
	RedisReadTimeout  time.Duration
	RedisWriteTimeout time.Duration
	DBPoolMax         int
	LogLevel          string
	SessionSecret     string
	SessionTTL        time.Duration
	// SessionCookieInsecure opts a development deployment out of the Secure
	// cookie flag; validate refuses it under APP_ENV=production.
	SessionCookieInsecure bool
	EncryptionKey         string
	BootstrapPassword     string
	LoginMaxFails         int
	LoginLockout          time.Duration
	GatewayKeyPrefix      string
	RateLimitPerMin       int
	// RequestLogMaxRows bounds request_logs between the two-day retention runs.
	RequestLogMaxRows int
	// UsageRetentionDays bounds usage_records, which has no API purge route.
	UsageRetentionDays int
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
	// OAuthClients is the per-provider OAuth client material the deployment
	// supplies, keyed by registry provider id. A provider absent from it keeps
	// the client id its registry entry declares and has no client secret at all,
	// which is why the secret is not stored in the generated registry document.
	OAuthClients map[string]OAuthClient
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
	// its real client. Empty, the default, means no proxy is trusted and every
	// request is bucketed by its direct peer.
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
	cfg.OAuthClients = loadOAuthClients()

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
		{"REDIS_POOL_SIZE", &cfg.RedisPoolSize, 20, 200},
		{"LOGIN_MAX_FAILS", &cfg.LoginMaxFails, 5, 100},
		{"RATE_LIMIT_PER_MIN", &cfg.RateLimitPerMin, 120, 10000},
		{"REQUEST_LOG_MAX_ROWS", &cfg.RequestLogMaxRows, 20000, 1000000},
		{"USAGE_RETENTION_DAYS", &cfg.UsageRetentionDays, 90, 3650},
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
		{"REDIS_DIAL_TIMEOUT", &cfg.RedisDialTimeout, 5 * time.Second},
		{"REDIS_READ_TIMEOUT", &cfg.RedisReadTimeout, 3 * time.Second},
		{"REDIS_WRITE_TIMEOUT", &cfg.RedisWriteTimeout, 3 * time.Second},
	}
	for _, f := range durFields {
		v, err := getenvDuration(f.name, f.def)
		if err != nil {
			return Config{}, fmt.Errorf("config: %s: %w", f.name, err)
		}
		*f.dst = v
	}

	insecureCookie, err := getenvBool("SESSION_COOKIE_INSECURE", false)
	if err != nil {
		return Config{}, fmt.Errorf("config: SESSION_COOKIE_INSECURE: %w", err)
	}
	cfg.SessionCookieInsecure = insecureCookie

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// IsProduction reports whether APP_ENV is a production profile.
func (c Config) IsProduction() bool {
	return c.AppEnv == "production"
}
