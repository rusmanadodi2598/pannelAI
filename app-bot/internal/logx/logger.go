// Package logx writes structured logs with credential redaction.
//
// @file      internal/logx/logger.go
// @for       The process logger and its attribute redaction policy.
// @uses      io, log/slog, strings.
// @reason    OWASP A09 requires security-relevant logs, and the same lines must never carry a token; one constructor keeps the redaction rule from being reimplemented per package.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package logx

import (
	"io"
	"log/slog"
	"strings"
)

const serviceName = "app-bot"

var sensitiveFragments = []string{
	"token",
	"secret",
	"password",
	"authorization",
	"signature",
	"api_key",
	"apikey",
}

// New returns a JSON logger tagged with the service and environment. The writer
// is injectable so tests assert the exact line that ships.
func New(level, env string, writer io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:       parseLevel(level),
		ReplaceAttr: redact,
	})
	return slog.New(handler).With("service", serviceName, "env", env)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func redact(_ []string, attr slog.Attr) slog.Attr {
	if isSensitive(attr.Key) {
		return slog.String(attr.Key, "[REDACTED]")
	}
	if attr.Value.Kind() == slog.KindGroup {
		attrs := attr.Value.Group()
		for i := range attrs {
			attrs[i] = redact(nil, attrs[i])
		}
		return slog.Group(attr.Key, attrsToAny(attrs)...)
	}
	return attr
}

func isSensitive(key string) bool {
	lowered := strings.ToLower(key)
	for _, fragment := range sensitiveFragments {
		if strings.Contains(lowered, fragment) {
			return true
		}
	}
	return false
}

func attrsToAny(attrs []slog.Attr) []any {
	values := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		values = append(values, attr)
	}
	return values
}
