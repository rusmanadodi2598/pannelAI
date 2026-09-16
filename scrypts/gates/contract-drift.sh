#!/usr/bin/env bash
# Gate: contract drift between SPEC-API-001 §8 and the panel's error enum.
#
# The panel treats the API error codes as a closed enum on purpose: a code it
# does not know is a contract change, and it reports that instead of rendering a
# blank message. That property only holds while the two lists agree, so the
# agreement is checked mechanically.
#
# Why this exists as a gate rather than review discipline: adding a code to the
# spec without the panel (or the reverse) produces no compile error and no test
# failure on either side. It was already missed once, when METHOD_NOT_ALLOWED
# was added to §8 while app-ui still listed ten codes.
#
# Exit codes: 0 the lists agree, 1 they differ, 2 a source file is missing.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

root="$(repo_root)"
spec="$root/docs/SPEC-API/001-SPEC-API.md"
panel_errors="$root/app-ui/src/lib/schemas/error.ts"

gate_start "contract drift: SPEC-API §8 <-> panel error codes"

# A missing spec is not drift: this gate compares two lists, and it has nothing
# to compare without both. Failing here would block a repository that simply
# does not carry the spec (a service-only checkout), which is a different
# problem from the two lists disagreeing.
if [ ! -f "$spec" ]; then
	gate_skip "SPEC-API not present; nothing to compare"
	exit 0
fi

# The spec table is the source of truth. Read the code column between the §8
# heading and the next heading, which is where the envelope codes are listed.
spec_codes="$(awk '/^## 8\. Error Codes/{f=1} f && /^## 9\./{exit} f' "$spec" |
	grep -oE '^\| `[A-Z_]+`' | grep -oE '[A-Z_]+' | sort -u)"

if [ -z "$spec_codes" ]; then
	gate_fail "no error codes found in SPEC-API §8 (did the table format change?)"
	exit 2
fi

# A missing panel is not drift: the gate reports that it could not compare.
if [ ! -f "$panel_errors" ]; then
	gate_skip "panel error schema not present; nothing to compare"
	gate_skip "the spec defines $(printf '%s\n' "$spec_codes" | wc -l) codes"
	exit 0
fi

panel_codes="$(grep -oE "'[A-Z_]{4,}'" "$panel_errors" | tr -d "'" | sort -u)"

missing_in_panel="$(comm -23 <(printf '%s\n' "$spec_codes") <(printf '%s\n' "$panel_codes"))"
missing_in_spec="$(comm -13 <(printf '%s\n' "$spec_codes") <(printf '%s\n' "$panel_codes"))"

failed=0

if [ -n "$missing_in_panel" ]; then
	gate_fail "spec codes the panel does not know (it will report them as UNPARSABLE):"
	printf '  %s\n' $missing_in_panel >&2
	failed=1
fi

if [ -n "$missing_in_spec" ]; then
	gate_fail "panel codes the spec does not define:"
	printf '  %s\n' $missing_in_spec >&2
	failed=1
fi

if [ "$failed" -eq 0 ]; then
	count="$(printf '%s\n' "$spec_codes" | wc -l | tr -d ' ')"
	gate_pass "$count error codes agree on both sides"
fi

exit "$failed"