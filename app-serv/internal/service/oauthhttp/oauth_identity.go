// Package oauthhttp performs the OAuth rounds the flow service orchestrates.
//
// @file      internal/service/oauthhttp/oauth_identity.go
// @for       The account identity a userinfo endpoint reports, decoded from every field spelling providers disagree on.
// @uses      encoding/json, internal/domain.
// @reason    SPEC-API-001 §8.1 makes a re-import an update rather than a duplicate, so the callback has to recognize an account it already holds. That match reads email, then workspace id, then login, and providers disagree on which of those they send and whether the id is a JSON number or a JSON string, so the tolerance lives in one type rather than in a branch at each call site.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package oauthhttp

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
	email := i.Email
	switch {
	case email == "" && i.Login != "":
		email = i.Login
	case email == "" && i.Sub != "":
		email = i.Sub
	}
	name := i.Name
	if name == "" {
		name = i.Login
	}
	workspaceID := string(i.ID)
	account, err := domain.NewEndpointAccount(domain.EndpointAccountInput{Name: name, Email: email, WorkspaceID: workspaceID})
	if err == nil {
		return account
	}
	// An address the vendor spelled with a space inside still has a name and a
	// provider id worth storing, so only the unusable field is dropped.
	account, _ = domain.NewEndpointAccount(domain.EndpointAccountInput{Name: name, WorkspaceID: workspaceID})
	return account
}
