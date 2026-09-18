// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_error_test.go
// @for       Table-driven tests for the endpoint and key driver-error mapping.
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn, errors,
//
//	net/http, testing, internal/domain.
//
// @reason    AGENTS.md §2.1 requires tests alongside repository logic, and this
//
//	mapping is what turns a constraint violation into the §8 code a client
//	sees. Getting it wrong is invisible until a duplicate label silently
//	returns 500 instead of CONFLICT, so it is pinned here — and it needs no
//	database, because the input is a driver error value.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestTranslateEndpointError covers every branch the endpoint mapper can take,
// including the boundary cases a malformed row produces.
func TestTranslateEndpointError(t *testing.T) {
	cases := []struct {
		name     string
		in       error
		wantIs   error
		wantNil  bool
		wantCode string
	}{
		{"nil stays nil", nil, nil, true, ""},
		{"no rows becomes not found", pgx.ErrNoRows, domain.ErrEndpointNotFound, false, "NOT_FOUND"},
		{"unique violation becomes conflict", &pgconn.PgError{
			Code: "23505", ConstraintName: "idx_upstream_endpoints_provider_label",
		}, domain.ErrEndpointExists, false, "CONFLICT"},
		{"wrapped unique violation is still a conflict", errors.Join(errors.New("exec"), &pgconn.PgError{
			Code: "23505", ConstraintName: "idx_upstream_endpoints_provider_label",
		}), domain.ErrEndpointExists, false, "CONFLICT"},
		{"check violation becomes validation", &pgconn.PgError{Code: "23514"}, nil, false, "VALIDATION_ERROR"},
		{"not-null violation becomes validation", &pgconn.PgError{Code: "23502"}, nil, false, "VALIDATION_ERROR"},
		{"unknown sqlstate is wrapped", &pgconn.PgError{Code: "08006"}, nil, false, "INTERNAL_ERROR"},
		{"unknown error is wrapped", errors.New("connection reset"), nil, false, "INTERNAL_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateEndpointError(tc.in)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("translateEndpointError(nil) = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("translateEndpointError() = nil, want an error")
			}
			if tc.wantIs != nil && !errors.Is(got, tc.wantIs) {
				t.Fatalf("translateEndpointError() = %v, want it to wrap %v", got, tc.wantIs)
			}
			if code := domain.AsAppError(got).Code; code != tc.wantCode {
				t.Fatalf("code = %q, want %q", code, tc.wantCode)
			}
		})
	}
}

// TestTranslateKeyError pins the key paths' mapping, which differs from the
// endpoint one on purpose: a key collision is the key's own conflict, not the
// endpoint sentinel, because a caller matching on the endpoint sentinel would
// report the wrong object.
func TestTranslateKeyError(t *testing.T) {
	cases := []struct {
		name     string
		in       error
		wantIs   error
		wantCode string
		wantHTTP int
	}{
		{"no rows becomes key not found", pgx.ErrNoRows, errUpstreamKeyNotFound, "NOT_FOUND", http.StatusNotFound},
		{"unique violation becomes key conflict", &pgconn.PgError{
			Code: "23505", ConstraintName: "upstream_keys_pkey",
		}, errUpstreamKeyConflict, "CONFLICT", http.StatusConflict},
		{"foreign key means the endpoint vanished", &pgconn.PgError{Code: "23503"},
			domain.ErrEndpointNotFound, "NOT_FOUND", http.StatusNotFound},
		{"check violation becomes validation", &pgconn.PgError{Code: "23514"},
			nil, "VALIDATION_ERROR", http.StatusBadRequest},
		{"unknown sqlstate is wrapped", &pgconn.PgError{Code: "08006"},
			nil, "INTERNAL_ERROR", http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateKeyError(tc.in)
			if got == nil {
				t.Fatal("translateKeyError() = nil, want an error")
			}
			if tc.wantIs != nil && !errors.Is(got, tc.wantIs) {
				t.Fatalf("translateKeyError() = %v, want it to wrap %v", got, tc.wantIs)
			}
			appErr := domain.AsAppError(got)
			if appErr.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", appErr.Code, tc.wantCode)
			}
			if appErr.HTTPStatus() != tc.wantHTTP {
				t.Fatalf("status = %d, want %d", appErr.HTTPStatus(), tc.wantHTTP)
			}
		})
	}
}

// TestTranslateNodeError pins the node mapping, whose unique index is the prefix.
func TestTranslateNodeError(t *testing.T) {
	cases := []struct {
		name     string
		in       error
		wantIs   error
		wantCode string
	}{
		{"no rows becomes node not found", pgx.ErrNoRows, domain.ErrNodeNotFound, "NOT_FOUND"},
		{"unique violation becomes prefix taken", &pgconn.PgError{
			Code: "23505", ConstraintName: "idx_provider_nodes_prefix",
		}, domain.ErrNodePrefixTaken, "CONFLICT"},
		{"check violation becomes validation", &pgconn.PgError{Code: "23514"}, nil, "VALIDATION_ERROR"},
		{"unknown error is wrapped", errors.New("eof"), nil, "INTERNAL_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateNodeError(tc.in)
			if got == nil {
				t.Fatal("translateNodeError() = nil, want an error")
			}
			if tc.wantIs != nil && !errors.Is(got, tc.wantIs) {
				t.Fatalf("translateNodeError() = %v, want it to wrap %v", got, tc.wantIs)
			}
			if code := domain.AsAppError(got).Code; code != tc.wantCode {
				t.Fatalf("code = %q, want %q", code, tc.wantCode)
			}
		})
	}
}

// TestTranslateUniqueViolationDoesNotLeakDriverText asserts the mapped error stays
// matchable by errors.Is and that the client-facing message is the domain sentence,
// never the driver's (AGENTS.md §1.3).
func TestTranslateUniqueViolationDoesNotLeakDriverText(t *testing.T) {
	const driverText = `duplicate key value violates unique constraint "idx_upstream_endpoints_provider_label"`
	got := translateEndpointError(&pgconn.PgError{
		Code: "23505", ConstraintName: "idx_upstream_endpoints_provider_label", Message: driverText,
	})

	public := domain.AsAppError(got)
	if public.Message != domain.ErrEndpointExists.Message {
		t.Fatalf("message = %q, want the domain message %q", public.Message, domain.ErrEndpointExists.Message)
	}
	if public.Message == driverText {
		t.Fatal("the driver message must never reach the client")
	}
}

// TestBulkRowErrorAttributesTheRow pins the structural contract the service and
// handler match on to report a batch row by index (§8.1).
func TestBulkRowErrorAttributesTheRow(t *testing.T) {
	cause := domain.ErrEndpointExists
	err := &BulkRowError{Index: 3, Err: cause}

	if !errors.Is(err, cause) {
		t.Fatal("BulkRowError must unwrap to its cause")
	}
	index, ok := err.BulkRowIndex()
	if !ok || index != 3 {
		t.Fatalf("BulkRowIndex() = (%d, %v), want (3, true)", index, ok)
	}
}
