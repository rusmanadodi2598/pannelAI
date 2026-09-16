#!/usr/bin/env bash
# Gate: secret scanning with gitleaks.
#
# AGENTS.md §1.4 lists gitleaks as a CI requirement. Three modes matter, and the
# subcommand choice is load-bearing rather than cosmetic:
#
#   --staged   `gitleaks git --staged` scans the staged patch. This is the
#              pre-commit mode: a secret must not enter history, because
#              deleting it later does not remove the commit.
#   --all      `gitleaks git` scans commits plus `gitleaks dir` scans the
#              working tree. Both are needed: `detect` only walks commits, so it
#              reports "no leaks" for a tree that contains a secret which has
#              not been committed yet. That was measured on 8.30.1, where
#              `detect` returned 0 on a staged credential while `git --staged`
#              returned 1.
#
# Both modes read .gitleaks.toml, so the hook and the CI run cannot drift into
# enforcing different rules.
#
# Usage:  scrypts/gates/secrets.sh [--staged|--all]
#
# Exit codes: 0 no leaks found, 1 leaks found, 2 gitleaks unavailable.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

root="$(repo_root)"
mode="${1:---all}"
config="$root/.gitleaks.toml"

if ! gl="$(tool_path gitleaks)"; then
	gate_fail "gitleaks is not installed"
	gate_fail "install: https://github.com/gitleaks/gitleaks#installing"
	exit 2
fi

# Only pass --config when the file exists; gitleaks errors on a missing path
# rather than falling back to its defaults.
gl_args=(--redact --no-banner)
if [ -f "$config" ]; then
	gl_args+=(--config "$config")
fi

case "$mode" in
--staged)
	gate_start "gitleaks (staged changes)"
	if (cd "$root" && "$gl" git --staged "${gl_args[@]}"); then
		gate_pass "no secrets in staged changes"
	else
		gate_fail "secrets detected in staged changes; the commit was not made"
		exit 1
	fi
	;;
--all)
	failed=0

	gate_start "gitleaks (commits)"
	if (cd "$root" && "$gl" git "${gl_args[@]}"); then
		gate_pass "no secrets in commits"
	else
		gate_fail "secrets detected in commits"
		failed=1
	fi

	# The working tree is scanned separately because a secret can exist in an
	# uncommitted or ignored-but-present file, which the commit walk never sees.
	gate_start "gitleaks (working tree)"
	if (cd "$root" && "$gl" dir . "${gl_args[@]}"); then
		gate_pass "no secrets in the working tree"
	else
		gate_fail "secrets detected in the working tree"
		failed=1
	fi

	exit "$failed"
	;;
*)
	gate_fail "unknown mode '$mode' (expected --staged or --all)"
	exit 2
	;;
esac

exit 0