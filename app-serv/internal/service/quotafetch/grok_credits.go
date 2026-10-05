// Grok CLI's weekly credit pool read: the gRPC-web call that the frame decoder parses.
//
// @file      internal/service/quotafetch/grok_credits.go
// @for       Asks Grok's gRPC-web credits endpoint and turns its frame into one percentage window.
// @uses      internal/service/quotafetch, net/http
// @reason    The weekly SuperGrok pool is published only over this binary surface, the REST
//
//	billing answer carries no weekly figure, so the second call and its
//	transport constants live apart from the decoder that reads the bytes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const (
	grokCreditsDisplay = "Grok CLI credits"

	// grokEmptyRequest is a zero-length gRPC-web frame. The endpoint answers an
	// absent body with grpc-status 13, so the call sends one honest empty frame.
	grokEmptyRequest = "\x00\x00\x00\x00\x00"

	// grokGrpcCreditsURL is this pool's host and method. It is not a second face of
	// the billing endpoint, so no registry key declares it; the test seam
	// (`creds.Endpoint`) is what redirects it.
	grokGrpcCreditsURL = "https://grok.com/grok_api_v2.GrokBuildBilling/GetGrokCreditsConfig"
)

// grokCreditsQuota reads the live weekly pool as one percentage window. Any failure is
// soft: a frame this package cannot read is a sentence on the card, not a broken page.
func grokCreditsQuota(ctx context.Context, creds Credentials) (Quota, Result, bool) {
	endpoint := endpointFor(grokGrpcCreditsURL, creds.Endpoint)
	headers := map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(creds.AccessToken),
		"Accept":        "application/grpc-web+proto",
		"Content-Type":  "application/grpc-web+proto",
		"X-Grpc-Web":    "1",
	}
	response, err := requestUsage(ctx, http.MethodPost, endpoint, headers, grokEmptyRequest)
	if err != nil {
		return Quota{}, Result{Message: fmt.Sprintf("Grok CLI credits error: %s", err)}, false
	}
	if failure, refused := response.softFailure(grokCreditsDisplay); refused {
		return Quota{}, failure, false
	}
	decoded, err := grokFrameDecode(response.body)
	if err != nil {
		return Quota{}, Result{Message: fmt.Sprintf("Grok CLI weekly quota frame could not be decoded: %s", err)}, false
	}
	row := Quota{Label: grokLabelWeekly, Used: roundGrokPercent(decoded.PercentUsed), Total: 100, Unit: "%", Recurring: true}
	if decoded.HasReset {
		row.ResetAt = decoded.ResetAt
	}
	return row, Result{}, true
}
