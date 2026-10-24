#!/usr/bin/env bash
# Gate: known vulnerabilities in the Go services under app-*/.
#
# govulncheck answers a question vet and staticcheck cannot: whether code the
# service actually reaches carries a published vulnerability. AGENTS.md §1.4
# makes the linter suite a CI requirement, and a reachable CVE is the one
# finding no compiler or style linter can report.
#
# The tool is installed pinned (CI and scrypts/README), not with @latest: a gate
# whose verdict changes without a commit here says nothing about yesterday's
# green run.
#
# Exit codes: 0 no vulnerability found in reachable code, 1 a finding or a
# missing tool.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

root="$(repo_root)"
dirs="$(go_service_dirs)"

if [ -z "$dirs" ]; then
	gate_skip "no Go module found (looked for app-*/*/go.mod)"
	exit 0
fi

require_tool go || exit 1

if ! govuln="$(tool_path govulncheck)"; then
	gate_fail "govulncheck is not installed"
	gate_fail "install: go install golang.org/x/vuln/cmd/govulncheck@v1.9.0"
	exit 1
fi

failed=0
while IFS= read -r dir; do
	[ -n "$dir" ] || continue
	rel="${dir#"$root"/}"

	gate_start "govulncheck     $rel"
	if (cd "$dir" && "$govuln" ./...); then
		gate_pass "govulncheck $rel"
	else
		gate_fail "govulncheck $rel"
		failed=1
	fi
done <<< "$dirs"

exit "$failed"
