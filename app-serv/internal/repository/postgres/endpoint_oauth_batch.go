// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_oauth_batch.go
// @for       The transactional apply step of the OAuth credential import, the endpoint UPDATE it shares with the single write path, and the credential-only write an OAuth rotation makes.
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn, internal/domain, context.
// @reason    SPEC-API-001 §7.5 imports a batch of already-obtained OAuth credentials, and §8.1 makes that batch all-or-nothing: a refused row must leave no account behind. A row here either creates an endpoint or updates the one that already stands for the account, so the two statements have to run inside one transaction and the UPDATE has to be callable from both a transaction and the pool.
//
//	The execer interface below is that second requirement, and it is
//	why the UPDATE lives here rather than in endpoint.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-18
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// dbExecer is the statement surface a write helper needs. Both *pgxpool.Pool and
// pgx.Tx satisfy it, which is what lets one implementation of a statement serve
// the single-write path and a batch transaction instead of two copies that drift.
type dbExecer interface {
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
				if err == nil {
					err = updateEndpointCredential(ctx, tx, endpoint)
				}
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

// updateEndpoint persists management fields only. It never touches oauth,
// account, or consecutive_use_count, which credential and outcome writes own.
func updateEndpoint(ctx context.Context, exec dbExecer, endpoint domain.UpstreamEndpoint) error {
	testJSON, err := marshalTestStatus(endpoint.TestStatus())
	if err != nil {
		return err
	}
	tagsJSON, err := marshalTags(endpoint.Tags())
	if err != nil {
		return err
	}
	q := `
UPDATE upstream_endpoints
   SET label = $1,
       priority = $2,
       status = $3,
       test_status = $4,
       rate_limited_until = $5,
       last_used_at = $6,
       updated_at = $7,
       global_priority = $8,
       default_model = $9,
       last_error = $10,
       last_error_at = $11,
       error_code = $12,
       proxy_pool_id = $13,
       oauth_retry_at = $14,
       oauth_retry_rounds = $15,
       tags = $16,
       disabled_until = $17
 WHERE id = $18`

	args := []any{
		endpoint.Label(), endpoint.Priority(), string(endpoint.Status()),
		testJSON, endpoint.RateLimitedUntil(), endpoint.LastUsedAt(),
		endpoint.UpdatedAt(),
	}
	args = append(args, parityColumns(endpoint, false)...)
	args = append(args, tagsJSON, endpoint.DisabledUntil(), endpoint.ID())
	tag, err := exec.Exec(ctx, q, args...)
	if err != nil {
		return translateEndpointError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEndpointNotFound
	}
	return nil
}

// updateEndpointCredential writes the credential and account identity an explicit
// import owns. It is separate from updateEndpoint so a management, probe, or key
// write cannot restore a token the vendor has already rotated.
func updateEndpointCredential(ctx context.Context, exec dbExecer, endpoint domain.UpstreamEndpoint) error {
	oauthJSON, err := marshalOAuth(endpoint.OAuth())
	if err != nil {
		return err
	}
	accountJSON, err := marshalAccount(endpoint.Account())
	if err != nil {
		return err
	}
	const q = `
UPDATE upstream_endpoints
   SET oauth = $1, account = $2, updated_at = $3
 WHERE id = $4`
	tag, err := exec.Exec(ctx, q, oauthJSON, accountJSON, endpoint.UpdatedAt(), endpoint.ID())
	if err != nil {
		return translateEndpointError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEndpointNotFound
	}
	return nil
}

// updateEndpointTokens writes the credential an OAuth rotation owns and nothing else.
// The rotation loaded the aggregate before it went to the vendor, so re-sending the
// other columns would restore whatever the operator changed in the meantime.
func updateEndpointTokens(
	ctx context.Context, exec dbExecer, endpoint domain.UpstreamEndpoint, loaded domain.OAuthCredential,
) error {
	oauthJSON, err := marshalOAuth(endpoint.OAuth())
	if err != nil {
		return err
	}
	accountJSON, err := marshalAccount(endpoint.Account())
	if err != nil {
		return err
	}
	// Both token fields are omitempty in the stored shape, so a credential that
	// carries neither is absent from the jsonb rather than present and blank;
	// COALESCE keeps that row comparable to the empty string the caller holds.
	q := `
UPDATE upstream_endpoints
   SET oauth = $1,
       account = $2,
       updated_at = $3
 WHERE id = $4
   AND COALESCE(oauth->>'access_token_encrypted', '')  = $5
   AND COALESCE(oauth->>'refresh_token_encrypted', '') = $6`
	tag, err := exec.Exec(ctx, q, oauthJSON, accountJSON, endpoint.UpdatedAt(), endpoint.ID(),
		loaded.AccessTokenEncrypted(), loaded.RefreshTokenEncrypted())
	if err != nil {
		return translateEndpointError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewConflictError("the endpoint credential changed during this write")
	}
	return nil
}
