// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_route_test.go
// @for       The plan table: what the pool offers a request, in what order,
//
//	and what is refused or degraded.
//
// @uses      context, testing, time, internal/domain.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D1-D6: the plan is the engine's
//
//	one decision, so its table pins every rule the request path leans
//	on: usability, stable order, parking, the static last resort, the
//	empty-URL bypass, the malformed-URL refusal, and the exemption.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestProxyRouteService_Plan(t *testing.T) {
	oldest := poolRow(t, "prx_a", 3*time.Hour, true, "ok", "", "")
	middle := poolRow(t, "prx_b", 2*time.Hour, true, "", "", "")
	newest := poolRow(t, "prx_c", 1*time.Hour, true, "ok", "operator", "sealed-c")

	cases := []struct {
		name     string
		rows     []domain.Proxy
		document domain.Settings
		parked   map[string]bool
		host     string
		wantIDs  []string
		wantErr  bool
	}{
		{
			name:     "proxying off plans nothing",
			rows:     []domain.Proxy{oldest},
			document: domain.Settings{},
			wantIDs:  []string{},
		},
		{
			name:     "fallback walks insertion order with the static url as the last resort",
			rows:     []domain.Proxy{newest, oldest, middle},
			document: planDocument("", "http://static.example.com:3128"),
			wantIDs:  []string{"prx_a", "prx_b", "prx_c", ""},
		},
		{
			name:     "a failed probe sits the row out until it is re-tested",
			rows:     []domain.Proxy{poolRow(t, "prx_a", 3*time.Hour, true, "fail", "", ""), middle},
			document: planDocument("", ""),
			wantIDs:  []string{"prx_b"},
		},
		{
			name:     "a parked row is skipped while an unparked one remains",
			rows:     []domain.Proxy{oldest, middle},
			document: planDocument("", ""),
			parked:   map[string]bool{"prx_a": true},
			wantIDs:  []string{"prx_b"},
		},
		{
			name:     "all parked degrades to the usable list, because parking is a hint",
			rows:     []domain.Proxy{oldest, middle},
			document: planDocument("", ""),
			parked:   map[string]bool{"prx_a": true, "prx_b": true},
			wantIDs:  []string{"prx_a", "prx_b"},
		},
		{
			name:     "a blank url plans no static tail, preserving the empty-url bypass",
			rows:     []domain.Proxy{oldest},
			document: planDocument("", "   "),
			wantIDs:  []string{"prx_a"},
		},
		{
			// The static route refuses a URL it cannot dial rather than dialing direct, and the
			// engine must not weaken that by quietly dropping the tail: an operator's stored
			// malformed value is a broken configuration, not a licence to bypass it.
			name:     "a malformed stored url is refused, not dropped",
			rows:     []domain.Proxy{oldest},
			document: planDocument("", "not-a-url"),
			wantErr:  true,
		},
		{
			name:     "no usable row plans nothing, so the shared client serves the static url",
			rows:     []domain.Proxy{poolRow(t, "prx_a", time.Hour, false, "ok", "", "")},
			document: planDocument("", "http://static.example.com:3128"),
			wantIDs:  []string{},
		},
		{
			name: "an exempt destination plans nothing",
			rows: []domain.Proxy{oldest},
			document: func() domain.Settings {
				document := planDocument("", "")
				document.Network.OutboundNoProxy = "destination.example"
				return document
			}(),
			host:    "api.destination.example",
			wantIDs: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := routeFixture(tc.rows, tc.document)
			svc.routes.(*fakeRouteStore).parked = tc.parked

			host := tc.host
			if host == "" {
				host = "api.destination.example"
			}
			plan, err := svc.Plan(context.Background(), host)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Plan() = nil error, want the malformed stored url refused")
				}
				return
			}
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			got := attemptIDs(plan)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("Plan() ids = %v, want %v", got, tc.wantIDs)
			}
			for i := range tc.wantIDs {
				if got[i] != tc.wantIDs[i] {
					t.Fatalf("Plan() ids = %v, want %v", got, tc.wantIDs)
				}
			}
		})
	}
}
