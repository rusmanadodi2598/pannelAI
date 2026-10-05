// Package router wires the HTTP surface of app-serv onto one mux.
//
// @file      internal/router/router_deps_guard.go
// @for       The boot-time assertion that every handler a route reaches is wired.
// @uses      fmt, strings.
// @reason    Registering a route against a nil handler cannot fail at boot, so a
//
//	half-wired deployment would answer 500 on that route until someone
//	clicks it. Asserting the whole set here turns that into a startup
//	failure that names what is missing (AGENTS.md §1.4).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-10-04
package router

import (
	"fmt"
	"strings"
)

// AssertWired returns an error naming every dependency the router registers a
// route against that is missing. The composition root calls it before New.
//
// The list is explicit rather than derived from Deps so that a handler added to
// Deps without a route here is reported by the router's own tests, and a route
// added without a check here is the one thing this function must never allow:
// keep the two in step.
func (d Deps) AssertWired() error {
	missing := make([]string, 0, 4)
	check := func(name string, wired bool) {
		if !wired {
			missing = append(missing, name)
		}
	}

	check("System", d.System != nil)
	check("Auth", d.Auth != nil)
	check("GatewayKey", d.GatewayKey != nil)
	check("Provider", d.Provider != nil)
	check("ProviderNode", d.ProviderNode != nil)
	check("ProviderValidate", d.ProviderValidate != nil)
	check("Endpoint", d.Endpoint != nil)
	check("EndpointKey", d.EndpointKey != nil)
	check("EndpointBulk", d.EndpointBulk != nil)
	check("OAuth", d.OAuth != nil)
	check("Model", d.Model != nil)
	check("Combo", d.Combo != nil)
	check("ComboTest", d.ComboTest != nil)
	check("ModelProbe", d.ModelProbe != nil)
	check("Proxy", d.Proxy != nil)
	check("MediaProvider", d.MediaProvider != nil)
	check("Media", d.Media != nil)
	check("VisionAdapter", d.VisionAdapter != nil)
	check("TokenSaver", d.TokenSaver != nil)
	check("Usage", d.Usage != nil)
	check("UsageLive", d.UsageLive != nil)
	check("Quota", d.Quota != nil)
	check("Log", d.Log != nil)
	check("Settings", d.Settings != nil)
	check("Skills", d.Skills != nil)
	check("OpenAPI", d.OpenAPI != nil)
	check("Changelog", d.Changelog != nil)
	check("Chat", d.Chat != nil)
	check("Embeddings", d.Embeddings != nil)
	check("TokenCount", d.TokenCount != nil)
	check("SystemOne", d.SystemOne != nil)
	check("RateLimiter", d.RateLimiter != nil)

	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("router: %d dependency fields are unwired: %s",
		len(missing), strings.Join(missing, ", "))
}
