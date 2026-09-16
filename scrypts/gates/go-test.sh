#!/usr/bin/env bash
# Gate: Go test suite for every service module under app-*/.
#
# Runs the default suite with the race detector, which AGENTS.md §2.1 makes a
# required CI gate for any package touching goroutines or shared state.
#
# The PostgreSQL integration suite runs only when PANNELAI_TEST_POSTGRES_DSN is
# set: those tests carry an `integration` build tag precisely so a machine
# without a database runs the hermetic suite and reports accurately, rather than
# skipping tests at runtime (§2.1 forbids t.Skip).
#
# Exit codes: 0 all suites passed, 1 a suite failed.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

root="$(repo_root)"
dirs="$(go_service_dirs)"

if [ -z "$dirs" ]; then
	gate_skip "no Go module found"
	exit 0
fi

require_tool go || exit 1

failed=0

while IFS= read -r dir; do
	[ -n "$dir" ] || continue
	rel="${dir#"$root"/}"

	gate_start "go test -race   $rel"
	if (cd "$dir" && go test -race -count=1 ./...); then
		gate_pass "go test -race $rel"
	else
		gate_fail "go test -race $rel"
		failed=1
	fi

	if [ -n "${PANNELAI_TEST_POSTGRES_DSN:-}" ]; then
		gate_start "integration     $rel (tagged)"
		if (cd "$dir" && go test -race -tags=integration -count=1 ./...); then
			gate_pass "integration suite $rel"
		else
			gate_fail "integration suite $rel"
			failed=1
		fi
	else
		gate_skip "integration suite skipped: PANNELAI_TEST_POSTGRES_DSN is not set"
	fi
done <<< "$dirs"

exit "$failed"