// Package router registers the HTTP routes and their guards.
//
// @file      internal/router/router_oauth_device.go
// @for       The §7.4 device-flow routes: start a verification round, poll it.
//
// @uses      net/http.
// @reason    A device flow belongs to the same provider section as the code
//
//	flow, but it has no public callback to serve: the panel asks twice
//	under its own session. Registered apart because router.go is at its
//	§1.1 budget, which is also why the node routes have their own file.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-27
package router

import (
	"net/http"
)

// registerOAuthDeviceRoutes registers the two device-flow routes. Both are
// session-gated: a poll consumes a flow the same panel started, and the device
// code is not a bearer credential — the session is.
func registerOAuthDeviceRoutes(mux *routeRecorder, deps Deps, gateway func(http.Handler) http.Handler) {
	mux.Handle("POST "+APIVersion+"/providers/{provider_id}/oauth/device/start", gateway(http.HandlerFunc(deps.OAuth.DeviceStart)))
	mux.Handle("POST "+APIVersion+"/providers/{provider_id}/oauth/device/poll", gateway(http.HandlerFunc(deps.OAuth.DevicePoll)))
}
