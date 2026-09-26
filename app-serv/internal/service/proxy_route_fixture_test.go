// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_route_fixture_test.go
// @for       The proxy route tests' in-memory collaborators and row builders,
//
//	apart from the assertions so each file's reason stays readable.
//
// @uses      context, testing, time, internal/domain.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D1-D6: the plan is the engine's
//
//	one decision, and both of its test files drive the same fakes and
//	row builders. Keeping them here means one definition serves every
//	assertion, the way the egress guard's source-reading helpers sit
//	apart from the assertions built on them.
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

type fakeProxyLister struct {
	rows []domain.Proxy
	err  error
}

func (f *fakeProxyLister) List(context.Context) ([]domain.Proxy, error) {
	return f.rows, f.err
}

type fakeRouteStore struct {
	nextOrder []string
	nextErr   error
	parked    map[string]bool
	parkedIDs []string
}

func (f *fakeRouteStore) Next(_ context.Context, _ string, ids []string) ([]string, error) {
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	if f.nextOrder != nil {
		return f.nextOrder, nil
	}
	return ids, nil
}

func (f *fakeRouteStore) Park(_ context.Context, proxyID string, _ time.Duration) error {
	f.parkedIDs = append(f.parkedIDs, proxyID)
	return nil
}

func (f *fakeRouteStore) Parked(_ context.Context, proxyID string) (bool, error) {
	return f.parked[proxyID], nil
}

type fakePlanSettings struct {
	document domain.Settings
	err      error
}

func (f *fakePlanSettings) Settings(context.Context) (domain.Settings, error) {
	return f.document, f.err
}

type fakePlanOpener struct {
	err error
}

func (f *fakePlanOpener) Open(sealed string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "opened-" + sealed, nil
}

func routeFixture(rows []domain.Proxy, document domain.Settings) *ProxyRouteService {
	return NewProxyRouteService(ProxyRouteDeps{
		Proxies:  &fakeProxyLister{rows: rows},
		Routes:   &fakeRouteStore{parked: map[string]bool{}},
		Settings: &fakePlanSettings{document: document},
		Opener:   &fakePlanOpener{},
		Clock:    func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) },
	})
}

func poolRow(t *testing.T, id string, age time.Duration, enabled bool, state string, username, sealed string) domain.Proxy {
	t.Helper()
	proxy, err := domain.NewProxy(id, "pool "+id, domain.ProxyProtocolHTTP, id+".example.com", 8080, username, sealed, time.Now().Add(-age))
	if err != nil {
		t.Fatalf("NewProxy(%s) error = %v", id, err)
	}
	proxy.SetEnabled(enabled, time.Now())
	if state != "" {
		proxy.RecordTest(state, 42, "", time.Now())
	}
	return proxy
}

func planDocument(strategy, staticURL string) domain.Settings {
	return domain.Settings{Network: domain.NetworkSettings{
		OutboundProxyEnabled:  true,
		OutboundProxyURL:      staticURL,
		OutboundProxyStrategy: strategy,
	}}
}

func attemptIDs(plan []domain.ProxyRouteAttempt) []string {
	ids := make([]string, len(plan))
	for i, attempt := range plan {
		ids[i] = attempt.ID
	}
	return ids
}
