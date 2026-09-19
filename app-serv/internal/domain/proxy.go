// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/proxy.go
// @for       The Proxy aggregate root: a saved outbound proxy candidate and its
//
//	last connectivity test (SPEC-API-001 §7.11).
//
// @uses      internal/domain (ULID, error constructors), strings, time.
// @reason    A proxy is a pool entry, not a routing rule: §7.11 assigns the pool
//
//	to traffic through settings, so what this aggregate owns is the
//	candidate's own validity and its last test result. The protocol and
//	host rules live in proxy_protocol.go so this file stays inside the
//	AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-19
package domain

import (
	"strings"
	"time"
)

// ProxyTestStatus is the outcome of the last connectivity test. It is stored as
// the row's status document and reported by the test route, so the two can
// never disagree about when the candidate was last proven.
type ProxyTestStatus struct {
	State     string
	LatencyMS int
	CheckedAt *time.Time
	Message   string
}

// Proxy is a saved proxy candidate. Fields are unexported on purpose
// (AGENTS.md §2.2): the host and port rules are enforced by the methods below,
// and the sealed password is never handed out except as the stored ciphertext.
type Proxy struct {
	id                string
	label             string
	protocol          ProxyProtocol
	host              string
	port              int
	username          string
	passwordEncrypted string
	enabled           bool
	status            ProxyTestStatus
	createdAt         time.Time
	updatedAt         time.Time
}

// NewProxy is the only constructor for a new candidate.
func NewProxy(id, label string, protocol ProxyProtocol, host string, port int, username, passwordEncrypted string, now time.Time) (Proxy, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return Proxy{}, NewValidationError("label is required")
	}
	if len(label) > 120 {
		return Proxy{}, NewValidationError("label must be at most 120 characters")
	}
	if err := validateProxyProtocol(protocol); err != nil {
		return Proxy{}, err
	}
	if err := validateProxyHost(host); err != nil {
		return Proxy{}, err
	}
	if err := validateProxyPort(port); err != nil {
		return Proxy{}, err
	}
	username = strings.TrimSpace(username)
	if len(username) > 200 {
		return Proxy{}, NewValidationError("username must be at most 200 characters")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		id = IDPrefixProxy + NewULID(now)
	} else if !strings.HasPrefix(id, IDPrefixProxy) {
		id = IDPrefixProxy + id
	}
	return Proxy{
		id:                id,
		label:             label,
		protocol:          protocol,
		host:              strings.TrimSpace(host),
		port:              port,
		username:          username,
		passwordEncrypted: passwordEncrypted,
		enabled:           true,
		createdAt:         now,
		updatedAt:         now,
	}, nil
}

// RehydrateProxy rebuilds a stored row. For the repository load path only;
// never use it to create a candidate.
func RehydrateProxy(id, label string, protocol ProxyProtocol, host string, port int, username, passwordEncrypted string, enabled bool, status ProxyTestStatus, createdAt, updatedAt time.Time) Proxy {
	return Proxy{
		id: id, label: label, protocol: protocol, host: host, port: port,
		username: username, passwordEncrypted: passwordEncrypted, enabled: enabled,
		status: status, createdAt: createdAt, updatedAt: updatedAt,
	}
}

// Accessors expose state without allowing mutation.
func (p Proxy) ID() string                { return p.id }
func (p Proxy) Label() string             { return p.label }
func (p Proxy) Protocol() ProxyProtocol   { return p.protocol }
func (p Proxy) Host() string              { return p.host }
func (p Proxy) Port() int                 { return p.port }
func (p Proxy) Username() string          { return p.username }
func (p Proxy) PasswordEncrypted() string { return p.passwordEncrypted }
func (p Proxy) Enabled() bool             { return p.enabled }
func (p Proxy) Status() ProxyTestStatus   { return p.status }
func (p Proxy) CreatedAt() time.Time      { return p.createdAt }
func (p Proxy) UpdatedAt() time.Time      { return p.updatedAt }

// HasPassword reports whether a sealed password is stored. The plaintext never
// leaves the service, so the wire shape can only say whether one exists.
func (p Proxy) HasPassword() bool { return p.passwordEncrypted != "" }

// Relabel changes the display label.
func (p *Proxy) Relabel(label string, now time.Time) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return NewValidationError("label is required")
	}
	if len(label) > 120 {
		return NewValidationError("label must be at most 120 characters")
	}
	p.label = label
	p.updatedAt = now
	return nil
}

// Repoint changes where the candidate connects. The stored test result is
// cleared: a status measured against the old address says nothing about the new
// one, and keeping it would report a proof that no longer exists.
func (p *Proxy) Repoint(protocol ProxyProtocol, host string, port int, now time.Time) error {
	if err := validateProxyProtocol(protocol); err != nil {
		return err
	}
	if err := validateProxyHost(host); err != nil {
		return err
	}
	if err := validateProxyPort(port); err != nil {
		return err
	}
	p.protocol = protocol
	p.host = strings.TrimSpace(host)
	p.port = port
	p.status = ProxyTestStatus{}
	p.updatedAt = now
	return nil
}

// SetCredentials replaces the stored username and sealed password.
func (p *Proxy) SetCredentials(username, passwordEncrypted string, now time.Time) error {
	username = strings.TrimSpace(username)
	if len(username) > 200 {
		return NewValidationError("username must be at most 200 characters")
	}
	p.username = username
	p.passwordEncrypted = passwordEncrypted
	p.status = ProxyTestStatus{}
	p.updatedAt = now
	return nil
}

// SetEnabled turns the candidate on or off without touching its address or
// credentials, so disabling and re-enabling does not lose either.
func (p *Proxy) SetEnabled(enabled bool, now time.Time) {
	p.enabled = enabled
	p.updatedAt = now
}

// RecordTest stores the last connectivity result.
func (p *Proxy) RecordTest(state string, latencyMS int, message string, now time.Time) {
	checked := now
	p.status = ProxyTestStatus{State: state, LatencyMS: latencyMS, CheckedAt: &checked, Message: message}
	p.updatedAt = now
}
