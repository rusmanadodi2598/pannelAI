// Package registry loads the provider registry and answers lookups over it.
//
// @file      internal/registry/provider_project.go
// @for       The entry's declaration of where an account's Cloud project is resolved.
// @uses      strings.
// @reason    A Google account's quota and inference are both scoped to a Cloud project the login response never names, and the reference declares the endpoint that does name it in three different places across its family. Answering "which endpoint, and with what client identification" from one accessor is what stops each reader from picking a different subset of the three.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-17
package registry

import "strings"

// ProjectBootstrap returns the endpoint that names this provider's Cloud
// project, and the user agent that call identifies itself with when the entry
// declares one. Both are empty for a provider with no such lookup.
//
// The three spellings the reference ships are read in one order: the oauth
// block, then the usage block's own load-assist url, then its project api url.
func (p Provider) ProjectBootstrap() (url string, userAgent string) {
	if p.OAuth != nil {
		if endpoint := strings.TrimSpace(p.OAuth.LoadCodeAssistURL); endpoint != "" {
			return endpoint, strings.TrimSpace(p.OAuth.LoadCodeAssistUA)
		}
		userAgent = strings.TrimSpace(p.OAuth.LoadCodeAssistUA)
	}
	if endpoint := strings.TrimSpace(p.Transport.Usage.LoadCodeAssistURL); endpoint != "" {
		return endpoint, userAgent
	}
	return strings.TrimSpace(p.Transport.Usage.LoadProjectAPIURL), userAgent
}
