// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/provider_node.go
// @for       PostgreSQL persistence for the ProviderNode aggregate root.
// @uses      github.com/jackc/pgx/v5, internal/domain, internal/repository.
// @reason    SPEC-API-001 §6 defines provider_nodes and §7.4 makes a node a
//
//	separate aggregate from an endpoint (a node carries no credential of
//	its own), so it gets its own statements rather than a mode on the
//	endpoint repository. The prefix is the model-string namespace and
//	carries a UNIQUE index, which is why a collision must surface as
//	domain.ErrNodePrefixTaken and not as a driver message.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// nodeColumns is the projection every node read uses, in scan order.
const nodeColumns = `id, type, name, prefix, api_type, base_url, created_at, updated_at`

// NodeRepository persists custom provider nodes in PostgreSQL.
type NodeRepository struct {
	pool *pgxpool.Pool
}

// NewNodeRepository binds the repository to a pool whose limits the composition
// root sets explicitly (AGENTS.md §1.7).
func NewNodeRepository(pool *pgxpool.Pool) *NodeRepository {
	return &NodeRepository{pool: pool}
}

// Create inserts a node. A duplicate prefix maps to domain.ErrNodePrefixTaken so
// the service can answer 409 rather than leaking a driver message.
func (r *NodeRepository) Create(ctx context.Context, node domain.ProviderNode) error {
	const q = `
INSERT INTO provider_nodes
    (id, type, name, prefix, api_type, base_url, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	tag, err := r.pool.Exec(ctx, q, node.ID(), string(node.Type()), node.Name(),
		node.Prefix(), node.APIType(), node.BaseURL(), node.CreatedAt(), node.UpdatedAt())
	if err != nil {
		return translateNodeError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewInternalError("provider node was not stored")
	}
	return nil
}

// List returns every node, ordered by prefix so the list is stable. It is
// deliberately unpaginated: nodes are a hand-configured set, and the §1.7 rule
// against unbounded reads is about request-serving tables that grow with traffic
// (upstream_endpoints, usage_records), not about a table an operator fills in by
// hand.
func (r *NodeRepository) List(ctx context.Context) ([]domain.ProviderNode, error) {
	const q = `SELECT ` + nodeColumns + ` FROM provider_nodes ORDER BY prefix, id`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, translateNodeError(err)
	}
	defer rows.Close()

	nodes := make([]domain.ProviderNode, 0, 8)
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, translateNodeError(err)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, translateNodeError(err)
	}
	return nodes, nil
}

// GetByID loads one node. A missing row yields domain.ErrNodeNotFound.
func (r *NodeRepository) GetByID(ctx context.Context, id string) (domain.ProviderNode, error) {
	const q = `SELECT ` + nodeColumns + ` FROM provider_nodes WHERE id = $1`

	node, err := scanNode(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return domain.ProviderNode{}, translateNodeError(err)
	}
	return node, nil
}

// Update persists a node's mutable fields. The type and api_type are part of the
// node's identity (they decide which wire format it speaks) and are therefore
// absent from the PATCH contract; a change there is a new node.
func (r *NodeRepository) Update(ctx context.Context, node domain.ProviderNode) error {
	const q = `
UPDATE provider_nodes
   SET name = $1, prefix = $2, base_url = $3, updated_at = $4
 WHERE id = $5`

	tag, err := r.pool.Exec(ctx, q, node.Name(), node.Prefix(), node.BaseURL(),
		node.UpdatedAt(), node.ID())
	if err != nil {
		return translateNodeError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNodeNotFound
	}
	return nil
}

// Delete removes a node. The caller checks CountEndpoints first, because
// provider_nodes carries no foreign key to upstream_endpoints: an endpoint
// references a provider by id string, and a built-in provider has no node row at
// all, so a constraint is not expressible here.
func (r *NodeRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM provider_nodes WHERE id = $1`, id)
	if err != nil {
		return translateNodeError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNodeNotFound
	}
	return nil
}

// CountEndpoints reports how many endpoints reference a provider id, so a delete
// can refuse while one still does (domain.ErrNodeInUse). It reads the endpoints
// table, not the node's, because the node id is what those rows store in
// provider_id.
func (r *NodeRepository) CountEndpoints(ctx context.Context, providerID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM upstream_endpoints WHERE provider_id = $1`, providerID).Scan(&count)
	if err != nil {
		return 0, translateEndpointError(err)
	}
	return count, nil
}

// scanNode reads one provider_nodes row into a rehydrated aggregate.
func scanNode(s scanner) (domain.ProviderNode, error) {
	var (
		id, nodeType, name, prefix, apiType, baseURL string
		createdAt, updatedAt                         time.Time
	)
	if err := s.Scan(&id, &nodeType, &name, &prefix, &apiType, &baseURL,
		&createdAt, &updatedAt); err != nil {
		return domain.ProviderNode{}, err
	}
	return domain.RehydrateProviderNode(id, domain.NodeType(nodeType), name,
		prefix, apiType, baseURL, createdAt, updatedAt), nil
}

// translateNodeError maps a driver error raised by a provider_nodes statement.
func translateNodeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNodeNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case uniqueViolation:
			// idx_provider_nodes_prefix is the table's only unique index, so a
			// collision is the namespace conflict the sentinel names.
			return fmt.Errorf("%w: %s", domain.ErrNodePrefixTaken, pgErr.ConstraintName)
		case checkViolation, notNullViolation:
			return domain.NewValidationError("provider node is malformed")
		}
	}
	return fmt.Errorf("provider_nodes: %w", err)
}
