// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_oauth_batch.go
// @for       The transactional apply step of the OAuth credential import, and
//
//	the endpoint UPDATE it shares with the single write path.
//
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn,
//
//	internal/domain, context.
//
// @reason    SPEC-API-001 §7.5 imports a batch of already-obtained OAuth
//
//	credentials, and §8.1 makes that batch all-or-nothing: a refused
//	row must leave no account behind. A row here either creates an
//	endpoint or updates the one that already stands for the account,
//	so the two statements have to run inside one transaction and the
//	UPDATE has to be callable from both a transaction and the pool.
//	The execer interface below is that second requirement, and it is
//	why the UPDATE lives here rather than in endpoint.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// endpointExecer is the statement surface the endpoint UPDATE needs. Both
// *pgxpool.Pool and pgx.Tx satisfy it, which is what lets one implementation of
// the statement serve the single write path and the batch transaction instead of
// two copies that drift.
type endpointExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// ImportOAuthBatch applies an imported account batch in one transaction.
//
// Row i is updated when existing[i] is set and inserted otherwise, so a batch
// that mixes a re-import with a new account is written atomically: a failure on
// any row rolls the whole batch back and no account is left half-imported
// (SPEC-API-001 §8.1). The failing row's position is attributed through
// BulkRowError, which is how the handler reports {index, error} per row.
func (r *EndpointRepository) ImportOAuthBatch(ctx context.Context, endpoints []domain.UpstreamEndpoint, existing []bool) error {
	if len(endpoints) == 0 {
		return nil
	}
	if len(existing) != len(endpoints) {
		return domain.NewInternalError("oauth import batch: existing flags do not match the endpoint count")
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for i, endpoint := range endpoints {
			var err error
			if existing[i] {
				err = updateEndpoint(ctx, tx, endpoint)
			} else {
				err = insertEndpoint(ctx, tx, endpoint)
			}
			if err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
		}
		return nil
	})
}

// updateEndpoint persists an endpoint's own fields through any execer, including
// its OAuth state (stored as ciphertext) and its account identity. Keys are
// untouched: they have their own methods because a key change is a different
// concern.
func updateEndpoint(ctx context.Context, exec endpointExecer, endpoint domain.UpstreamEndpoint) error {
	oauthJSON, err := marshalOAuth(endpoint.OAuth())
	if err != nil {
		return err
	}
	accountJSON, err := marshalAccount(endpoint.Account())
	if err != nil {
		return err
	}
	testJSON, err := marshalTestStatus(endpoint.TestStatus())
	if err != nil {
		return err
	}
	const q = `
UPDATE upstream_endpoints
   SET label = $1,
       priority = $2,
       status = $3,
       oauth = $4,
       account = $5,
       test_status = $6,
       rate_limited_until = $7,
       last_used_at = $8,
       updated_at = $9
 WHERE id = $10`

	tag, err := exec.Exec(ctx, q, endpoint.Label(), endpoint.Priority(),
		string(endpoint.Status()), oauthJSON, accountJSON, testJSON,
		endpoint.RateLimitedUntil(), endpoint.LastUsedAt(),
		endpoint.UpdatedAt(), endpoint.ID())
	if err != nil {
		return translateEndpointError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEndpointNotFound
	}
	return nil
}
