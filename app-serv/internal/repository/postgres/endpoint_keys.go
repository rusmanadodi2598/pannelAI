// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_keys.go
// @for       PostgreSQL persistence for the upstream_keys child rows, including the transactional reorder of a provider's endpoints.
// @uses      github.com/jackc/pgx/v5, internal/domain.
// @reason    A key is a child of the endpoint aggregate, so nothing here loads or stores a key on its own: every statement is scoped by endpoint_id and the aggregate's invariant (an api_key endpoint keeps a usable credential) is enforced in the domain before a call arrives. The batch and reorder statements live here because AGENTS.md §1.7 forbids a query per row and the interface's per-key methods cannot express one statement boundary across a set.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-17
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// AddKey persists one new key under an existing endpoint. A duplicate label is
// the endpoint's own invariant, checked in the domain; the missing-row case here
// is a vanished endpoint, which is the not-found sentinel a caller maps to 404.
func (r *EndpointRepository) AddKey(ctx context.Context, endpointID string, key domain.UpstreamKey) error {
	const q = `
INSERT INTO upstream_keys
    (id, endpoint_id, label, value_encrypted, key_hint, priority, status,
     consecutive_errors, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	tag, err := r.pool.Exec(ctx, q, key.ID(), endpointID, key.Label(),
		key.EncryptedValue(), key.Hint(), key.Priority(), string(key.Status()),
		key.ConsecutiveErrors(), key.CreatedAt(), key.UpdatedAt())
	if err != nil {
		return translateKeyError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEndpointNotFound
	}
	return nil
}

// UpdateKey persists a key's mutable fields, its operator marks, and, when a
// replacement credential is supplied, its sealed value and hint. An empty value leaves
// the stored credential untouched, because the value is write-only on the wire: a PATCH
// that omits it must not blank the credential.
func (r *EndpointRepository) UpdateKey(ctx context.Context, key domain.UpstreamKey) error {
	tagsJSON, err := marshalTags(key.Tags())
	if err != nil {
		return err
	}
	return updateKey(ctx, r.pool, key, tagsJSON)
}

// updateKey writes one key's own fields through any execer, so the bulk path runs the
// same statement inside its transaction instead of carrying a second spelling.
func updateKey(ctx context.Context, exec dbExecer, key domain.UpstreamKey, tagsJSON string) error {
	const q = `
UPDATE upstream_keys
   SET label = $1,
       priority = $2,
       status = $3,
       value_encrypted = COALESCE(NULLIF($4, ''), value_encrypted),
       key_hint = COALESCE(NULLIF($5, ''), key_hint),
       updated_at = $6,
       tags = $7,
       disabled_until = $8
 WHERE id = $9 AND endpoint_id = $10`

	tag, err := exec.Exec(ctx, q, key.Label(), key.Priority(), string(key.Status()),
		key.EncryptedValue(), key.Hint(), key.UpdatedAt(), tagsJSON, key.DisabledUntil(),
		key.ID(), key.EndpointID())
	if err != nil {
		return translateKeyError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("upstream key not found")
	}
	return nil
}

// DeleteKey removes one key, scoped by endpoint so a key id from another account
// cannot be deleted through it.
func (r *EndpointRepository) DeleteKey(ctx context.Context, endpointID, keyID string) error {
	const q = `DELETE FROM upstream_keys WHERE id = $1 AND endpoint_id = $2`

	tag, err := r.pool.Exec(ctx, q, keyID, endpointID)
	if err != nil {
		return translateKeyError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("upstream key not found")
	}
	return nil
}

// Reorder assigns new priorities to every endpoint of one provider in one
// transaction.
//
// It takes the provider's rows under a lock and refuses a set that does not name
// exactly the endpoints currently stored, because assigning a subset would leave
// the remainder holding their old numbers and two endpoints claiming one slot,
// the collision this method exists to prevent. The caller therefore sends the
// complete new order rather than one endpoint's new number.
func (r *EndpointRepository) Reorder(ctx context.Context, providerID string, orderedIDs []string) error {
	if len(orderedIDs) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id FROM upstream_endpoints WHERE provider_id = $1 ORDER BY id FOR UPDATE`, providerID)
		if err != nil {
			return translateEndpointError(err)
		}
		defer rows.Close()

		current := make(map[string]struct{}, len(orderedIDs))
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			current[id] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			return translateEndpointError(err)
		}
		if len(current) != len(orderedIDs) {
			return domain.NewConflictError("reorder must name every endpoint of the provider")
		}
		for _, id := range orderedIDs {
			if _, ok := current[id]; !ok {
				return domain.NewConflictError("reorder names an endpoint of another provider")
			}
		}

		priorities := make([]int, len(orderedIDs))
		for i := range orderedIDs {
			priorities[i] = i + 1
		}
		const q = `
UPDATE upstream_endpoints AS e
   SET priority = v.priority, updated_at = now()
  FROM unnest($2::text[], $3::int[]) AS v(id, priority)
 WHERE e.id = v.id AND e.provider_id = $1`
		if _, err := tx.Exec(ctx, q, providerID, orderedIDs, priorities); err != nil {
			return translateEndpointError(err)
		}
		return nil
	})
}
