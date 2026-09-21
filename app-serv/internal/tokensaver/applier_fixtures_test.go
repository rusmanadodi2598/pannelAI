// Package tokensaver implements the native token savers of SPEC-API-002.
//
// @file      internal/tokensaver/applier_fixtures_test.go
// @for       Shared fixtures for direct tests of the saver orchestrator.
// @uses      internal/domain, context, encoding/json, errors, io, net/http,
//
//	net/http/httptest, strings, testing.
//
// @reason    The orchestrator needs concrete settings, translator, and proxy
// fixtures so its fail-open behavior can be tested without production wiring.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package tokensaver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

type applierSettingsStub struct {
	settings domain.Settings
	err      error
	calls    int
}

func (s *applierSettingsStub) Settings(context.Context) (domain.Settings, error) {
	s.calls++
	return s.settings, s.err
}

type applierTranslatorStub struct {
	delegate       rawHeadroomTranslator
	prepared       []byte
	restored       []byte
	prepareCalls   int
	restoreCalls   int
	refuse         bool
	restoreFailure error
}

func (t *applierTranslatorStub) Prepare(body []byte, wire, model string) (HeadroomRequest, bool, error) {
	t.prepareCalls++
	t.prepared = append([]byte(nil), body...)
	if t.refuse {
		return HeadroomRequest{}, false, nil
	}
	return t.delegate.Prepare(body, wire, model)
}

func (t *applierTranslatorStub) Restore(body []byte, wire, model string, messages json.RawMessage) ([]byte, error) {
	t.restoreCalls++
	t.restored = append([]byte(nil), body...)
	if t.restoreFailure != nil {
		return body, t.restoreFailure
	}
	return t.delegate.Restore(body, wire, model, messages)
}

type applierHeadroomStub struct {
	server   *httptest.Server
	calls    int
	request  []byte
	status   int
	messages string
}

func newApplierHeadroomStub(t *testing.T, status int, messages string) *applierHeadroomStub {
	t.Helper()
	stub := &applierHeadroomStub{status: status, messages: messages}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls++
		stub.request, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		if stub.messages != "" {
			_, _ = io.WriteString(w, `{"messages":`+stub.messages+`}`)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func applierBody() []byte {
	return []byte(`{"model":"m","messages":[{"role":"user","content":"hello"}]}`)
}

func applierSettings(headroomURL string, rtk, headroom, ponytail bool) domain.Settings {
	return domain.Settings{TokenSaver: domain.TokenSaverSettings{
		RTK:      domain.TokenSaverRTK{Enabled: rtk, Filters: []string{"git-diff"}},
		Headroom: domain.TokenSaverHeadroom{Enabled: headroom, URL: headroomURL},
		Ponytail: domain.TokenSaverToggle{Enabled: ponytail, Level: PonytailFull},
	}}
}

func newApplierForCase(
	t *testing.T,
	settings domain.Settings,
	settingsErr error,
	proxy *applierHeadroomStub,
	translator *applierTranslatorStub,
) (*Applier, *applierSettingsStub) {
	t.Helper()
	reader := &applierSettingsStub{settings: settings, err: settingsErr}
	var headroom *HeadroomClient
	if proxy != nil {
		headroom = NewHeadroomClient(proxy.server.Client())
	}
	applier, err := NewApplier(ApplierDeps{
		Settings: reader, Headroom: headroom, Translator: translator,
	})
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}
	return applier, reader
}

func mustJSON(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func applierBodyCarriesPonytail(t *testing.T, body []byte) bool {
	t.Helper()
	object, ok := decodeObject(body)
	if !ok {
		return false
	}
	messages, ok := decodeArray(object["messages"])
	if !ok {
		return false
	}
	for _, raw := range messages {
		message, ok := decodeObject(raw)
		if !ok {
			continue
		}
		role, roleOK := message.memberString("role")
		content, contentOK := message.memberString("content")
		if role == "system" && roleOK && contentOK && strings.Contains(content, ponytailPrompts[PonytailFull]) {
			return true
		}
	}
	return false
}

var errSettingsUnavailable = errors.New("settings unavailable")
