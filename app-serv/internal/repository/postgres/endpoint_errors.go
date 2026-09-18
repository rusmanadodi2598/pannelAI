// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_errors.go
// @for       Driver-to-domain error translation for the endpoint and key tables.
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn, internal/domain,
//
//	errors, fmt.
//
// @reason    A PostgreSQL constraint violation has to become the §8 code a client
//
//	sees, and getting it wrong is invisible until a duplicate label returns
//	500 instead of CONFLICT. Keeping the mapping in one small file — rather
//	than beside each statement — is what stops the endpoint and key paths
//	from drifting apart, and it keeps the mapping testable without a
//	database.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// PostgreSQL SQLSTATE codes this package maps. Naming them keeps the literal out
// of a conditional, where a typo would silently disable the mapping.
const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
	checkViolation      = "23514"
	notNullViolation    = "23502"
)

// translateEndpointError maps a driver error raised by an upstream_endpoints
// statement to a domain error a caller can act on. Unrecognised errors are
// wrapped with table context; the wrapped text reaches logs only, because the
// handler renders domain.Message, never the chain (AGENTS.md §1.3).
func translateEndpointError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrEndpointNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case uniqueViolation:
			// The only unique constraint on this table is
			// idx_upstream_endpoints_provider_label, so a collision is exactly
			// the duplicate-account case the sentinel names.
			return fmt.Errorf("%w: %s", domain.ErrEndpointExists, pgErr.ConstraintName)
		case checkViolation, notNullViolation:
			// The table's CHECK constraints close auth_type and status to the
			// sets the domain also enforces. Reaching one means a value was
			// written that the API never accepts, which is a validation
			// failure rather than a missing row.
			return domain.NewValidationError("upstream endpoint is malformed")
		}
	}
	return fmt.Errorf("upstream_endpoints: %w", err)
}

// translateKeyError maps a driver error raised by an upstream_keys statement.
//
// The label-uniqueness rule is a domain invariant, not a database one: the
// migration declares a plain index on (endpoint_id, priority) rather than a
// unique one, so a duplicate label is refused by UpstreamEndpoint.AddKey before a
// statement runs. A unique violation here is therefore the primary key on id —
// refused as CONFLICT rather than reported as an internal fault, because a
// caller can act on "that key already exists".
func translateKeyError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errUpstreamKeyNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case uniqueViolation:
			return fmt.Errorf("%w: upstream key already exists", errUpstreamKeyConflict)
		case foreignKeyViolation:
			// The parent endpoint vanished between the caller's load and this
			// write, so the account the key would belong to does not exist.
			return domain.ErrEndpointNotFound
		case checkViolation, notNullViolation:
			return domain.NewValidationError("upstream key is malformed")
		}
	}
	return fmt.Errorf("upstream_keys: %w", err)
}

// Sentinels for the key paths. They are declared here rather than in
// domain/errors.go because that file is frozen for this vertical, while
// repository.EndpointRepository documents only that a missing key is a not-found
// and a collision a CONFLICT. Both carry the §8 codes and an English message, so
// a handler maps them exactly as it maps the domain sentinels.
var (
	// errUpstreamKeyNotFound is returned when a key id does not belong to the
	// given endpoint.
	errUpstreamKeyNotFound = domain.NewNotFoundError("upstream key not found")

	// errUpstreamKeyConflict is returned when the identity a key insert claims is
	// already taken.
	errUpstreamKeyConflict = domain.NewConflictError("upstream key already exists")
)
