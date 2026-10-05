// The CodeBuddy families (CN and Intl): one Tencent billing endpoint per region.
//
// @file      internal/service/quotafetch/codebuddy.go
// @for       Asks one CodeBuddy region's billing endpoint with the identity that region's entry declares.
// @uses      internal/service/quotafetch, net/http, encoding/json
// @reason    The billing answer arrives as a doubled envelope behind a refusal-prone POST, so the
//
//	request and its two soft failures belong here; how the credit packages inside it
//	become windows is read by codebuddy_packs.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-25
package quotafetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// codebuddyBodyLimit caps one quota answer read from the vendor. The shared
// quota reader already holds reads to 1 MiC (request.go); this family builds its
// own request, so it carries the same ceiling rather than an unbounded decode.
const codebuddyBodyLimit = 1 << 20

// fetchCodeBuddy binds the shared billing reader to one region's endpoint and headers.
func fetchCodeBuddy(family codebuddyFamily) func(context.Context, Credentials) Result {
	return func(ctx context.Context, creds Credentials) Result {
		return readCodeBuddy(ctx, family, creds)
	}
}

type codebuddyFamily struct {
	id      string            // the registry family key, which is where its usage endpoint is declared
	name    string            // human-facing region word for messages
	headers map[string]string // the transport headers the billing endpoint expects, as the registry declares them
}

var codebuddyCN = codebuddyFamily{
	id:   "codebuddy-cn",
	name: "CN",
	headers: map[string]string{
		"User-Agent":          "CLI/2.108.1 CodeBuddy/2.108.1",
		"X-Product":           "SaaS",
		"X-IDE-Type":          "CLI",
		"X-IDE-Name":          "CLI",
		"x-requested-with":    "XMLHttpRequest",
		"x-codebuddy-request": "1",
	},
}

var codebuddyIntl = codebuddyFamily{
	id:   "codebuddy-intl",
	name: "Intl",
	headers: map[string]string{
		"User-Agent":          "IDE/2.108.1 CodeBuddy/2.108.1",
		"X-Product":           "SaaS",
		"X-IDE-Type":          "IDE",
		"X-IDE-Name":          "IDE",
		"x-requested-with":    "XMLHttpRequest",
		"x-codebuddy-request": "1",
	},
}

func readCodeBuddy(ctx context.Context, family codebuddyFamily, creds Credentials) Result {
	endpoint := endpointFor(usageEndpoint(creds, family.id), creds.Endpoint)

	token := creds.AccessToken
	if token == "" {
		token = creds.APIKey
	}
	if token == "" {
		return Result{Message: fmt.Sprintf("CodeBuddy %s credential not available.", family.name)}
	}

	payload, err := postCodeBuddy(ctx, endpoint, codebuddyHeaders(family, creds), token)
	if err != nil {
		switch detail := err.(type) {
		case refusedCredential:
			return Result{Message: fmt.Sprintf("CodeBuddy %s credential invalid or expired (%d).", family.name, detail.status)}
		case billingRejected:
			return Result{Message: fmt.Sprintf("CodeBuddy %s quota error: %s", family.name, detail.message)}
		default:
			return Result{Message: fmt.Sprintf("CodeBuddy %s error: %s", family.name, err)}
		}
	}
	return codeBuddyResult(family, payload)
}

// codebuddyHeaders prefers the identification the provider's registry entry
// declares, falling back to the family's copy for a caller that handed no entry,
// the same order the endpoint resolution follows, so headers cannot travel to a
// host that was chosen from a different declaration.
func codebuddyHeaders(family codebuddyFamily, creds Credentials) map[string]string {
	if len(creds.UsageHeaders) > 0 {
		return creds.UsageHeaders
	}
	return family.headers
}

// refusedCredential and billingRejected are the two provider answers that stay soft:
// the sentence each maps to is what the reference's card renders for them.
type refusedCredential struct {
	status int
}

func (e refusedCredential) Error() string {
	return fmt.Sprintf("credential refused (%d)", e.status)
}

type billingRejected struct {
	message string
}

func (e billingRejected) Error() string {
	return e.message
}

func postCodeBuddy(ctx context.Context, endpoint string, headers map[string]string, token string) (codebuddyEnvelope, error) {
	// The bearer is merged before the request is built so the same map that goes on
	// the wire is the one presentedSecrets reads. A Go transport error carries the
	// URL and a provider's `msg` field can quote the request it refused, and both
	// end up as a sentence the poll worker caches.
	sent := make(map[string]string, len(headers)+1)
	for key, value := range headers {
		sent[key] = value
	}
	sent["Authorization"] = "Bearer " + token
	sent["Accept"] = "application/json"
	secrets := presentedSecrets(endpoint, sent)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString("{}"))
	if err != nil {
		return codebuddyEnvelope{}, errors.New(scrubText(err.Error(), secrets))
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range sent {
		request.Header.Set(key, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return codebuddyEnvelope{}, errors.New(scrubText(err.Error(), secrets))
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return codebuddyEnvelope{}, refusedCredential{status: response.StatusCode}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return codebuddyEnvelope{}, fmt.Errorf("quota API error (%d)", response.StatusCode)
	}

	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, codebuddyBodyLimit)).Decode(&envelope); err != nil {
		return codebuddyEnvelope{}, err
	}
	if envelope.Code != 0 {
		message := scrubText(envelope.Msg, secrets)
		if message == "" {
			message = "unknown"
		}
		return codebuddyEnvelope{}, billingRejected{message: message}
	}

	var parsed codebuddyEnvelope
	if err := json.Unmarshal(envelope.Data, &parsed); err != nil {
		return codebuddyEnvelope{}, err
	}
	return parsed, nil
}
