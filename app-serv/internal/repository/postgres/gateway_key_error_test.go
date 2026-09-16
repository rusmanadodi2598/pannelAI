// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/gateway_key_error_test.go
// @for       Table-driven tests for driver-to-domain error translation.
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn, errors, testing.
// @reason    AGENTS.md §2.1 requires tests alongside repository logic, and this
//
//	mapping is what turns a PostgreSQL constraint violation into the
//	§8 code a client sees. Getting it wrong is invisible until a
//	duplicate name silently returns 500 instead of CONFLICT, so the
//	rule is pinned here rather than left to a live smoke run.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-16
package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestTranslatePGError covers every branch the mapper can take.
func TestTranslatePGError(t *testing.T) {
	cases := []struct {
		name    string
		in      error
		wantIs  error
		wantNil bool
	}{
		{"nil stays nil", nil, nil, true},
		{"no rows becomes not found", pgx.ErrNoRows, domain.ErrGatewayKeyNotFound, false},
		{"unique violation becomes conflict", &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "uniq_gateway_keys_name",
		}, domain.ErrGatewayKeyExists, false},
		{"unknown pg error is wrapped", &pgconn.PgError{Code: "08006"}, nil, false},
		{"unknown error is wrapped", errors.New("connection reset"), nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translatePGError(tc.in)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("translatePGError(nil) = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("translatePGError() = nil, want an error")
			}
			if tc.wantIs != nil && !errors.Is(got, tc.wantIs) {
				t.Fatalf("translatePGError() = %v, want it to wrap %v", got, tc.wantIs)
			}
		})
	}
}

// TestTranslatePGError_UniqueIsConflictWithoutLeakingDriverText asserts the
// mapped error stays matchable by errors.Is and does not expose raw driver text
// through the domain message a client receives (AGENTS.md §1.3).
func TestTranslatePGError_UniqueIsConflictWithoutLeakingDriverText(t *testing.T) {
	got := translatePGError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "uniq_gateway_keys_name",
		Message:        "duplicate key value violates unique constraint \"uniq_gateway_keys_name\"",
	})

	public := domain.AsAppError(got)
	if public.Code != "CONFLICT" {
		t.Fatalf("code = %q, want CONFLICT", public.Code)
	}
	if public.HTTPStatus() != 409 {
		t.Fatalf("status = %d, want 409", public.HTTPStatus())
	}
	// The client-facing message is the domain sentence, not the driver's.
	if public.Message != domain.ErrGatewayKeyExists.Message {
		t.Fatalf("message = %q, want the domain message %q", public.Message, domain.ErrGatewayKeyExists.Message)
	}
}
