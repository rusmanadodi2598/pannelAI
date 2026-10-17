// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_project.go
// @for       The seam that turns a freshly issued token into the Cloud project the account holds.
// @uses      context, internal/registry, strings.
// @reason    A Google account's calls are scoped to a project the authorization response never names, and only some providers declare the endpoint that does. The flow therefore asks through one seam it does not implement: the lookup belongs to the package that already speaks that protocol, and the flow keeps to deciding whether an entry declares one.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ProjectResolver answers which Cloud project an access token holds. It is
// optional: a deployment that installs none connects accounts without a
// project rather than refusing to connect them.
type ProjectResolver interface {
	ResolveProject(ctx context.Context, providerID, accessToken string) (string, bool)
}

// resolveProject asks the seam on a login's behalf, and only for an entry that
// declares where to ask.
//
// No answer is a normal outcome, not a failure: the credential the operator
// just granted is what lets the project be found later, so a login whose
// bootstrap is silent, refused, or unreachable still lands the account.
func (s *OAuthFlowService) resolveProject(ctx context.Context, entry registry.Provider, accessToken string) string {
	if s.projects == nil || strings.TrimSpace(accessToken) == "" {
		return ""
	}
	if endpoint, _ := entry.ProjectBootstrap(); endpoint == "" {
		return ""
	}
	project, ok := s.projects.ResolveProject(ctx, entry.ID, accessToken)
	if !ok {
		return ""
	}
	return strings.TrimSpace(project)
}
