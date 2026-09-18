#!/usr/bin/env bash
# Gate: the AGENTS.md §1.2 header contract on every hand-authored Go file.
#
# §1.2 requires a tagged header on every new or modified source file, and the
# compliance checklist treats a missing tag as a CI failure. The rule is easy to
# satisfy once and then drift, so it is asserted mechanically:
#
#   - all eight tags present, each exactly once, so a duplicated header (which
#     happens when a file is split and the header is copied) is caught
#   - @file matches the path relative to the module root, so a header that was
#     not updated after a rename is caught
#   - @author matches the one value §1.2 fixes
#   - @layer is one of the enumerated values
#   - @stability is one of the enumerated values
#
# Generated code is exempt per §1.1/§1.2.
#
# Exit codes: 0 all headers valid, 1 at least one violation.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

root="$(repo_root)"

readonly REQUIRED_TAGS=(@file @for @uses @reason @author @layer @stability @since)
readonly AUTHOR_NAME="Dodi Rusmana <rusmanadodi@kentangtech.com>"
readonly AUTHOR="@author    $AUTHOR_NAME"
readonly LAYERS="schema domain repository service handler router worker job util config"
readonly STABILITIES="experimental stable deprecated"

failed=0
checked=0

# is_member reports whether the first word appears in the space-separated list.
# It is a case match rather than `printf | tr | grep -qx` because under
# `set -o pipefail` the producer can be killed by SIGPIPE when grep exits on its
# first match, which turns a match into a reported violation.
is_member() {
	case " $2 " in
	*" $1 "*) return 0 ;;
	*) return 1 ;;
	esac
}

# is_generated reports whether a file is tool output, which §1.1 and §1.2 exempt.
is_generated() {
	case "$1" in
	*_gen.go | *.pb.go) return 0 ;;
	*) return 1 ;;
	esac
}

# module_root_for prints the module root containing the given file.
module_root_for() {
	local file="$1" dir
	dir="$(dirname "$file")"
	while [ "$dir" != "/" ] && [ "$dir" != "$root" ]; do
		if [ -f "$dir/go.mod" ]; then
			printf '%s\n' "$dir"
			return 0
		fi
		dir="$(dirname "$dir")"
	done
	printf '%s\n' "$root"
}

# check_file reports every violation for one file.
check_file() {
	local file="$1"
	local rel="${file#"$root"/}"

	if is_generated "$rel"; then
		return 0
	fi

	# Only hand-authored Go under a service module is in scope, matching the
	# `app-*/internal/**` and `app-*/cmd/**` scope in §1.1.
	case "$rel" in
	app-*/internal/* | app-*/cmd/*) ;;
	*) return 0 ;;
	esac

	# The header is the comment block above `package`, so tag-looking text in a
	# doc comment further down cannot satisfy the rule.
	local header
	header="$(awk '/^package /{exit} {print}' "$file")"

	if [ -z "$header" ]; then
		gate_fail "$rel: no comment block above the package clause"
		failed=1
		return 0
	fi

	checked=$((checked + 1))

	# One awk pass extracts every tag and how often it appears. It replaces
	# `grep -m1 … | awk …` pipelines, which under `set -o pipefail` report 141
	# whenever grep closes the pipe before printf has finished writing — a
	# flaky failure that printed no detail and blocked commits at random.
	local tags tag count
	tags="$(printf '%s\n' "$header" | awk '
		match($0, /^\/\/[ \t]+@[a-z-]+[ \t]+/) {
			line = substr($0, RSTART + RLENGTH)
			tag = substr($0, RSTART, RLENGTH)
			sub(/^\/\/[ \t]+/, "", tag)
			gsub(/[ \t]+$/, "", tag)
			sub(/[ \t]+$/, "", line)
			value[tag] = line
			count[tag]++
		}
		END {
			for (t in count) printf "%s\t%d\t%s\n", t, count[t], value[t]
		}')"

	tag_count() { printf '%s\n' "$tags" | awk -v want="$1" -F'\t' '$1 == want { print $2; found = 1 } END { if (!found) print 0 }'; }
	tag_value() { printf '%s\n' "$tags" | awk -v want="$1" -F'\t' '$1 == want { print $3; exit }'; }

	for tag in "${REQUIRED_TAGS[@]}"; do
		count="$(tag_count "$tag")"
		if [ "$count" -eq 0 ]; then
			gate_fail "$rel: missing $tag"
			failed=1
		elif [ "$count" -gt 1 ]; then
			gate_fail "$rel: $tag appears $count times (duplicated header?)"
			failed=1
		fi
	done

	# @file must name the real path relative to its module root.
	local module_root expected declared
	module_root="$(module_root_for "$file")"
	expected="${file#"$module_root"/}"
	declared="$(tag_value '@file')"
	if [ -n "$declared" ] && [ "$declared" != "$expected" ]; then
		gate_fail "$rel: @file says '$declared', expected '$expected'"
		failed=1
	fi

	local author
	author="$(tag_value '@author')"
	if [ "$author" != "$AUTHOR_NAME" ]; then
		gate_fail "$rel: @author must be '$AUTHOR'"
		failed=1
	fi

	local layer stability
	layer="$(tag_value '@layer')"
	if [ -n "$layer" ] && ! is_member "$layer" "$LAYERS"; then
		gate_fail "$rel: @layer '$layer' is not one of: $LAYERS"
		failed=1
	fi

	stability="$(tag_value '@stability')"
	if [ -n "$stability" ] && ! is_member "$stability" "$STABILITIES"; then
		gate_fail "$rel: @stability '$stability' is not one of: $STABILITIES"
		failed=1
	fi
}

gate_start "AGENTS.md §1.2 headers"

while IFS= read -r file; do
	[ -n "$file" ] || continue
	check_file "$file"
done < <(find "$root" -path '*/node_modules' -prune -o -name '*.go' -type f -print 2>/dev/null | sort)

if [ "$failed" -eq 0 ]; then
	gate_pass "$checked Go files carry a complete header"
fi

exit "$failed"