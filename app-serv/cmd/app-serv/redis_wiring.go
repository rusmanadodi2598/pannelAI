// Command app-serv builds the Redis client every store shares.
//
// @file      cmd/app-serv/redis_wiring.go
// @for       The Redis client options, including the optional TLS transport.
// @uses      internal/config, crypto/tls, crypto/x509, os, github.com/redis/go-redis/v9.
// @reason    Session, limiter, and rotation state all travel over this client, so the TLS decision is made once here rather than per store; an unreadable CA bundle fails the boot instead of silently falling back to system roots.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-24
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
)

// buildRedisClient applies the validated config to one client.
func buildRedisClient(cfg config.Config) (*redis.Client, error) {
	tlsConfig, err := redisTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		PoolSize:     cfg.RedisPoolSize,
		DialTimeout:  cfg.RedisDialTimeout,
		ReadTimeout:  cfg.RedisReadTimeout,
		WriteTimeout: cfg.RedisWriteTimeout,
		TLSConfig:    tlsConfig,
	}), nil
}

// redisTLSConfig returns nil when TLS is off, and a config that verifies the
// server against the system roots or the configured bundle when it is on.
func redisTLSConfig(cfg config.Config) (*tls.Config, error) {
	if !cfg.RedisTLSEnabled {
		return nil, nil
	}
	// TLS 1.2 is the floor Redis and its proxies support in common; leaving the
	// field at its zero value would still negotiate 1.2, but stating it keeps a
	// future default change from silently lowering the floor.
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.RedisTLSCAFile == "" {
		return tlsConfig, nil
	}
	bundle, err := os.ReadFile(cfg.RedisTLSCAFile)
	if err != nil {
		return nil, fmt.Errorf("config: reading REDIS_TLS_CA_FILE: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(bundle) {
		return nil, fmt.Errorf("config: REDIS_TLS_CA_FILE contains no PEM certificate")
	}
	tlsConfig.RootCAs = roots
	return tlsConfig, nil
}
