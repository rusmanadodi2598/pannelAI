// Package handler serves the management HTTP API of app-serv.
//
// @file      internal/handler/quota_truncation_test.go
// @for       The truncated flag on the quota collection answer.
// @uses      encoding/json, net/http, net/http/httptest, strings, testing, time, internal/domain, internal/schema.
// @reason    Paging bounds the provider groups and a separate ceiling bounds the rows, so a page can be cut without looking cut, and the flag is the only thing that tells the operator the cards are not everything the groups hold.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-08
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestQuotaHandler_ListReportsACutPage pins that either of the page's two reads raises
// the flag. The rows the cards draw from and the accounts the provider answers are
// looked up by carry the same ceiling, and a page is incomplete when either was cut.
func TestQuotaHandler_ListReportsACutPage(t *testing.T) {
	cases := []struct {
		name          string
		windowsCut    bool
		accountsCut   bool
		wantTruncated bool
	}{
		{name: "nothing was cut", wantTruncated: false},
		{name: "the counted rows were cut", windowsCut: true, wantTruncated: true},
		{name: "the accounts were cut", accountsCut: true, wantTruncated: true},
		{name: "both reads were cut", windowsCut: true, accountsCut: true, wantTruncated: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, repo, _ := newQuotaFixture(t)
			// A page has to hold rows before anything can be cut from it, and the
			// accounts stub derives them from the windows, so one seed feeds both reads.
			window, err := domain.NewQuotaWindow("ep_a", "alpha", domain.QuotaWindowMonthly, nil, nil, time.Now())
			if err != nil {
				t.Fatalf("NewQuotaWindow() error = %v", err)
			}
			repo.windows = []domain.QuotaWindow{window}
			repo.windowsCut = tc.windowsCut
			repo.accountsCut = tc.accountsCut

			rec := httptest.NewRecorder()
			handler.List(rec, httptest.NewRequest(http.MethodGet, "/api/v1/quotas", strings.NewReader("")))
			if rec.Code != http.StatusOK {
				t.Fatalf("List status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
			}
			var list schema.QuotaWindowList
			if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
				t.Fatalf("decoding List body: %v", err)
			}
			if list.Truncated != tc.wantTruncated {
				t.Fatalf("truncated = %v, want %v", list.Truncated, tc.wantTruncated)
			}
			// Decoding cannot tell a false key from an absent one, so the contract's
			// "always present" is checked on the raw body.
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatalf("reading the raw body: %v", err)
			}
			if _, ok := raw["truncated"]; !ok {
				t.Fatalf("body carries no truncated key: %s", rec.Body.String())
			}
		})
	}
}
