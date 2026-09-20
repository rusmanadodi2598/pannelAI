// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/changelog_test.go
// @for       The §7.18 release notes' ordering and completeness.
// @uses      net/http/httptest, encoding/json, testing.
// @reason    The panel renders the entries as served, so the tests pin the
//
//	shape a consumer relies on: newest first, every field populated,
//	and the version line anchored at the skeleton tag the repository
//	actually carries.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChangelogEntries_NewestFirst(t *testing.T) {
	if changelogEntries[0].Version != "v0.4.0" {
		t.Fatalf("newest entry is %q, want the current phase", changelogEntries[0].Version)
	}
	for i := 1; i < len(changelogEntries); i++ {
		if changelogEntries[i-1].Date < changelogEntries[i].Date {
			t.Fatalf("entry %d (%s %s) is newer than entry %d (%s %s)",
				i-1, changelogEntries[i-1].Version, changelogEntries[i-1].Date,
				i, changelogEntries[i].Version, changelogEntries[i].Date)
		}
	}
	last := changelogEntries[len(changelogEntries)-1]
	if last.Version != "v0.0.1" || last.Date != "2026-09-16" {
		t.Fatalf("oldest entry is %s (%s), want the repository's skeleton tag v0.0.1", last.Version, last.Date)
	}
}

func TestChangelogList_Shape(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/changelog", nil)
	NewChangelogHandler().List(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body changelogResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	if len(body.Data) != len(changelogEntries) {
		t.Fatalf("served %d entries, table carries %d", len(body.Data), len(changelogEntries))
	}
	for _, entry := range body.Data {
		if entry.Version == "" || entry.Date == "" || entry.Title == "" || entry.Notes == "" {
			t.Fatalf("entry %s has an empty field: %+v", entry.Version, entry)
		}
	}
}
