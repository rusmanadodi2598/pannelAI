// Package domain holds the business entities and value objects of app-serv.
//
// @file      internal/domain/quota_account.go
// @for       One connection as the quota screen's card list sees it.
// @uses      strings.
// @reason    The quota cards page over provider groups, and a group is made of
//
//	accounts — which exist whether or not any traffic has been routed
//	through them. Deriving the card list from counted windows instead hid
//	every account that had not served a request yet, so a provider could
//	be configured, polled, and published about, and still render nothing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-02
package domain

import "strings"

// QuotaAccount is one connection named by the collection page, independent of what
// this gateway has counted for it.
type QuotaAccount struct {
	EndpointID string
	ProviderID string
}

// Validate refuses an account the screen cannot render: an endpoint with no identity is
// not a card, and a lane whose provider is unknown is reported by the empty provider id
// the credential-free virtual endpoint already carries.
func (a QuotaAccount) Validate() error {
	if strings.TrimSpace(a.EndpointID) == "" {
		return NewValidationError("quota account requires an endpoint_id")
	}
	return nil
}
