// Command app-serv wires the process-wide egress policy.
//
// @file      cmd/app-serv/egress_wiring.go
// @for       Builds the one egress guard and the guarded HTTP client every
//
//	upstream dial shares.
//
// @uses      internal/config, internal/dataplane, internal/netguard, fmt,
//
//	net/http, time.
//
// @reason    OWASP A01 makes the outbound policy one decision rather than one
//
//	per caller: the connectivity probe, the chat and media transports,
//	the OAuth client, and the proxy test all dial an address an operator
//	typed, so they must share one guard and one allowlist. Building it
//	here is what keeps a second allowlist from appearing — the proxy
//	wiring was the first caller and used to build its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
)

// egressKeepAlive mirrors the keep-alive the plain client configures, so a
// guarded dialer keeps the connection behaviour the pool was tuned for.
const egressKeepAlive = 30 * time.Second

// egress is the process-wide outbound policy.
type egress struct {
	// Guard validates a destination: before the dial wherever a caller can name
	// the host (the probe, the proxy test), and at connect time everywhere.
	Guard *netguard.Guard
	// Client is the one HTTP client every upstream call is made with.
	Client *http.Client
}

// buildEgress parses EGRESS_ALLOWED_TARGETS once and returns the guard plus the
// client built on its dialer. A malformed entry fails the boot rather than
// silently widening what the gateway may reach.
func buildEgress(cfg config.Config) (egress, error) {
	guard, err := netguard.NewGuard(cfg.EgressAllowedTargets)
	if err != nil {
		return egress{}, fmt.Errorf("egress wiring: %w", err)
	}
	return egress{
		Guard: guard,
		Client: dataplane.NewHTTPClient(dataplane.HTTPClientDeps{
			Dialer: guard.NewDialer(dataplane.ConnectTimeout, egressKeepAlive),
		}),
	}, nil
}
