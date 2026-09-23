#!/usr/bin/env bash
# Gate: secret scanning with gitleaks.
#
# AGENTS.md §1.4 lists gitleaks as a CI requirement. Three modes matter, and the
# subcommand choice is load-bearing rather than cosmetic:
#
#   --staged   `gitleaks git --staged` scans the staged patch. This is the
#              pre-commit mode: a secret must not enter history, because
#              deleting it later does not remove the commit.
#   --all      `gitleaks git` scans commits plus `gitleaks dir` scans the files a
#              push would carry: tracked files, and untracked files that are not
#              ignored. Both are needed: `detect` only walks commits, so it
#              reports "no leaks" for a tree that contains a secret which has
#              not been committed yet. That was measured on 8.30.1, where
#              `detect` returned 0 on a staged credential while `git --staged`
#              returned 1. The working-tree scan deliberately excludes ignored
#              files; the phase comment below records why.
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
	# uncommitted file, which the commit walk never sees. The scan covers exactly
	# the files a push could carry, which is what `git ls-files --cached --others
	# --exclude-standard` lists.
	#
	# Ignored files are out of scope on purpose. They cannot enter a commit
	# without `git add -f`, and that path is already covered by the --staged scan
	# at commit time. Scanning them anyway made this gate permanently red on a
	# working copy that was doing its job: `app-serv/.env` holds the local dev
	# SESSION_SECRET and ENCRYPTION_KEY, and `graphify-out/cache/stat-index.json`
	# embeds every file's content hash under its "hashes" key, which the
	# generic-api-key heuristic reads as 121 credentials. A gate that can never go
	# green is a gate people learn to bypass, which is the opposite of the point.
	#
	# gitleaks `dir` accepts one path and has no exclusion flag (measured on
	# 8.30.1: passing two paths silently falls back to scanning the whole
	# directory), so the list is materialised into a temporary tree and that tree
	# is scanned. The list is NUL-separated, so a path with a space survives, and
	# `--no-recursion` keeps a directory entry, if one ever appears, from pulling
	# ignored files in behind it. A list that cannot be materialised fails the
	# gate: blocking is the safe direction.
	gate_start "gitleaks (working tree)"
	scratch="$(mktemp -d)"
	trap 'rm -rf "$scratch"' EXIT
	if ! (cd "$root" && git ls-files -z --cached --others --exclude-standard |
		tar --null --no-recursion -T - -cf - | tar -xf - -C "$scratch"); then
		gate_fail "could not materialise the pushable file list into $scratch"
		failed=1
	elif (cd "$scratch" && "$gl" dir . "${gl_args[@]}"); then
		gate_pass "no secrets in the files a push would carry"
	else
		gate_fail "secrets detected in the files a push would carry"
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