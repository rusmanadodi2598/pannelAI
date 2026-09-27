// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder.go
// @for       The Qoder connector: the host a credential is served from, and the
//
//	COSY signature that replaces a bearer token.
//
// @uses      io, net/http, net/url, strings, internal/registry.
// @reason    Qoder breaks the two assumptions the default connector makes: the
//
//	inference host depends on which kind of token the account holds, and
//	the credential is not placed in a header but woven into a signature
//	over the exact bytes that go out. Both rules belong to the provider,
//	so the core never learns either one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Qoder is the connector for the Qoder and Qoder CN providers. It holds the registry
// entry it was built for and the seam that turns a stored credential into a
// signature, so it carries no per-request state and is safe to share.
type Qoder struct {
	Base

	entry  registry.Provider
	tokens QoderJobToken
	signer cosySigner
}

// NewQoder builds the connector for one registry entry. The exchange client is the
// caller's, so the composition root decides whether a Personal Access Token exchange
// rides the process egress guard; a nil client gets one bounded by the exchange's own
// timeout.
func NewQoder(entry registry.Provider, client *http.Client) (*Qoder, error) {
	if entry.OAuth == nil {
		return nil, fmt.Errorf("provider %s: a qoder connector needs an oauth block", entry.ID)
	}
	tokens, err := NewQoderJobTokenClient(entry.OAuth.OpenAPIBaseURL, client)
	if err != nil {
		return nil, fmt.Errorf("provider %s: %w", entry.ID, err)
	}
	return &Qoder{
		Base:   Base{ID: entry.ID, Auth: entry.AuthType, Format: entry.Transport.Format},
		entry:  entry,
		tokens: tokens,
		signer: qoderCosy,
	}, nil
}

// Endpoint returns the inference URL for this call.
//
// The registry entry names the full chat URL, so the only decision here is the host,
// and the host follows the kind of credential the account holds: on intl a Personal
// Access Token is exchanged for a job token before it signs anything, and job-token
// traffic is served from a different gateway than device traffic (draft 036 §5). The
// choice is made from the stored credential rather than by performing the exchange
// here, because a URL decision should not be the moment the gateway makes an outbound
// call. A CN entry declares one gateway for every kind, so the swap never matches it.
func (c *Qoder) Endpoint(_ Request, cred Credential) (string, error) {
	base := strings.TrimSpace(c.entry.Transport.BaseURL)
	if base == "" {
		return "", fmt.Errorf("provider %s: the chat base url is not declared", c.entry.ID)
	}
	value, err := c.credential(cred)
	if err != nil {
		return "", err
	}
	if !isQoderJobCredential(value) {
		return base, nil
	}
	return qoderSwapHost(base, qoderChatBaseIntlDevice, qoderChatBaseIntlJob)
}

// ApplyAuth signs the request. The body bytes are read here because the signature
// covers exactly what leaves the process: a hash taken before the transport finished
// shaping the body would describe a request nobody sent.
func (c *Qoder) ApplyAuth(req *http.Request, cred Credential) error {
	if req == nil {
		return fmt.Errorf("provider %s: there is no request to sign", c.entry.ID)
	}
	body, err := readForSigning(req)
	if err != nil {
		return err
	}
	token, err := c.signingToken(req.Context(), cred)
	if err != nil {
		return err
	}
	header, err := c.signer.headers(body, req.URL.String(), cosyIdentity{
		UserID:    cred.ProjectID,
		AuthToken: token,
		Email:     cred.Account,
		MachineID: cred.Metadata[MetadataMachineID],
	})
	if err != nil {
		return err
	}
	for name, values := range header {
		for _, value := range values {
			req.Header.Set(name, value)
		}
	}
	// reason: a compressed answer trips the CDN's signature validation, so the
	// official client and the reference both insist on identity encoding.
	req.Header.Set("Accept-Encoding", "identity")
	return nil
}

// signingToken is the credential a signed request presents: the stored device or job
// token, or — for a Personal Access Token — an exchanged job token. A PAT never
// reaches the wire itself, so there is no fallback path here by design.
func (c *Qoder) signingToken(ctx context.Context, cred Credential) (string, error) {
	value, err := c.credential(cred)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(value, qoderTokenPAT) {
		return value, nil
	}
	return c.tokens.JobToken(ctx, value)
}

// credential reads the one value this account presents and refuses an account that
// has none. The family decides nothing here — a Qoder device token and an exchanged
// job token are both bearer material, and a static key is a Personal Access Token —
// so only its emptiness matters.
func (c *Qoder) credential(cred Credential) (string, error) {
	_, value := cred.family()
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("provider %s: the account carries no credential", c.entry.ID)
	}
	return value, nil
}

// isQoderJobCredential reports whether a stored credential ends up as a job token on
// the wire: either one already exchanged, or a Personal Access Token that will be.
func isQoderJobCredential(value string) bool {
	return strings.HasPrefix(value, qoderTokenPAT) || strings.HasPrefix(value, qoderTokenJob)
}

// qoderSwapHost rewrites one absolute URL's scheme and host onto a target base,
// keeping the path and query the registry declared. Both halves are compared as parsed
// URLs rather than as string prefixes, so a host that merely ends with the expected
// name cannot be rewritten by accident.
func qoderSwapHost(rawURL, fromBase, toBase string) (string, error) {
	current, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("provider: the chat url could not be read: %w", err)
	}
	from, err := url.Parse(fromBase)
	if err != nil || from.Host == "" {
		return "", fmt.Errorf("provider: the device host is not absolute")
	}
	to, err := url.Parse(toBase)
	if err != nil || to.Host == "" {
		return "", fmt.Errorf("provider: the job host is not absolute")
	}
	if !strings.EqualFold(current.Host, from.Host) {
		return rawURL, nil
	}
	current.Scheme = to.Scheme
	current.Host = to.Host
	return current.String(), nil
}

// qoderMaxSignBodyBytes bounds what the signer will hold in order to hash one body.
// The vendor's own payload ceiling is 6MB (draft 036 §5), so this is that limit with
// room for the envelope around it.
const qoderMaxSignBodyBytes = 8 << 20

// readForSigning returns the body a request will send without consuming it. A request
// built by the transport always carries GetBody; one hand-built is read and restored,
// so a caller outside the transport is still signed over the right bytes.
func readForSigning(req *http.Request) ([]byte, error) {
	if req.GetBody != nil {
		reader, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("provider: the request body could not be re-read: %w", err)
		}
		defer func() {
			// reason: the reader wraps a byte slice; close cannot fail usefully.
			_ = reader.Close()
		}()
		body, err := io.ReadAll(io.LimitReader(reader, qoderMaxSignBodyBytes))
		if err != nil {
			return nil, fmt.Errorf("provider: the request body could not be read: %w", err)
		}
		return body, nil
	}
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, qoderMaxSignBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("provider: the request body could not be read: %w", err)
	}
	req.Body = io.NopCloser(strings.NewReader(string(body)))
	return body, nil
}
