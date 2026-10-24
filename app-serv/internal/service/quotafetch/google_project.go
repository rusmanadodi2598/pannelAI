// The Cloud Code Assist project lookup on its own, so a caller that needs the
// project and not the quota can ask for it.
//
// @file      internal/service/quotafetch/google_project.go
// @for       Resolving which Cloud project an access token holds, for the login path that stores it.
// @uses      context, strings.
// @reason    Every Google family call is scoped to a project the token response never names, and the answer arrives polymorphic (a bare id on one account shape, an object on another). The quota fetchers already read it through this package's own bootstrap; the OAuth landing needs the same single reading rather than a second one that can drift from it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package quotafetch

import (
	"context"
	"strings"
)

// LoadProject asks the bootstrap which Cloud project the token's account holds.
// The endpoint is the one the entry declares, falling back to the family's
// built-in, and the user agent is the identification that call expects when the
// entry declares one.
//
// A lookup that is refused, unanswered, or silent about a project reports false
// rather than an error: the reference treats this as best-effort at login and
// looks the project up again where it is needed, and a login that stored no
// project is recoverable while a refused login is not.
func LoadProject(ctx context.Context, creds Credentials, token string, userAgent string) (string, bool) {
	if strings.TrimSpace(token) == "" {
		return "", false
	}
	account, ok := googleLoadAccount(ctx, creds, token, googleBootstrap{
		declared:  creds.Endpoints.LoadCodeAssistURL,
		builtIn:   googleLoadCodeAssistURL,
		userAgent: strings.TrimSpace(userAgent),
	})
	if !ok {
		return "", false
	}
	project := googleAccountProject(account.Project)
	return project, project != ""
}
