// Command app-serv is the composition root: it loads configuration, builds the
//
//	graph, and serves it.
//
// @file      cmd/app-serv/version_test.go
// @for       Test that the version payload reports the loaded registry revision.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §7.1 exposes registry_revision so an operator can tell
//
//	which registry a running gateway loaded. The field shipped as the
//	literal "none" for as long as nothing asserted otherwise, which is a
//	payload that answers the question wrongly rather than not at all.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package main

import "testing"

// TestBuildInfo_ReportsTheRegistryRevision pins the field to its argument, so a
// placeholder cannot come back unnoticed.
func TestBuildInfo_ReportsTheRegistryRevision(t *testing.T) {
	const revision = "9router@db4499d (2026-06-19)"

	info := buildInfo(revision)
	if info.RegistryRevision != revision {
		t.Fatalf("registry_revision = %q, want %q", info.RegistryRevision, revision)
	}
	if info.Version == "" || info.GoVersion == "" {
		t.Fatalf("version payload = %+v, want the identifying fields filled", info)
	}
}
