// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_proxy_mask_test.go
// @for       The proxy password mask on a settings response.
// @uses      net/url, testing, internal/domain.
// @reason    A dashboard session must be able to read the URL it edits without receiving the password, so the response keeps the username and host and replaces only the secret.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-22
package schema

import (
	"net/url"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestSettingsResponseMasksAProxyPassword(t *testing.T) {
	settings := domain.DefaultSettings()
	settings.Network.OutboundProxyURL = "http://user:secret@proxy.internal:8080"

	response := SettingsResponseFrom(settings)
	parsed, err := url.Parse(response.Network.OutboundProxyURL)
	if err != nil {
		t.Fatalf("parsing masked URL: %v", err)
	}
	password, _ := parsed.User.Password()
	if password != domain.ProxyPasswordMask {
		t.Fatalf("password = %q, want the mask", password)
	}
	if parsed.Host != "proxy.internal:8080" {
		t.Fatalf("host = %q, want the stored host", parsed.Host)
	}
}

func TestSettingsResponseLeavesAURLWithoutPassword(t *testing.T) {
	settings := domain.DefaultSettings()
	settings.Network.OutboundProxyURL = "http://proxy.internal:8080"

	if got := SettingsResponseFrom(settings).Network.OutboundProxyURL; got != "http://proxy.internal:8080" {
		t.Fatalf("URL = %q, want it unchanged", got)
	}
}
