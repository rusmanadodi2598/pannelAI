// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config.go
// @for       Typed environment configuration for app-bot, validated once at boot.
// @uses      fmt, net, net/url, regexp, strconv, strings, time.
// @reason    AGENTS.md §1.4 requires env vars to become a typed Config with fail-fast validation, so no raw os.Getenv reaches business logic and a missing value stops the process with one message.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-21
package config

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr         = "127.0.0.1:8090"
	defaultRedisAddr        = "127.0.0.1:6379"
	defaultLogLevel         = "info"
	defaultPoolMax          = 10
	defaultReadinessTimeout = 3 * time.Second
	defaultShutdownTimeout  = 10 * time.Second
	defaultTelegramAPIBase  = "https://api.telegram.org"
)

var allowedEnvs = map[string]bool{"development": true, "staging": true, "production": true}

var allowedLogLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

// Config is immutable after Load.
type Config struct {
	AppEnv           string
	HTTPAddr         string
	DatabaseURL      string
	DBPoolMax        int32
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	LogLevel         string
	ReadinessTimeout time.Duration
	ShutdownTimeout  time.Duration

	TelegramBotToken      string
	TelegramWebhookSecret string
	TelegramAPIBase       string

	GatewayBaseURL   string
	GatewayKey       string
	BuilderModel     string
	BuilderMaxTokens int
	BuilderTimeout   time.Duration
	BuilderDailyCap  int
	PromptMaxChars   int
}

// Load reads every setting through getenv, applies defaults, and validates the
// whole set before returning. The error names the field, never its value.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		AppEnv:           strings.TrimSpace(getenv("APP_ENV")),
		HTTPAddr:         valueOr(getenv, "HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL:      strings.TrimSpace(getenv("DATABASE_URL")),
		RedisAddr:        valueOr(getenv, "REDIS_ADDR", defaultRedisAddr),
		RedisPassword:    getenv("REDIS_PASSWORD"),
		LogLevel:         strings.ToLower(valueOr(getenv, "LOG_LEVEL", defaultLogLevel)),
		ReadinessTimeout: defaultReadinessTimeout,
		ShutdownTimeout:  defaultShutdownTimeout,

		TelegramBotToken:      strings.TrimSpace(getenv("TELEGRAM_BOT_TOKEN")),
		TelegramWebhookSecret: strings.TrimSpace(getenv("TELEGRAM_WEBHOOK_SECRET")),
		TelegramAPIBase:       strings.TrimRight(valueOr(getenv, "TELEGRAM_API_BASE", defaultTelegramAPIBase), "/"),

		GatewayBaseURL:   strings.TrimRight(valueOr(getenv, "GATEWAY_BASE_URL", ""), "/"),
		GatewayKey:       strings.TrimSpace(getenv("GATEWAY_KEY")),
		BuilderModel:     strings.TrimSpace(getenv("BUILDER_MODEL")),
		BuilderTimeout:   defaultBuilderTimeout,
		BuilderMaxTokens: defaultBuilderMaxTokens,
		BuilderDailyCap:  defaultBuilderDailyCap,
		PromptMaxChars:   defaultPromptMaxChars,
	}

	if !allowedEnvs[cfg.AppEnv] {
		return Config{}, fmt.Errorf("APP_ENV must be one of development, staging, production")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "postgres://") && !strings.HasPrefix(cfg.DatabaseURL, "postgresql://") {
		return Config{}, fmt.Errorf("DATABASE_URL must be a postgres URL")
	}
	if err := requireHostPort("HTTP_ADDR", cfg.HTTPAddr); err != nil {
		return Config{}, err
	}
	if err := requireHostPort("REDIS_ADDR", cfg.RedisAddr); err != nil {
		return Config{}, err
	}
	if !allowedLogLevels[cfg.LogLevel] {
		return Config{}, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}
	if cfg.AppEnv == "production" && cfg.LogLevel == "debug" {
		return Config{}, fmt.Errorf("LOG_LEVEL debug is refused in production")
	}

	poolMax, err := parseInt("DB_POOL_MAX", valueOr(getenv, "DB_POOL_MAX", strconv.Itoa(defaultPoolMax)), 1, 100)
	if err != nil {
		return Config{}, err
	}
	cfg.DBPoolMax = int32(poolMax)

	redisDB, err := parseInt("REDIS_DB", valueOr(getenv, "REDIS_DB", "0"), 0, 15)
	if err != nil {
		return Config{}, err
	}
	cfg.RedisDB = redisDB

	if err := requireTelegram(cfg); err != nil {
		return Config{}, err
	}

	if cfg.ReadinessTimeout, err = parseDuration("READINESS_TIMEOUT", getenv("READINESS_TIMEOUT"), cfg.ReadinessTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = parseDuration("SHUTDOWN_TIMEOUT", getenv("SHUTDOWN_TIMEOUT"), cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.BuilderTimeout, err = parseDuration("BUILDER_TIMEOUT", getenv("BUILDER_TIMEOUT"), cfg.BuilderTimeout); err != nil {
		return Config{}, err
	}
	if cfg.BuilderMaxTokens, err = parseInt("BUILDER_MAX_TOKENS", valueOr(getenv, "BUILDER_MAX_TOKENS", strconv.Itoa(defaultBuilderMaxTokens)), 1, 8000); err != nil {
		return Config{}, err
	}
	if cfg.BuilderDailyCap, err = parseInt("BUILDER_DAILY_CAP", valueOr(getenv, "BUILDER_DAILY_CAP", strconv.Itoa(defaultBuilderDailyCap)), 1, 10000); err != nil {
		return Config{}, err
	}
	if cfg.PromptMaxChars, err = parseInt("PROMPT_MAX_CHARS", valueOr(getenv, "PROMPT_MAX_CHARS", strconv.Itoa(defaultPromptMaxChars)), 100, 4000); err != nil {
		return Config{}, err
	}

	if err := requireBuilder(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func valueOr(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func requireHostPort(field, value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil || host == "" || port == "" {
		return fmt.Errorf("%s must be host:port", field)
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("%s must be host:port with a numeric port", field)
	}
	return nil
}

func parseInt(field, raw string, min, max int) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	if value < min || value > max {
		return 0, fmt.Errorf("%s must be between %d and %d", field, min, max)
	}
	return value, nil
}

func parseDuration(field, raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", field)
	}
	return value, nil
}

var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,256}$`)

// requireTelegram validates the Bot API material without echoing either
// value, so a boot failure is safe to paste into a ticket.
func requireTelegram(cfg Config) error {
	if cfg.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if !strings.Contains(cfg.TelegramBotToken, ":") || len(cfg.TelegramBotToken) < 20 {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is not a bot token")
	}
	if cfg.TelegramWebhookSecret == "" {
		return fmt.Errorf("TELEGRAM_WEBHOOK_SECRET is required")
	}
	if !webhookSecretPattern.MatchString(cfg.TelegramWebhookSecret) {
		return fmt.Errorf("TELEGRAM_WEBHOOK_SECRET must be 16 to 256 characters of A-Z, a-z, 0-9, _ or -")
	}
	if distinctChars(cfg.TelegramWebhookSecret) < minSecretAlphabet {
		return fmt.Errorf("TELEGRAM_WEBHOOK_SECRET must use at least %d distinct characters", minSecretAlphabet)
	}
	if err := validateAPIBase(cfg.TelegramAPIBase); err != nil {
		return err
	}
	return nil
}

// minSecretAlphabet rejects a long but trivial secret such as aaaaaaaaaaaaaaaa.
const minSecretAlphabet = 8

func distinctChars(value string) int {
	seen := make(map[rune]struct{}, len(value))
	for _, char := range value {
		seen[char] = struct{}{}
	}
	return len(seen)
}

// validateAPIBase refuses a base that would send the bot token somewhere the
// operator did not mean, including a bare scheme or a path suffix.
func validateAPIBase(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("TELEGRAM_API_BASE is not a URL")
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("TELEGRAM_API_BASE must be an https URL with a host and no user info, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("TELEGRAM_API_BASE must not carry a path")
	}
	return nil
}
