// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_identity.go
// @for       The account identity a userinfo endpoint reports, decoded from
//
//	every field spelling providers disagree on.
//
// @uses      encoding/json, internal/domain.
// @reason    SPEC-API-001 §8.1 makes a re-import an update rather than a
//
//	duplicate, so the callback has to recognize an account it already
//	holds. That match reads email, then workspace id, then login, and
//	providers disagree on which of those they send and whether the id is
//	a JSON number or a JSON string, so the tolerance lives in one type
//	rather than in a branch at each call site.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// OAuthIdentity is the account identity a userinfo endpoint reports. Providers
// disagree on field names (sub, id, login), so all known spellings are decoded
// and the first non-empty one wins.
type OAuthIdentity struct {
	Sub   string     `json:"sub"`
	ID    identityID `json:"id"`
	Login string     `json:"login"`
	Name  string     `json:"name"`
	Email string     `json:"email"`
}

// identityID decodes an account id that one provider sends as a JSON number and
// the next sends as a JSON string. Both spellings become the same string, so a
// string id cannot fail the whole userinfo decode and, with it, the connect.
type identityID string

func (id *identityID) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*id = ""
		return nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		*id = identityID(asString)
		return nil
	}
	*id = identityID(string(raw))
	return nil
}

// Account returns the identity as an endpoint account value: the email the
// account is known by, falling back to login and then sub, the display name,
// and the provider's own id as the workspace id.
func (i OAuthIdentity) Account() domain.EndpointAccount {
	account := domain.EndpointAccount{Name: i.Name, Email: i.Email}
	switch {
	case i.Email == "" && i.Login != "":
		account.Email = i.Login
	case i.Email == "" && i.Sub != "":
		account.Email = i.Sub
	}
	if account.Name == "" {
		account.Name = i.Login
	}
	if account.WorkspaceID == "" {
		account.WorkspaceID = string(i.ID)
	}
	return account
}
