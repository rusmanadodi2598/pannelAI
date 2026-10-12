// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_service_account.go
// @for       The project and location a service_account endpoint addresses, and the rule that attaches them.
// @uses      internal/domain (error constructors), strings, time.
// @reason    A Google service-account credential is split in two: the JSON key is a secret and lives sealed in the keys table, while the project and location are configuration an operator types and the panel shows back. They are validated on the aggregate because a project stored on an endpoint that never makes the call it describes is a fact the router would only discover as a wrong URL.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-12
package domain

import (
	"regexp"
	"strings"
	"time"
)

// serviceAccountSegmentPattern is what a Google resource name may look like
// where it is spliced into a URL path: one plain segment, which is all a
// project id or a location ever is.
var serviceAccountSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// serviceAccountSegment reports whether one value may be spliced into a URL
// path as a segment. The two dot names are refused on their own: they pass the
// character class, and a path walk would rewrite the URL rather than address
// the resource.
func serviceAccountSegment(value string) bool {
	if value == "." || value == ".." {
		return false
	}
	return serviceAccountSegmentPattern.MatchString(value)
}

// ServiceAccountRef is the non-secret half of a service-account credential:
// the project and the location its calls address. The secret half is a row in
// the endpoint's keys table, sealed like every other credential, so nothing
// here is ciphertext.
type ServiceAccountRef struct {
	ProjectID string
	Location  string
}

// SetServiceAccount attaches the project and location a service_account
// endpoint addresses.
//
// It refuses an endpoint of any other auth type: a project id stored on an
// api_key endpoint would describe a call that endpoint never makes, and
// ignoring the field instead would let an operator believe it took effect. It
// also refuses an empty field, because an empty project or location builds a
// URL with a blank path segment that reaches the wrong resource rather than
// failing, and a value that is not one plain path segment, because a slash or
// a dot name would rewrite the path the call addresses.
func (e *UpstreamEndpoint) SetServiceAccount(ref ServiceAccountRef, now time.Time) error {
	if e.authType != UpstreamAuthServiceAccount {
		return NewValidationError("sa_project_id and sa_location belong to a service_account endpoint")
	}
	project := strings.TrimSpace(ref.ProjectID)
	location := strings.TrimSpace(ref.Location)
	if project == "" || location == "" {
		return NewValidationError("a service_account endpoint requires sa_project_id and sa_location")
	}
	if !serviceAccountSegment(project) || !serviceAccountSegment(location) {
		return NewValidationError("sa_project_id and sa_location must each be one path segment")
	}
	e.saProjectID = project
	e.saLocation = location
	e.updatedAt = now
	return nil
}

// ServiceAccount reports the project and location this endpoint addresses. It
// is the zero value for every other auth type, which is why a reader that
// needs it checks the auth type first.
func (e UpstreamEndpoint) ServiceAccount() ServiceAccountRef {
	return ServiceAccountRef{ProjectID: e.saProjectID, Location: e.saLocation}
}
