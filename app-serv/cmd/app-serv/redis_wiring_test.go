// Command app-serv builds the Redis client every store shares.
//
// @file      cmd/app-serv/redis_wiring_test.go
// @for       The Redis client's TLS transport: off by default, verifying when on, and a bad bundle refused at boot.
// @uses      crypto/ecdsa, crypto/elliptic, crypto/rand, crypto/tls, crypto/x509, encoding/pem, os, testing, time, internal/config.
// @reason    A plaintext fallback or a silently ignored CA path would defeat the transport policy, so the options the client actually receives are asserted rather than the config alone.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-24
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
)

func TestBuildRedisClient_TLS(t *testing.T) {
	cfg := config.Config{RedisAddr: "redis.example.com:6379", RedisPoolSize: 4}

	client, err := buildRedisClient(cfg)
	if err != nil {
		t.Fatalf("buildRedisClient() error = %v", err)
	}
	if client.Options().TLSConfig != nil {
		t.Fatal("TLSConfig is set while REDIS_TLS_ENABLED is false")
	}

	cfg.RedisTLSEnabled = true
	client, err = buildRedisClient(cfg)
	if err != nil {
		t.Fatalf("buildRedisClient() error = %v", err)
	}
	tlsConfig := client.Options().TLSConfig
	if tlsConfig == nil || tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLSConfig = %+v, want TLS 1.2 or newer", tlsConfig)
	}
	if tlsConfig.RootCAs != nil {
		t.Fatal("RootCAs is set without REDIS_TLS_CA_FILE")
	}

	cfg.RedisTLSCAFile = writeTestCA(t)
	client, err = buildRedisClient(cfg)
	if err != nil {
		t.Fatalf("buildRedisClient() with a bundle error = %v", err)
	}
	if client.Options().TLSConfig.RootCAs == nil {
		t.Fatal("RootCAs is empty, want the configured bundle")
	}

	cfg.RedisTLSCAFile = filepath.Join(t.TempDir(), "missing.pem")
	if _, err := buildRedisClient(cfg); err == nil {
		t.Fatal("buildRedisClient() accepted a missing CA bundle")
	}
}

func TestRedisTLSConfig_RefusesANonPEMBundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("writing the bundle: %v", err)
	}
	if _, err := redisTLSConfig(config.Config{RedisTLSEnabled: true, RedisTLSCAFile: path}); err == nil {
		t.Fatal("redisTLSConfig() accepted a bundle with no PEM certificate")
	}
}

// writeTestCA writes a self-signed certificate and returns its path, so the
// pool accepts a real PEM rather than a placeholder the parser would refuse.
func writeTestCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating the key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "redis-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating the certificate: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	block := &pem.Block{Type: "CERTIFICATE", Bytes: der}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("writing the certificate: %v", err)
	}
	return path
}
