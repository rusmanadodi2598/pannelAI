// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_identity.go
// @for       The account a COSY signature is made as: the vendor's own user id behind
//
//	the credential a connection holds.
//
// @uses      context, encoding/json, fmt, net/http, strings, sync, time.
// @reason    Every signed Qoder request carries an encrypted identity whose `uid` is
//
//	the vendor's user id, and the vendor refuses a request without it. A device
//	login learns that id during its own flow and stores it, but a Personal Access
//	Token pasted into the panel is only a token: nothing on the connection row
//	names the account behind it. Reading it from the vendor at sign time is
//	what makes a PAT connection usable at all — the reference does the same,
//	and refuses with "reconnect the account" when the id cannot be had.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-28
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// qoderUserInfoPath is the identity endpoint on the openapi host, used when the
// registry entry names none.
const qoderUserInfoPath = "/api/v1/userinfo"

// qoderIdentityTTL bounds how long a resolved user id is reused. The vendor states no
// expiry on this answer, so the cache is bounded by age; an hour is well inside any
// job token's life, so a renewal cannot serve an identity for a dead token.
const qoderIdentityTTL = time.Hour

// qoderAccount is the non-secret identity one credential belongs to.
type qoderAccount struct {
	UserID string
	Email  string
}

type qoderIdentityEntry struct {
	account   qoderAccount
	fetchedAt time.Time
}

// qoderIdentityCache is the per-connector identity cache, keyed by a digest of the
// bearer asked with. It is safe for concurrent use and holds no per-request state:
// signing happens on every request, so an unbounded read of the vendor's identity
// endpoint would be a second call per call.
type qoderIdentityCache struct {
	mu      sync.Mutex
	entries map[string]qoderIdentityEntry
	now     func() time.Time
}

func newQoderIdentityCache() *qoderIdentityCache {
	return &qoderIdentityCache{entries: map[string]qoderIdentityEntry{}, now: time.Now}
}

func (c *qoderIdentityCache) read(key string, now time.Time) (qoderAccount, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || now.Sub(entry.fetchedAt) >= qoderIdentityTTL {
		return qoderAccount{}, false
	}
	return entry.account, true
}

func (c *qoderIdentityCache) write(key string, account qoderAccount, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = qoderIdentityEntry{account: account, fetchedAt: now}
}

// signingIdentity is the account a signed request speaks as. A stored user id wins,
// because a device login already learned it during its own flow; a credential that
// carries none — a Personal Access Token added by hand — is resolved against the
// vendor here. An identity that cannot be had is refused rather than sent as an
// empty `uid`, which the vendor answers as a rejected signature and the operator
// reads as a broken credential.
func (c *Qoder) signingIdentity(ctx context.Context, cred Credential, token string) (cosyIdentity, error) {
	account := qoderAccount{
		UserID: strings.TrimSpace(cred.ProjectID),
		Email:  strings.TrimSpace(cred.Account),
	}
	if account.UserID == "" {
		resolved, err := c.accountFor(ctx, token)
		if err != nil {
			return cosyIdentity{}, err
		}
		account.UserID = resolved.UserID
		if account.Email == "" {
			account.Email = resolved.Email
		}
	}
	if account.UserID == "" {
		return cosyIdentity{}, fmt.Errorf(
			"provider %s: the vendor names no account for this credential, so its requests cannot be signed; reconnect it", c.entry.ID)
	}
	return cosyIdentity{
		UserID:    account.UserID,
		AuthToken: token,
		Email:     account.Email,
		MachineID: strings.TrimSpace(cred.Metadata[MetadataMachineID]),
	}, nil
}

// accountFor reads the identity one bearer belongs to, through the cache.
func (c *Qoder) accountFor(ctx context.Context, token string) (qoderAccount, error) {
	endpoint := c.userInfoURL()
	key := hashCredentialToken(token)
	if cached, ok := c.identity.read(key, c.identity.now()); ok {
		return cached, nil
	}

	account, err := c.fetchAccount(ctx, endpoint, token)
	if err != nil {
		return qoderAccount{}, err
	}
	c.identity.write(key, account, c.identity.now())
	return account, nil
}

// userInfoURL is the identity endpoint: the entry's declared one when it has it, the
// openapi host's otherwise. An entry with neither cannot be built at all —
// NewQoder needs the openapi base for the exchange — so this always names a host.
// The two agree for both Qoder sites today, and the declared one is what the
// registry keeps truthful.
func (c *Qoder) userInfoURL() string {
	if declared := strings.TrimSpace(c.entry.OAuth.UserInfoURL); declared != "" {
		return declared
	}
	base := strings.TrimSuffix(strings.TrimSpace(c.entry.OAuth.OpenAPIBaseURL), "/")
	if base == "" {
		return ""
	}
	return base + qoderUserInfoPath
}

// fetchAccount asks the vendor who this bearer is. The answer is read under the three
// names the vendor has used for the same value across its surfaces — the reference
// reads that same set, because a client that picks one of them breaks when the site
// it talks to changes spelling.
func (c *Qoder) fetchAccount(ctx context.Context, endpoint, token string) (qoderAccount, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return qoderAccount{}, fmt.Errorf("provider %s: the identity request could not be built", c.entry.ID)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "qodercli/"+qoderIDEVersion)

	response, err := c.client.Do(request)
	if err != nil {
		return qoderAccount{}, fmt.Errorf("provider %s: the account identity could not be read: %w", c.entry.ID, err)
	}
	defer func() {
		// reason: the body is read in full below, so a close error adds nothing.
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return qoderAccount{}, fmt.Errorf(
			"provider %s: the account identity was refused with http %d", c.entry.ID, response.StatusCode)
	}

	var payload struct {
		ID          string `json:"id"`
		UserID      string `json:"userId"`
		UserIDSnake string `json:"user_id"`
		Email       string `json:"email"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, qoderCatalogBodyByte)).Decode(&payload); err != nil {
		return qoderAccount{}, fmt.Errorf("provider %s: the account identity could not be read: %w", c.entry.ID, err)
	}
	return qoderAccount{
		UserID: firstNonEmpty(payload.ID, payload.UserID, payload.UserIDSnake),
		Email:  strings.TrimSpace(payload.Email),
	}, nil
}

// firstNonEmpty is the vendor's own fallback chain, named because three spellings of
// one value read as a choice otherwise.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
