// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_builder.go
// @for       The builder and gateway configuration the six-section builder needs at boot.
// @uses      fmt, net/url, time.
// @reason    Keeping the builder block beside the telegram block lets each sit under the line limit without changing what Load validates.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-23
package config

import (
	"fmt"
	"net/url"
	"time"
)

const (
	defaultBuilderMaxTokens = 700
	defaultBuilderTimeout   = 20 * time.Second
	defaultBuilderDailyCap  = 60
	defaultPromptMaxChars   = 2000
)

// requireBuilder validates the builder transport without echoing the key.
func requireBuilder(cfg Config) error {
	if cfg.GatewayBaseURL == "" {
		return fmt.Errorf("GATEWAY_BASE_URL is required")
	}
	parsed, err := url.Parse(cfg.GatewayBaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("GATEWAY_BASE_URL must be an https URL with a host and no user info")
	}
	if cfg.GatewayKey == "" {
		return fmt.Errorf("GATEWAY_KEY is required")
	}
	if cfg.BuilderModel == "" {
		return fmt.Errorf("BUILDER_MODEL is required")
	}
	return nil
}
