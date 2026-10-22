// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings_proxy.go
// @for       The write side of the proxy URL mask: keeping a stored password when a PATCH echoes the mask.
// @uses      net/url, strings, internal/domain.
// @reason    The settings response never returns the proxy password, so an unchanged form sends the mask back; replacing it with the stored secret is what lets host and username edits work without retyping a credential the panel cannot see.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package service

import (
	"net/url"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// mergeProxyPassword replaces the response's mask with the stored password, so a
// panel that saves an unchanged URL does not overwrite the credential it never saw.
func mergeProxyPassword(incoming, stored string) string {
	incomingURL, err := url.Parse(strings.TrimSpace(incoming))
	if err != nil || incomingURL.User == nil {
		return incoming
	}
	password, hasPassword := incomingURL.User.Password()
	if !hasPassword || password != domain.ProxyPasswordMask {
		return incoming
	}
	storedURL, err := url.Parse(strings.TrimSpace(stored))
	if err != nil || storedURL.User == nil {
		incomingURL.User = url.User(incomingURL.User.Username())
		return incomingURL.String()
	}
	storedPassword, hasStored := storedURL.User.Password()
	if !hasStored {
		incomingURL.User = url.User(incomingURL.User.Username())
		return incomingURL.String()
	}
	incomingURL.User = url.UserPassword(incomingURL.User.Username(), storedPassword)
	return incomingURL.String()
}
