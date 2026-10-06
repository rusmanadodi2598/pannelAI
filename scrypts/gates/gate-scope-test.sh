#!/usr/bin/env bash
# Gate: the push and commit gate router's own routing table.
#
# scrypts/lib/gate-scope.sh decides which half of the slow gates a change set runs.
# A wrong answer there is not a visible failure: it is a push that skipped the panel
# suite, or the panel suite running on a documentation typo. Both halves of that
# mistake are cheap to test and expensive to discover later, so the table is pinned
# here and runs in the hooks and in CI.
#
# Exit codes: 0 every case agrees, 1 at least one disagreed.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"
. "$(dirname "$0")/../lib/gate-scope.sh"

root="$(repo_root)"
failed=0

# check_route runs one case: paths, then the expected go and panel decisions.
check_route() {
	local label="$1" paths="$2" want_go="$3" want_fe="$4"
	(
		set -euo pipefail
		. "$(dirname "$0")/../lib/gate-scope.sh"
		if [ -z "$paths" ]; then
			gate_scope_from_files </dev/null
		else
			gate_scope_from_files <<<"$paths"
		fi
		[ "$GATE_SCOPE_GO" = "$want_go" ] || {
			printf 'go = %s, want %s\n' "$GATE_SCOPE_GO" "$want_go"
			exit 1
		}
		[ "$GATE_SCOPE_FE" = "$want_fe" ] || {
			printf 'panel = %s, want %s\n' "$GATE_SCOPE_FE" "$want_fe"
			exit 1
		}
	) || {
		gate_fail "gate scope: $label"
		failed=1
	}
}

gate_start "gate scope routing"

# A Go-only change cannot break the panel suite, and a panel-only change cannot
# break the Go build.
check_route "app-serv only runs go" \
	"app-serv/internal/service/oauth_flow.go
app-serv/cmd/app-serv/main.go" 1 0

check_route "app-ui only runs panel" \
	"app-ui/src/lib/schemas/openapi.ts
app-ui/tests/unit/docs.spec.ts" 0 1

# Both surfaces are read by each half: the panel renders what the spec and the
# generated artifact declare.
check_route "spec api runs both" "docs/SPEC-API/001-SPEC-API.md" 1 1
check_route "contract yaml runs both" "docs/CONTRACT/001-CONTRACT-API-V1.yaml" 1 1
check_route "wire schema runs both" "app-serv/internal/schema/chat.go" 1 1
check_route "generated openapi runs both" "app-serv/internal/handler/openapi.json" 1 1

# The gate machinery cannot be vouched for by the selector it is changing.
check_route "scrypts runs both" "scrypts/lib/gate-scope.sh" 1 1
check_route "workflows run both" ".github/workflows/gates.yml" 1 1

# Anything unrecognised runs everything rather than being assumed harmless.
check_route "unknown top level runs both" "DESIGN.md" 1 1
check_route "new directory runs both" "app-newsvc/main.go" 1 1

check_route "mixed runs both" \
	"app-serv/internal/domain/usage_test.go
app-ui/src/routes/+page.svelte" 1 1

# An empty change set is unknown, not clean: it must never read as a pass.
check_route "empty list runs both" "" 1 1

if [ "$failed" = 0 ]; then
	gate_pass "gate scope routing"
fi
exit "$failed"
