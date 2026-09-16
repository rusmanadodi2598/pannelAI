#!/usr/bin/env bash
# Gate: the app-ui panel checks (SvelteKit/TypeScript, Bun toolchain).
#
# The declared toolchain is Bun (`package.json` engines, `app-ui/README.md`), so
# the npm scripts run through Bun. There is one environment trap worth knowing:
#
#   Bun ships two x86-64 builds. The default one requires AVX2; the `baseline`
#   build does not. On a CPU without AVX2 (Sandy/Ivy Bridge era) the default
#   binary dies with SIGILL even for `bun --version`.
#
# So this gate probes Bun by running it, not by finding it on PATH. A Bun that
# cannot execute is reported, and the gate falls back to the local npm/node
# toolchain so a machine whose panel code is fine still gets checked. The
# fallback is reported as a skip, never silently, because it leaves the
# production `bun --preload` start path unverified.
#
# If the probe fails, the fix is a different Bun binary, not a different machine:
#   curl -fsSL -o bun.zip \
#     https://github.com/oven-sh/bun/releases/latest/download/bun-linux-x64-baseline.zip
#   unzip bun.zip && install -m755 bun-linux-x64-baseline/bun ~/.local/bin/bun
#
# Checks, in increasing cost: format, type check, tests, build.
#
# Environment:
#   SKIP_PANEL_BUILD=1   skip the production build (the slowest step)
#
# Exit codes: 0 all checks passed, 1 a check failed.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

panel="$(panel_dir)"
if [ -z "$panel" ]; then
	gate_skip "no panel found (looked for app-ui/package.json)"
	exit 0
fi

# ------------------------------------------------------------- runner choice

runner_label=""

# usable_bun prints the path of a Bun that actually runs. It searches PATH plus
# ~/.local/bin, which is where a baseline build is commonly installed without
# disturbing a system-wide one.
usable_bun() {
	local candidate
	for candidate in bun "$HOME/.local/bin/bun"; do
		if command -v "$candidate" >/dev/null 2>&1 && "$candidate" --version >/dev/null 2>&1; then
			command -v "$candidate"
			return 0
		fi
	done
	return 1
}

if bun_path="$(usable_bun)"; then
	run_script() { (cd "$panel" && "$bun_path" run "$@"); }
	runner_label="bun ($("$bun_path" --version 2>/dev/null))"
else
	if have bun; then
		gate_skip "the bun on PATH cannot execute (likely the non-baseline build on a CPU without AVX2)"
		gate_skip "install the baseline build, see the header of this script"
	fi
	if ! have npm; then
		gate_fail "neither a working bun nor npm is available"
		exit 1
	fi
	if [ ! -d "$panel/node_modules" ]; then
		gate_fail "app-ui/node_modules is missing; run 'bun install' or 'npm install' first"
		exit 1
	fi
	gate_skip "falling back to npm/node; the bun --preload start path stays UNVERIFIED"
	run_script() { (cd "$panel" && npm run --silent "$@"); }
	runner_label="npm/node (fallback)"
fi

gate_start "app-ui checks via $runner_label"

failed=0

# ---------------------------------------------------------------- format

gate_start "prettier --check"
if run_script lint >/dev/null; then
	gate_pass "prettier"
else
	gate_fail "prettier: files need formatting (run 'bun run format')"
	failed=1
fi

# ------------------------------------------------------------- type lint

# ESLint covers what svelte-check does not: unused bindings, a floating
# promise, an explicit any, and a leftover eslint-disable. Prettier owns
# formatting, so the two do not overlap.
if [ -f "$panel/eslint.config.js" ]; then
	gate_start "eslint"
	if run_script lint:ts >/dev/null; then
		gate_pass "eslint"
	else
		gate_fail "eslint: fix the reported problems (run 'bun run lint:ts')"
		failed=1
	fi
else
	gate_skip "eslint config absent; skipped"
fi

# ------------------------------------------------------------ type check

gate_start "svelte-check"
if run_script check; then
	gate_pass "svelte-check"
else
	gate_fail "svelte-check"
	failed=1
fi

# ------------------------------------------------------------------ tests

gate_start "vitest run"
if run_script test; then
	gate_pass "vitest"
else
	gate_fail "vitest"
	failed=1
fi

# ------------------------------------------------------------------ build

if [ "${SKIP_PANEL_BUILD:-0}" = "1" ]; then
	gate_skip "production build skipped (SKIP_PANEL_BUILD=1)"
else
	gate_start "vite build"
	if run_script build; then
		gate_pass "vite build"
	else
		gate_fail "vite build"
		failed=1
	fi
fi

exit "$failed"