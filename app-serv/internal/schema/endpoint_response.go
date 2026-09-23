// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/endpoint_response.go
// @for       The upstream endpoint and key response shapes plus the bulk reports
//
//	(SPEC-API-001 §7.5, §8.1).
//
// @uses      standard library only.
// @reason    §6 requires the stored OAuth state to be redacted on read and §8.1
//
//	requires a batch to report every row by index, so these shapes are the
//	contract for what a client may see. They are separated from the request
//	DTOs because a response shape has no validation tags and one file
//	holding both would exceed the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

// EndpointKeyResponse is the key shape for list, detail, and create. It carries
// key_hint and health only: the stored value is never present (§7.5).
type EndpointKeyResponse struct {
	ID                string  `json:"id"`
	EndpointID        string  `json:"endpoint_id"`
	Label             string  `json:"label"`
	KeyHint           string  `json:"key_hint"`
	Priority          int     `json:"priority"`
	Status            string  `json:"status"`
	Available         bool    `json:"available"`
	LastUsedAt        *string `json:"last_used_at,omitempty"`
	LastError         string  `json:"last_error,omitempty"`
	ConsecutiveErrors int     `json:"consecutive_errors"`
	RateLimitedUntil  *string `json:"rate_limited_until,omitempty"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// EndpointOAuthResponse is the redacted OAuth state (§6: oauth is redacted on
// read). Presence flags stand in for the tokens, which never cross the wire.
type EndpointOAuthResponse struct {
	ExpiresAt       *string  `json:"expires_at,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`
	ProjectID       string   `json:"project_id,omitempty"`
	AccountID       string   `json:"account_id,omitempty"`
	AccountEmail    string   `json:"account_email,omitempty"`
	LastRefreshAt   *string  `json:"last_refresh_at,omitempty"`
	HasAccessToken  bool     `json:"has_access_token"`
	HasRefreshToken bool     `json:"has_refresh_token"`
}

// EndpointTestStatusResponse is the last connectivity result (§7.5).
type EndpointTestStatusResponse struct {
	State     string  `json:"state"`
	LatencyMS int     `json:"latency_ms"`
	Message   string  `json:"message,omitempty"`
	CheckedAt *string `json:"checked_at,omitempty"`
}

// EndpointResponse is the list and detail shape. ProviderName is carried so the
// panel's endpoint table does not have to join the registry by hand, and Keys is
// populated on detail only.
type EndpointResponse struct {
	ID               string                      `json:"id"`
	ProviderID       string                      `json:"provider_id"`
	ProviderName     string                      `json:"provider_name,omitempty"`
	Label            string                      `json:"label"`
	AuthType         string                      `json:"auth_type"`
	Priority         int                         `json:"priority"`
	Status           string                      `json:"status"`
	Account          EndpointAccountResponse     `json:"account"`
	OAuth            *EndpointOAuthResponse      `json:"oauth,omitempty"`
	TestStatus       *EndpointTestStatusResponse `json:"test_status,omitempty"`
	RateLimitedUntil *string                     `json:"rate_limited_until,omitempty"`
	LastUsedAt       *string                     `json:"last_used_at,omitempty"`
	KeyCount         int                         `json:"key_count"`
	ActiveKeyCount   int                         `json:"active_key_count"`
	Available        bool                        `json:"available"`
	Keys             []EndpointKeyResponse       `json:"keys,omitempty"`
	CreatedAt        string                      `json:"created_at"`
	UpdatedAt        string                      `json:"updated_at"`

	// The connection-parity fields (draft 017 §4.1b). GlobalPriority is 0 when
	// unset; DefaultModel and ProxyPoolID are empty when unset. LastError is
	// absent until an upstream failure that was not a connectivity test, and its
	// message is scrubbed of credential material before it is stored.
	GlobalPriority      int                    `json:"global_priority"`
	DefaultModel        string                 `json:"default_model"`
	ConsecutiveUseCount int                    `json:"consecutive_use_count"`
	ProxyPoolID         string                 `json:"proxy_pool_id"`
	LastError           *EndpointErrorResponse `json:"last_error,omitempty"`
}

// EndpointErrorResponse is the last non-test upstream failure. It is an object
// rather than three sibling fields because a client renders it as one block, and
// it is absent entirely when the endpoint has not failed.
type EndpointErrorResponse struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	At      string `json:"at"`
}

// EndpointList wraps a page of endpoints with the pagination meta block (§4).
type EndpointList struct {
	Data []EndpointResponse `json:"data"`
	Meta Page               `json:"meta"`
}

// EndpointKeyList wraps a page of keys with the pagination meta block (§4).
type EndpointKeyList struct {
	Data []EndpointKeyResponse `json:"data"`
	Meta Page                  `json:"meta"`
}

// BulkResultRow reports one batch row by index (§8.1). Nothing is written unless
// every row passes, so a row carries an id only on an accepted batch.
type BulkResultRow struct {
	Index int    `json:"index"`
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// BulkError repeats the §8 code/message pair inside a bulk body. A refused batch
// is an error response, so it must carry the code a client maps on, and it must
// also carry the per-row detail §8.1 requires; a bare envelope could not hold
// both.
type BulkError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BulkEndpointResponse is the answer to POST /endpoints/bulk (§7.5).
type BulkEndpointResponse struct {
	Error   *BulkError         `json:"error,omitempty"`
	Created []EndpointResponse `json:"created"`
	Results []BulkResultRow    `json:"results"`
}

// BulkRefusal is the answer to a refused batch (§8.1): the §8 code and message, plus
// every row's index when the offending one could be named. It exists because a
// refusal has to carry both — a bare envelope could not name the row and a bare row
// list could not name the code.
type BulkRefusal struct {
	Error   BulkError       `json:"error"`
	Results []BulkResultRow `json:"results"`
}

// BulkKeyResponse is the answer to POST /endpoints/{id}/keys/bulk.
type BulkKeyResponse struct {
	Error   *BulkError            `json:"error,omitempty"`
	Created []EndpointKeyResponse `json:"created"`
	Results []BulkResultRow       `json:"results"`
}

// BulkOAuthRow is one imported account: the endpoint identity and a hint only,
// never the token (§8.1).
type BulkOAuthRow struct {
	Index     int                     `json:"index"`
	ID        string                  `json:"id,omitempty"`
	Label     string                  `json:"label,omitempty"`
	Account   EndpointAccountResponse `json:"account"`
	TokenHint string                  `json:"token_hint,omitempty"`
	Updated   bool                    `json:"updated"`
	Error     string                  `json:"error,omitempty"`
}

// BulkOAuthResponse is the answer to POST /providers/{provider_id}/oauth/bulk.
type BulkOAuthResponse struct {
	Error   *BulkError      `json:"error,omitempty"`
	Created []BulkOAuthRow  `json:"created"`
	Results []BulkResultRow `json:"results"`
}
