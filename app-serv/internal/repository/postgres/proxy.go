// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/proxy.go
// @for       PostgreSQL persistence for saved proxy candidates (SPEC-API-001 §7.11).
// @uses      github.com/jackc/pgx/v5, internal/domain, encoding/json, bytes,
//
//	errors, fmt, time.
//
// @reason    The status document is jsonb, so its codec lives beside the
//
//	statements that read and write it, and the driver's errors are
//	translated here — a missing row is domain.ErrProxyNotFound, never
//	pgx.ErrNoRows, so the service layer stays driver-free (AGENTS.md §1.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// proxyColumns is the read column list, in the order scanProxy expects.
const proxyColumns = `id, label, protocol, host, port, username, password_encrypted,
	enabled, status, created_at, updated_at`

// proxyStatusDocument is the jsonb shape of a candidate's last test result. A
// concrete struct, so the column has one declared decode contract
// (AGENTS.md §1.4).
type proxyStatusDocument struct {
	State     string     `json:"state,omitempty"`
	LatencyMS int        `json:"latency_ms,omitempty"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	Message   string     `json:"message,omitempty"`
}

// ProxyRepository stores proxy candidates.
type ProxyRepository struct {
	pool *pgxpool.Pool
}

// NewProxyRepository binds the repository to a pool.
func NewProxyRepository(pool *pgxpool.Pool) *ProxyRepository {
	return &ProxyRepository{pool: pool}
}

// Create inserts one candidate, status document included, so a create and a
// later test write the same columns.
func (r *ProxyRepository) Create(ctx context.Context, proxy domain.Proxy) error {
	status, err := encodeProxyStatus(proxy.Status())
	if err != nil {
		return err
	}
	const q = `
INSERT INTO proxies (id, label, protocol, host, port, username, password_encrypted,
	enabled, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err = r.pool.Exec(ctx, q,
		proxy.ID(), proxy.Label(), string(proxy.Protocol()), proxy.Host(), proxy.Port(),
		proxy.Username(), proxy.PasswordEncrypted(), proxy.Enabled(), status,
		proxy.CreatedAt(), proxy.UpdatedAt())
	if err != nil {
		return fmt.Errorf("proxies: %w", err)
	}
	return nil
}

// List returns every candidate ordered by label, which is the order the panel
// shows and the order an operator scans.
func (r *ProxyRepository) List(ctx context.Context) ([]domain.Proxy, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+proxyColumns+` FROM proxies ORDER BY label, id`)
	if err != nil {
		return nil, fmt.Errorf("proxies: %w", err)
	}
	defer rows.Close()

	proxies := make([]domain.Proxy, 0)
	for rows.Next() {
		proxy, err := scanProxy(rows)
		if err != nil {
			return nil, translateProxyError(err)
		}
		proxies = append(proxies, proxy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("proxies: %w", err)
	}
	return proxies, nil
}

// GetByID loads one candidate, or domain.ErrProxyNotFound.
func (r *ProxyRepository) GetByID(ctx context.Context, id string) (domain.Proxy, error) {
	proxy, err := scanProxy(r.pool.QueryRow(ctx, `SELECT `+proxyColumns+` FROM proxies WHERE id = $1`, id))
	if err != nil {
		return domain.Proxy{}, translateProxyError(err)
	}
	return proxy, nil
}

// Update writes every mutable field as one statement, so a test result and a
// repoint can never be stored half-applied.
func (r *ProxyRepository) Update(ctx context.Context, proxy domain.Proxy) error {
	status, err := encodeProxyStatus(proxy.Status())
	if err != nil {
		return err
	}
	const q = `
UPDATE proxies
SET label = $2, protocol = $3, host = $4, port = $5, username = $6,
	password_encrypted = $7, enabled = $8, status = $9, updated_at = $10
WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q,
		proxy.ID(), proxy.Label(), string(proxy.Protocol()), proxy.Host(), proxy.Port(),
		proxy.Username(), proxy.PasswordEncrypted(), proxy.Enabled(), status, proxy.UpdatedAt())
	if err != nil {
		return fmt.Errorf("proxies: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProxyNotFound
	}
	return nil
}

// Delete removes one candidate, or reports domain.ErrProxyNotFound.
func (r *ProxyRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM proxies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("proxies: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProxyNotFound
	}
	return nil
}

// scanProxy reads one row into a rehydrated aggregate.
func scanProxy(s pgx.Row) (domain.Proxy, error) {
	var (
		id, label, protocol, host   string
		port                        int
		username, passwordEncrypted string
		enabled                     bool
		rawStatus                   []byte
		createdAt, updatedAt        time.Time
	)
	if err := s.Scan(&id, &label, &protocol, &host, &port, &username, &passwordEncrypted,
		&enabled, &rawStatus, &createdAt, &updatedAt); err != nil {
		return domain.Proxy{}, err
	}
	status, err := decodeProxyStatus(rawStatus)
	if err != nil {
		return domain.Proxy{}, err
	}
	return domain.RehydrateProxy(id, label, domain.ProxyProtocol(protocol), host, port,
		username, passwordEncrypted, enabled, status, createdAt, updatedAt), nil
}

// encodeProxyStatus renders the status document. An untested candidate encodes
// as an empty object, which decodes back to the zero status.
func encodeProxyStatus(status domain.ProxyTestStatus) ([]byte, error) {
	encoded, err := json.Marshal(proxyStatusDocument{
		State:     status.State,
		LatencyMS: status.LatencyMS,
		CheckedAt: status.CheckedAt,
		Message:   status.Message,
	})
	if err != nil {
		return nil, fmt.Errorf("proxies: encoding status: %w", err)
	}
	return encoded, nil
}

// decodeProxyStatus reads the status document back. A blank column is the zero
// status rather than an error: the schema defaults it to an empty object, and a
// candidate that was never tested has no state to report.
func decodeProxyStatus(raw []byte) (domain.ProxyTestStatus, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return domain.ProxyTestStatus{}, nil
	}
	var document proxyStatusDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return domain.ProxyTestStatus{}, fmt.Errorf("proxies: decoding status: %w", err)
	}
	return domain.ProxyTestStatus{
		State:     document.State,
		LatencyMS: document.LatencyMS,
		CheckedAt: document.CheckedAt,
		Message:   document.Message,
	}, nil
}

// translateProxyError maps a driver error to a domain error a caller can act
// on; the wrapped chain reaches logs only (AGENTS.md §1.3).
func translateProxyError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrProxyNotFound
	}
	return fmt.Errorf("proxies: %w", err)
}
