// Handlers serve the HTTP surface of app-serv.
//
// @file      internal/handler/dataplane_auth_order_test.go
// @for       The refusal an unauthenticated caller gets before its body is read.
// @uses      net/http, testing, internal/dataplane
// @reason    Two data-plane routes read and validated the body before checking the key, so a
//
//	caller with no credential could cost an 8 MiB read and learn the schema from the
//	VALIDATION_ERROR it got back. The rest of the plane authenticates first, and this pins
//	that these two now do the same.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-04
package handler

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

func TestDataPlaneRoutesAuthenticateBeforeReadingTheBody(t *testing.T) {
	refusing := stubAuthenticator{refuse: dataplane.ErrUnauthorized()}
	// A malformed body is the probe: if it were read first it would answer 400 and
	// hand the caller the schema. Neither handler touches its service, so a nil one
	// keeps this test about ordering rather than about wiring.
	cases := []struct {
		name     string
		target   string
		body     string
		endpoint http.HandlerFunc
	}{
		{
			name:     "count_tokens",
			target:   "/api/v1/messages/count_tokens",
			body:     `{"model":`,
			endpoint: NewTokenCountHandler(service.NewTokenCountService(), refusing).Count,
		},
		{
			name:     "embeddings",
			target:   "/api/v1/embeddings",
			body:     `{"model":"text-embedding-3-small","input":`,
			endpoint: NewEmbeddingsHandler(nil, refusing).Embed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, http.MethodPost, tc.target, tc.body, tc.endpoint)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 before the body is read (body: %s)", rr.Code, rr.Body.String())
			}
		})
	}
}
