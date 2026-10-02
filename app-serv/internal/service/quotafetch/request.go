// One outbound quota read, shared by every family.
//
// @file      internal/service/quotafetch/request.go
// @for       Performs one provider quota request and turns a refused or failed answer into the reference's soft message.
// @uses      internal/service/quotafetch, net/http, encoding/json
// @reason    Each family repeats the same three moves — send one bounded request, treat a
//
//	401/403 as a dead credential, treat a non-2xx as a sentence the card renders
//	rather than a failure of the page. Reading them once keeps a family from
//	inventing a fourth behaviour where the reference agreed on one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxErrorBody bounds how much of a provider's error body is quoted back into a client-
// facing message. The body is the provider's words, not ours, and an unbounded page of
// HTML from a captive portal would otherwise reach the operator's card.
const maxErrorBody = 200

// usageResponse is one provider answer: the status, the headers the family may read
// instead of a body (Groq publishes its quota only in x-ratelimit-*), and the body.
type usageResponse struct {
	status int
	header http.Header
	body   []byte
}

// requestUsage sends one quota read. The caller owns the deadline — Fetch bounds every
// family and this helper adds no second timeout of its own — and the response body is
// always closed here, including on a decode the family never reaches.
//
// Whatever this call put on the wire as a credential is scrubbed out of what comes back.
// Several providers quote the request they refused in their 4xx body, and a Go transport
// error carries the URL, so an unsent-through scrub would let a live bearer token land in
// a card sentence and — because the poll worker caches every answer — in the database
// behind it. Scrubbing here means no family has to remember to.
func requestUsage(ctx context.Context, method, endpoint string, headers map[string]string, body string) (usageResponse, error) {
	secrets := presentedSecrets(endpoint, headers)

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return usageResponse{}, errors.New(scrubText(err.Error(), secrets))
	}
	request.Header.Set("Accept", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return usageResponse{}, errors.New(scrubText(err.Error(), secrets))
	}
	defer func() { _ = response.Body.Close() }()

	quoted, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return usageResponse{}, errors.New(scrubText(err.Error(), secrets))
	}
	for _, secret := range secrets {
		quoted = bytes.ReplaceAll(quoted, []byte(secret), []byte("[redacted]"))
	}
	return usageResponse{status: response.StatusCode, header: response.Header, body: quoted}, nil
}

// credentialHeaderNames are the headers whose value IS the secret. Identification headers
// (product name, editor version, API version) are deliberately absent: they are not
// secrets, and scrubbing them would mangle the sentences providers return.
var credentialHeaderNames = map[string]struct{}{
	"authorization": {}, "x-api-key": {}, "apikey": {}, "x-goog-api-key": {},
	"x-amz-security-token": {}, "api-key": {}, "x-msh-api-key": {},
}

// minScrubLength is the shortest value worth scrubbing. Real credentials are long, and a
// short header value ("Bearer x", a version string) that happens to occur in a provider's
// English sentence would otherwise be replaced inside that sentence — mangling the one
// thing the operator needs to read.
const minScrubLength = 9

// presentedSecrets lists the values this call handed the provider that must never read
// back as text: each credential header in full and with its scheme stripped, plus any
// query parameter whose name says it carries one.
func presentedSecrets(endpoint string, headers map[string]string) []string {
	secrets := make([]string, 0, len(headers)+1)
	for key, value := range headers {
		if _, credential := credentialHeaderNames[strings.ToLower(key)]; !credential {
			continue
		}
		trimmed := strings.TrimSpace(value)
		if len(trimmed) < minScrubLength {
			continue
		}
		secrets = append(secrets, trimmed)
		// "Bearer x" and "token x": the scheme is not the secret, and a provider that
		// echoes only the bare token would survive a whole-value match.
		if space := strings.IndexByte(trimmed, ' '); space > 0 {
			if bare := strings.TrimSpace(trimmed[space:]); len(bare) >= minScrubLength {
				secrets = append(secrets, bare)
			}
		}
	}

	parsed, err := url.Parse(endpoint)
	if err == nil {
		for name, values := range parsed.Query() {
			lower := strings.ToLower(name)
			if !strings.Contains(lower, "key") && !strings.Contains(lower, "token") && !strings.Contains(lower, "secret") {
				continue
			}
			for _, value := range values {
				if len(strings.TrimSpace(value)) >= minScrubLength {
					secrets = append(secrets, strings.TrimSpace(value))
				}
			}
		}
	}
	return secrets
}

// scrubText removes every presented secret from one string, used for the transport errors
// that quote the request URL back.
func scrubText(text string, secrets []string) string {
	for _, secret := range secrets {
		text = strings.ReplaceAll(text, secret, "[redacted]")
	}
	return text
}

// bearer returns the header set a bearer-authenticated read needs, with the caller's
// extra identification merged in. It allocates a fresh map so a family's headers never
// mutate the registry entry's own.
func bearer(token string, extra map[string]string) map[string]string {
	headers := make(map[string]string, len(extra)+1)
	for key, value := range extra {
		headers[key] = value
	}
	headers["Authorization"] = "Bearer " + token
	return headers
}

// softFailure maps a non-2xx answer to the sentence the reference renders on the card,
// and reports false when the answer was good enough to parse. A dead credential is
// named as one because the operator's next action differs from a 500: re-authenticate
// rather than wait. `display` is the family's own region word, which each family already
// carries for its messages.
// hardFailure is `softFailure` for the families whose reference handler throws: same
// sentence, same status handling, and the one bit that tells the worker this answer is a
// provider failing rather than a provider speaking.
func (r usageResponse) hardFailure(display string) (Result, bool) {
	failure, refused := r.softFailure(display)
	failure.Failed = refused
	return failure, refused
}

func (r usageResponse) softFailure(display string) (Result, bool) {
	if r.status >= 200 && r.status < 300 {
		return Result{}, false
	}

	if r.status == http.StatusUnauthorized || r.status == http.StatusForbidden {
		return Result{Message: fmt.Sprintf("%s credential invalid or expired (%d).", display, r.status)}, true
	}

	detail := providerDetail(r.body)
	if detail != "" {
		detail = ": " + detail
	}
	return Result{Message: fmt.Sprintf("%s quota API error (%d)%s", display, r.status, detail)}, true
}

// providerDetail quotes a provider's own rejection, with one exception: markup. A moved
// endpoint answers a 404 with an HTML page, and the poll worker stores whatever sentence
// comes back, so that page would land on the state row and be printed on the operator's
// card — a wall of markup where one sentence belongs. A short plain-text answer ("upstream
// down") is kept, because it is the provider's reason and the operator can act on it.
func providerDetail(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || trimmed[0] == '<' {
		return ""
	}
	if markup := strings.Index(strings.ToLower(trimmed), "<html"); markup >= 0 {
		trimmed = strings.TrimSpace(trimmed[:markup])
	}
	if len(trimmed) > maxErrorBody {
		trimmed = trimmed[:maxErrorBody]
	}
	return trimmed
}
