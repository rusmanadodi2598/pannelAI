// Package tokensaver implements the native token savers of SPEC-API-002.
//
// @file      internal/tokensaver/applier_test.go
// @for       Table-driven tests for Apply: bypass, fail-open, order, and restore.
// @uses      context, errors, net/http, testing.
// @reason    SPEC-API-002 §8 pins the request-path behavior: the optional
//
//	saver steps must run in order and never abort an otherwise valid
//	upstream body, so each failure mode is its own table case.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package tokensaver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
)

type applierCase struct {
	name                    string
	body                    []byte
	bypass                  bool
	settingsErr             error
	rtk, headroom, ponytail bool
	headroomWithURL         bool
	proxyStatus             int
	proxyMessages           string
	translatorRefuses       bool
	translatorRestoreErr    error
	wantSettingsCalls       int
	wantProxyCalls          int
	wantPrepareCalls        int
	wantRestoreCalls        int
	wantUnchanged           bool
	wantOrder               bool
	wantContains            []string
}

func TestApplierApply_Table(t *testing.T) {
	cases := []applierCase{
		{
			name: "bypass returns the body before reading settings",
			body: applierBody(), bypass: true,
			wantUnchanged: true,
		},
		{
			name:          "empty body returns before reading settings",
			body:          nil,
			wantUnchanged: true,
		},
		{
			name: "settings failure is fail-open and calls no transform",
			body: applierBody(), settingsErr: errSettingsUnavailable,
			rtk: true, headroom: true, ponytail: true, headroomWithURL: true,
			wantSettingsCalls: 1, wantProxyCalls: 0, wantPrepareCalls: 0, wantRestoreCalls: 0,
			wantUnchanged: true,
		},
		{
			name:              "all groups off calls nothing",
			body:              applierBody(),
			wantSettingsCalls: 1, wantUnchanged: true,
		},
		{
			name: "headroom enabled without a url is skipped",
			body: applierBody(), headroom: true,
			wantSettingsCalls: 1, wantProxyCalls: 0, wantPrepareCalls: 0, wantRestoreCalls: 0,
			wantUnchanged: true,
		},
		{
			name: "translator refusal skips the proxy",
			body: applierBody(), headroom: true, headroomWithURL: true,
			translatorRefuses: true,
			wantSettingsCalls: 1, wantProxyCalls: 0, wantPrepareCalls: 1, wantRestoreCalls: 0,
			wantUnchanged: true,
		},
		{
			name: "proxy failure still reaches ponytail",
			body: applierBody(), headroom: true, headroomWithURL: true, ponytail: true,
			proxyStatus:       http.StatusBadGateway,
			wantSettingsCalls: 1, wantProxyCalls: 1, wantPrepareCalls: 1, wantRestoreCalls: 0,
			wantContains: []string{ponytailPrompts[PonytailFull]},
		},
		{
			name: "restore failure keeps the latest body",
			body: applierBody(), headroom: true, headroomWithURL: true,
			proxyStatus: http.StatusOK, proxyMessages: `[{"role":"system","content":"compressed instructions"}]`,
			translatorRestoreErr: errRestoreFailed,
			wantSettingsCalls:    1, wantProxyCalls: 1, wantPrepareCalls: 1, wantRestoreCalls: 1,
			wantUnchanged: true,
		},
		{
			name: "the full order: rtk then headroom then ponytail",
			body: []byte(`{"model":"m","messages":[{"role":"tool","content":` + mustJSON(diffBlob()) + `},{"role":"user","content":"hello"}]}`),
			rtk:  true, headroom: true, headroomWithURL: true, ponytail: true,
			proxyStatus: http.StatusOK, proxyMessages: `[{"role":"user","content":"headroom-result"}]`,
			wantSettingsCalls: 1, wantProxyCalls: 1, wantPrepareCalls: 1, wantRestoreCalls: 1,
			wantOrder:    true,
			wantContains: []string{"headroom-result"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var proxy *applierHeadroomStub
			headroomURL := ""
			if tc.headroomWithURL {
				status := tc.proxyStatus
				if status == 0 {
					status = http.StatusOK
				}
				proxy = newApplierHeadroomStub(t, status, tc.proxyMessages)
				headroomURL = proxy.server.URL
			}
			settings := applierSettings(headroomURL, tc.rtk, tc.headroom, tc.ponytail)
			translator := &applierTranslatorStub{
				refuse: tc.translatorRefuses, restoreFailure: tc.translatorRestoreErr,
			}
			applier, reader := newApplierForCase(t, settings, tc.settingsErr, proxy, translator)
			got := applier.Apply(context.Background(), tc.body, WireOpenAI, "upstream", tc.bypass)
			if reader.calls != tc.wantSettingsCalls {
				t.Fatalf("settings calls = %d, want %d", reader.calls, tc.wantSettingsCalls)
			}
			if proxy != nil && proxy.calls != tc.wantProxyCalls {
				t.Fatalf("proxy calls = %d, want %d", proxy.calls, tc.wantProxyCalls)
			}
			if translator.prepareCalls != tc.wantPrepareCalls {
				t.Fatalf("translator prepare = %d, want %d", translator.prepareCalls, tc.wantPrepareCalls)
			}
			if translator.restoreCalls != tc.wantRestoreCalls {
				t.Fatalf("translator restore = %d, want %d", translator.restoreCalls, tc.wantRestoreCalls)
			}
			if tc.wantUnchanged && !bytes.Equal(got, tc.body) {
				t.Fatalf("body changed = %s, want %s", got, tc.body)
			}
			for _, want := range tc.wantContains {
				if want == ponytailPrompts[PonytailFull] && applierBodyCarriesPonytail(t, got) {
					continue
				}
				if !bytes.Contains(got, []byte(want)) {
					t.Fatalf("body = %s, want it to contain %q", got, want)
				}
			}
			if tc.wantOrder && !applierOrderLooksRight(t, translator, proxy, got) {
				t.Fatalf("pipeline order not as expected: prepared=%s request=%s got=%s", translator.prepared, proxy.request, got)
			}
		})
	}
}

// applierOrderLooksRight proves rtk compacted the body before headroom saw it,
// and headroom's result reached ponytail.
func applierOrderLooksRight(t *testing.T, translator *applierTranslatorStub, proxy *applierHeadroomStub, got []byte) bool {
	t.Helper()
	if bytes.Contains(translator.prepared, []byte("diff --git")) {
		return false
	}
	if bytes.Contains(proxy.request, []byte("diff --git")) {
		return false
	}
	if !bytes.Contains(proxy.request, []byte("@@")) {
		return false
	}
	if !bytes.Contains(got, []byte("headroom-result")) {
		return false
	}
	if !applierBodyCarriesPonytail(t, got) {
		return false
	}
	return true
}

var errRestoreFailed = errors.New("the compressed messages are not a message array")
