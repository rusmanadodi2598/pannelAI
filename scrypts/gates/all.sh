#!/usr/bin/env bash
# Runs the full gate set locally, without committing or pushing.
#
# Useful before opening a pull request, and as the single command CI can call so
# the pipeline and the hooks enforce exactly the same rules.
#
# Usage:
#   scrypts/gates/all.sh              run everything
#   SKIP_PANEL_BUILD=1 scrypts/gates/all.sh   skip the panel production build
#
# Exit codes: 0 all gates passed, 1 at least one gate failed.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

root="$(repo_root)"
failed=0
declare -a summary=()

run_gate() {
	local label="$1"
	shift
	gate_start "$label"
	if "$@"; then
		gate_pass "$label"
		summary+=("PASS  $label")
	else
		gate_fail "$label"
		summary+=("FAIL  $label")
		failed=1
	fi
}

run_gate "go lint (vet, gofmt, staticcheck, golangci-lint)" bash "$root/scrypts/gates/go-lint.sh"
run_gate "go headers (AGENTS.md §1.2)" bash "$root/scrypts/gates/go-headers.sh"
run_gate "go test (race)" bash "$root/scrypts/gates/go-test.sh"
run_gate "panel checks (app-ui)" bash "$root/scrypts/gates/panel-check.sh"
run_gate "secrets (gitleaks)" bash "$root/scrypts/gates/secrets.sh" --all
run_gate "contract drift (SPEC-API <-> panel)" bash "$root/scrypts/gates/contract-drift.sh"

printf '\n%s==>%s summary\n' "$(_colour "$C_BOLD")" "$(_colour "$C_RESET")"
printf '%s\n' "${summary[@]}"

if [ "$failed" -ne 0 ]; then
	printf '\n%sone or more gates failed%s\n' "$(_colour "$C_RED")" "$(_colour "$C_RESET")" >&2
	exit 1
fi

printf '\n%sall gates passed%s\n' "$(_colour "$C_GREEN")" "$(_colour "$C_RESET")"