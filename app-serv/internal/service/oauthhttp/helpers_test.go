// Package oauthhttp performs the OAuth rounds the flow service orchestrates.
//
// @file      internal/service/oauthhttp/helpers_test.go
// @for       The assertions shared by the transport's own tests.
// @uses      internal/domain, testing.
// @reason    Each moved file needs the same domain-error check, and repeating a five-line helper per file is how one assertion starts to disagree with the others.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package oauthhttp

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// mustAppError pins one domain error code, so a refusal stays attributable.
func mustAppError(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", code)
	}
	if got := domain.AsAppError(err).Code; got != code {
		t.Fatalf("code = %q, want %q (error: %v)", got, code, err)
	}
}
