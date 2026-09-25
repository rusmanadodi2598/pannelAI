// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/credential_free.go
// @for       The one answer to "does this provider answer without a credential?"
// @uses      strings (standard library only).
// @reason    Two callers have to agree on this question, so it is asked here once
//
//	(AGENTS.md §1.1 keeps it out of load.go, which is about decoding):
//	routing decides whether it may synthesize a virtual endpoint for a
//	provider the operator configured none for (draft 029 §4.8 F8), and
//	the catalog's `?active=true` filter decides whether a provider with no
//	stored row is still one the router can serve. When those two answered
//	separately, the filter hid the whole OpenCode free lane from the panel
//	while the router served it with 200 — so a credential-free provider
//	looked like it needed configuration it did not need.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package registry

import "strings"

// NeedsNoCredential reports whether this entry answers without any credential,
// which is the property the reference's `noAuth` flag carries and the one the
// data plane reads to synthesize a virtual endpoint for a provider the operator
// configured none for (SPEC-API-001 §7.4; draft 029 §4.8 F8).
//
// Both document spellings are read because the reference writes `noAuth` on the
// provider for some entries and on the transport for others, and `BuildAuthType`
// folds the two into `AuthType` at load; reading the derived value as well keeps
// an entry that declared `auth_type: no_auth` directly from being reported as
// keyed.
func (p Provider) NeedsNoCredential() bool {
	return p.NoAuth || p.Transport.NoAuth || strings.TrimSpace(p.AuthType) == AuthNone
}
